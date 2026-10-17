#!/usr/bin/env python3
"""Serve the factory dashboard, and answer /github.json from the GitHub CLI.

    python factory/serve.py            # http://localhost:8000/dashboard.html
    python factory/serve.py --port 9000

The dashboard polls two files:

  board.json   written by the orchestrator, the factory's own state
  github.json  served by this script, the truth on GitHub - the ready-for-dev
               backlog, and which factory PRs actually merged

gh is slow enough that calling it per poll would be rude, so results are cached
for CACHE_TTL seconds and mirrored to factory/github.json. That mirror is also
the fallback: under plain `python -m http.server`, or when gh is missing or
unauthenticated, the dashboard reads the last good snapshot off disk and says
how old it is.
"""

import argparse
import json
import re
import shutil
import subprocess
import sys
import threading
import time
from datetime import datetime, timezone
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

HERE = Path(__file__).resolve().parent
MIRROR = HERE / "github.json"
CACHE_TTL = 60.0
GH_TIMEOUT = 25.0
BACKLOG_LABEL = "ready-for-dev"
FACTORY_BRANCH = "factory/"

# What a spec-writer needs out of an issue body. Synonyms matter: a bug report
# writes Summary / Affected files / Fix where a feature writes Goal / Files, and
# counting only the feature spelling misreads a perfectly groomed bug as a stub.
SECTION_SYNONYMS = {
    "goal": ("goal", "summary", "problem", "motivation", "context"),
    "files": ("files", "affected file", "affected", "surface"),
    "criteria": ("acceptance criteri", "criteria", "definition of done"),
}
# Anchored at the start of the heading on purpose: #172 has both a real
# "Prerequisite - not startable on a Windows dev box" heading and a prose one,
# "The codegen engine already depends on this API", which is not a blocker.
PREREQ_HEAD = re.compile(r"^(prerequisites?|blocked by|depends on|dependency order)\b", re.I)
ISSUE_REF = re.compile(r"#(\d+)")

_lock = threading.Lock()
_cache = {"at": 0.0, "payload": None}


def _repo():
    """The repo to query, from board.json - the orchestrator already names it."""
    try:
        with open(HERE / "board.json", encoding="utf-8") as fh:
            repo = json.load(fh).get("repo")
            if repo:
                return repo
    except (OSError, ValueError):
        pass
    return ""


def _gh(args, repo):
    exe = shutil.which("gh")
    if not exe:
        raise RuntimeError("gh not found on PATH")
    cmd = [exe] + list(args)
    if repo:
        cmd += ["--repo", repo]
    out = subprocess.run(
        cmd, capture_output=True, text=True, timeout=GH_TIMEOUT, encoding="utf-8"
    )
    if out.returncode != 0:
        why = (out.stderr or out.stdout or "gh failed").strip().splitlines()
        raise RuntimeError(why[0] if why else "gh failed")
    return json.loads(out.stdout or "[]")


def _is_epic(title, body, labels):
    """A hint that an issue is too big for one autonomous job - not a verdict.

    The skill's original rule (more than 3 `## Step` headings, or more than a
    dozen checklist items) does not survive contact with this repo's issues: a
    well-scoped issue routinely carries 13-27 acceptance-criteria checkboxes,
    while #101 - the live epic - has no `## Step` headings and six checkboxes,
    having been rewritten into child issues. So count only explicit markers.
    """
    steps = len(re.findall(r"^##\s+Step\b", body or "", re.MULTILINE))
    labelled = any(lb.lower() == "epic" for lb in labels or [])
    titled = bool(re.search(r"\bepics?\b", title or "", re.IGNORECASE))
    return steps > 3 or labelled or titled


def _headings(body):
    return re.findall(r"^#{2,3}\s+(.*)$", body or "", re.MULTILINE)


def _sections(body):
    """Which of the sections a spec-writer needs are present, by synonym."""
    heads = [h.lower() for h in _headings(body)]
    return {
        key: any(any(w in h for w in words) for h in heads)
        for key, words in SECTION_SYNONYMS.items()
    }


def _blockers(body):
    """What an issue itself says must happen first.

    Two kinds, and the difference decides the tier: issue numbers ("#172 must
    land first") clear themselves when those issues close, while an environment
    blocker ("needs a Swift toolchain, so macOS or Linux") is why an otherwise
    finished issue is deliberately left unlabelled - #172 and #174 are both that
    case, not grooming oversights.
    """
    lines = (body or "").split("\n")
    needs, reason = [], ""
    for i, line in enumerate(lines):
        if not re.match(r"^#{2,3}\s", line):
            continue
        head = re.sub(r"^#{2,3}\s+", "", line).strip()
        if not PREREQ_HEAD.match(head):
            continue
        # First one wins, in document order. A heading that carries its own
        # reason ("Prerequisite - not startable on a Windows dev box") is the
        # most useful single line we can show.
        if not reason:
            reason = head
        for sub in lines[i + 1:]:
            if re.match(r"^#{2,3}\s", sub):
                break
            needs += ISSUE_REF.findall(sub)
    for m in re.finditer(r"^\s*[-*]?\s*Requires:\s*(.+)$", body or "", re.MULTILINE | re.I):
        needs += ISSUE_REF.findall(m.group(1))
    return sorted({int(n) for n in needs}), reason


def _tier(issue, open_numbers):
    """ready | unlabelled | blocked | groom | epic - one bucket per issue.

    An explicit `ready-for-dev` label outranks every soft signal here. A human
    applying it has judged the issue startable, and a heading-name scan is a far
    weaker signal than that: #178 and #179 are labelled, well specified, and
    carry `## Fix` rather than `## Files`, which makes them ready, not stubs.
    So `missing` rides along as a hint on the row and never demotes a tier.

    What does outrank the label is an open issue the body itself says must land
    first - that is checkable fact rather than inference. An environment blocker
    ("needs a Swift toolchain") only tiers an issue as blocked while it is
    unlabelled, which is exactly how #172 and #174 are parked today.
    """
    if issue["epic"]:
        return "epic"
    if [n for n in issue["needs"] if n in open_numbers]:
        return "blocked"
    if issue["labelled"]:
        return "ready"
    if issue["blockedReason"]:
        return "blocked"
    return "groom" if issue["missing"] else "unlabelled"


def _checks(pr):
    """Roll a PR's check runs up to one word a card can show."""
    rollup = pr.get("statusCheckRollup") or []
    if not rollup:
        return "none"
    states = {(c.get("conclusion") or c.get("state") or "").upper() for c in rollup}
    if states & {"FAILURE", "ERROR", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED"}:
        return "failing"
    if states & {"", "PENDING", "IN_PROGRESS", "QUEUED", "EXPECTED", "WAITING"}:
        return "running"
    return "passing"


def collect():
    repo = _repo()
    # Every open issue, not just the labelled ones - the point of the grooming
    # rail is the work that is NOT yet queueable, and why.
    issues = _gh(
        [
            "issue", "list", "--state", "open", "--limit", "200",
            "--json", "number,title,body,labels,milestone,updatedAt",
        ],
        repo,
    )
    merged = _gh(
        [
            "pr", "list", "--state", "merged", "--limit", "60",
            "--json", "number,title,headRefName,mergedAt,url,mergeCommit,additions,deletions",
        ],
        repo,
    )
    open_prs = _gh(
        [
            "pr", "list", "--state", "open", "--limit", "60",
            "--json", "number,title,headRefName,url,createdAt,mergeable,"
                      "reviewDecision,statusCheckRollup",
        ],
        repo,
    )

    open_numbers = {i["number"] for i in issues}

    def issue_row(i):
        labels = [lb["name"] for lb in i.get("labels") or []]
        body = i.get("body") or ""
        sections = _sections(body)
        needs, reason = _blockers(body)
        row = {
            "number": i["number"],
            "title": i["title"],
            "labels": labels,
            "milestone": (i.get("milestone") or {}).get("title"),
            "updated": i.get("updatedAt"),
            "epic": _is_epic(i.get("title"), body, labels),
            "labelled": BACKLOG_LABEL in labels,
            "missing": [k for k, present in sections.items() if not present],
            "needs": needs,
            "blockedReason": reason,
            "size": len(body),
        }
        row["blockedBy"] = [n for n in needs if n in open_numbers]
        row["tier"] = _tier(row, open_numbers)
        return row

    backlog = sorted((issue_row(i) for i in issues), key=lambda i: i["number"])
    tiers = {}
    for row in backlog:
        tiers[row["tier"]] = tiers.get(row["tier"], 0) + 1

    def row(pr):
        return {
            "number": pr["number"],
            "title": pr["title"],
            "branch": pr.get("headRefName"),
            "url": pr["url"],
            "factory": (pr.get("headRefName") or "").startswith(FACTORY_BRANCH),
        }

    merged_rows = []
    for pr in merged:
        r = row(pr)
        r["mergedAt"] = pr.get("mergedAt")
        r["mergeCommit"] = ((pr.get("mergeCommit") or {}).get("oid") or "")[:7]
        r["additions"] = pr.get("additions")
        r["deletions"] = pr.get("deletions")
        merged_rows.append(r)
    merged_rows.sort(key=lambda p: p.get("mergedAt") or "", reverse=True)

    open_rows = []
    for pr in open_prs:
        r = row(pr)
        r["created"] = pr.get("createdAt")
        r["mergeable"] = pr.get("mergeable")
        r["review"] = pr.get("reviewDecision")
        r["checks"] = _checks(pr)
        open_rows.append(r)
    open_rows.sort(key=lambda p: p["number"], reverse=True)

    return {
        "fetched": datetime.now(timezone.utc)
        .isoformat(timespec="seconds")
        .replace("+00:00", "Z"),
        "repo": repo,
        "label": BACKLOG_LABEL,
        "backlog": backlog,
        "tiers": tiers,
        "merged": merged_rows,
        "openPRs": open_rows,
        "error": None,
    }


def snapshot():
    """A fresh snapshot, the cached one, or the disk mirror with the reason attached."""
    with _lock:
        if _cache["payload"] and time.monotonic() - _cache["at"] < CACHE_TTL:
            return _cache["payload"]
    try:
        payload = collect()
    except Exception as exc:  # gh missing, unauthenticated, offline, rate-limited
        stale = _cache["payload"]
        if stale is None:
            try:
                with open(MIRROR, encoding="utf-8") as fh:
                    stale = json.load(fh)
            except (OSError, ValueError):
                stale = {"backlog": [], "merged": [], "openPRs": []}
        payload = dict(stale, error=str(exc), stale=True)
        with _lock:
            _cache.update(at=time.monotonic(), payload=payload)
        return payload

    with _lock:
        _cache.update(at=time.monotonic(), payload=payload)
    try:
        MIRROR.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
    except OSError:
        pass
    return payload


class Handler(SimpleHTTPRequestHandler):
    def do_GET(self):  # noqa: N802 - base class name
        if self.path.split("?")[0].lstrip("/") == "github.json":
            body = json.dumps(snapshot()).encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        super().do_GET()

    def end_headers(self):
        self.send_header("Cache-Control", "no-store")
        super().end_headers()

    def log_message(self, fmt, *args):
        pass  # the dashboard polls twice a second; the access log is noise


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--port", type=int, default=8000)
    ap.add_argument("--host", default="127.0.0.1")
    args = ap.parse_args()

    if not shutil.which("gh"):
        print(
            "warning: gh not on PATH - backlog and merged PRs will be the last snapshot",
            file=sys.stderr,
        )

    srv = ThreadingHTTPServer((args.host, args.port), partial(Handler, directory=str(HERE)))
    print("factory dashboard  http://{}:{}/dashboard.html".format(args.host, args.port))
    try:
        srv.serve_forever()
    except KeyboardInterrupt:
        print()
    srv.server_close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
