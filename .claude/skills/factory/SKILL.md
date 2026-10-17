---
name: factory
description: Run GitHub issues through the software factory loop — spec, build, four parallel reviews, approval, pull request, then follow-up issues for the findings nobody blocked on. Up to three jobs run at once, each in its own git worktree, and each ships as its own PR. Use for /factory <feature>, /factory next [n], /factory groom [n|issue…], /factory approve <job-id>, /factory rework <job-id> <note>, /factory followups <job-id>, /factory status, /factory clean.
---

# The software factory

You are the **orchestrator**. You run the loop, spawn every agent, and you are the
**only writer of `factory/board.json`**. You never write code yourself and you never
write an agent's file for it.

**Up to 3 jobs run at once.** Each job owns a branch and its own git worktree, and each
approved job ships as **its own pull request**. You stay in the main tree, and the main
tree always stays on `main`.

**You never write to `main`.** Not a merge, not a commit, not a push. `main` is a
read-only reference you branch from and diff against. An approved job is handed over by
pushing its branch and opening a PR; whether that PR lands, and when, is the user's call
on GitHub. One feature, one branch, one PR.

## The loop (edit this to change the factory)

```
                   ┌──────────────────┐
                   │     feature      │
                   └─────────┬────────┘
                             ▼
                   ┌──────────────────┐
                   │   spec-writer    │
                   │       opus       │
                   └─────────┬────────┘
                             ▼
                   ┌──────────────────┐
                   │     builder      │◄────────────────────┐
                   │       opus       │                     │
                   └─────────┬────────┘                     │
        ┌─────────────┬──────┴──────┬─────────────┐         │
        ▼             ▼             ▼             ▼         │
  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐   │
  │  security │ │     ux    │ │ ui design │ │    code   │   │
  │    opus   │ │   sonnet  │ │   sonnet  │ │   sonnet  │   │
  └─────┬─────┘ └─────┬─────┘ └─────┬─────┘ └─────┬─────┘   │
        └─────────────┴──────┬──────┴─────────────┘         │
                             ▼                              │
                   ┌──────────────────┐   any CHANGES:      │
                   │   WAIT FOR ALL   ├─────────────────────┘
                   │   4 reviews in   │   resume builder
                   └─────────┬────────┘   (max 2 rounds)
                             ▼ all PASS
                   ┌──────────────────┐
                   │     approver     │
                   │       opus       │
                   └─┬───────────────┬┘
             APPROVE │               │ ESCALATE
                     ▼               ▼
              ┌─────────────┐ ┌─────────────┐
              │  push + PR  │ │ needs-human │ ◄── /factory approve
              └──────┬──────┘ └─────────────┘
                     ▼
              ┌──────────────────┐
              │ followup-writer  │  deferred findings → proposed issues
              │       opus       │
              └──────┬───────────┘
                     ▼ ask once, then file
              ┌─────────────┐
              │ human merges│  ← on GitHub, never here
              └─────────────┘
```

Every job runs its own copy of that loop, and the copies overlap in time:

```
168-rate-limiting  spec─build─review r1─build─review r2─decision──► PR #181
167-validate-json       spec─build─review r1──────────────────────► needs-human
169-pii-redaction            spec─build─review r1─ …
170-docs-lint          queued — at the cap, waiting for a slot
```

## Geography

| What | Where | Who writes it |
|---|---|---|
| `main` | the main tree, `<root>/` | **nobody** — read-only here; only a human merging a PR on GitHub |
| job code | `<root>/.worktrees/<job-id>/`, branch `factory/<job-id>` | that job's builder |
| job artifacts | `<root>/factory/jobs/<job-id>/` | spec-writer, builder, reviewers, approver, followup-writer |
| groom drafts | `<root>/factory/groom/<issue>/` | groomer — drafts for a human, never applied |
| the board | `<root>/factory/board.json` | you, and only you |
| the GitHub snapshot | `<root>/factory/github.json` | `factory/serve.py` — a cache, never you |

`factory/jobs/` and `board.json` are gitignored, so they exist **only in the main tree** —
a worktree has no `factory/jobs/` in it at all. Hand every agent **absolute** paths: the
job folder to write to, and the worktree to work in. An agent that writes its artifact
inside a worktree has written it nowhere.

Worktrees live under `.worktrees/` because the Go tool and golangci-lint skip
dot-directories: `go test ./...` in one tree never sees another job's code. That is
verified behavior — do not relocate them anywhere without it.

## The backlog (GitHub issues)

The backlog is GitHub, not a file in this repo. The queue is **open issues labeled
`ready-for-dev`**, lowest number first:

```
gh issue list --repo dejanradmanovic/event-spec --state open --label ready-for-dev \
  --json number,title,body,labels,milestone --jq 'sort_by(.number)'
```

An issue *is* the feature request. Its `## Goal`, `## File` and `## Acceptance Criteria`
sections are what the spec-writer builds on, and the issue number becomes the job id — so
every branch, job folder and board card traces back to one issue.

**Skip epics.** `ready-for-dev` marks work ready for a *human*, which is not the same as
small enough for one autonomous job. Before opening an issue, read its body. The signals
that it is an epic are **explicit**: more than three `## Step` headings, an `epic` label,
or "epic" in the title — #101 (the Swift SDK, now a tracking epic over #172-#177) is the
live example. Do not open it. Say it needs splitting into one issue per deliverable, and
move to the next in the queue.

Do **not** judge size by counting checkboxes. A well-scoped issue here routinely carries
13-27 acceptance-criteria checkboxes — that is the house style, not an epic. #167 has 13
and is one afternoon's work; #101 has six and is six issues. The dashboard flags the
same explicit signals as `epic?`, a hint to read the issue, never a verdict.

An issue that is *not* `ready-for-dev` is not necessarily unfinished. #172 and #174 are
complete and deliberately unlabelled, because they need a Swift toolchain that this
Windows box does not have — the issues say so themselves. Never "fix" that by labelling
them. The dashboard tiers the whole tracker (ready, blocked, not labelled, needs
grooming, epic) and `/factory groom` interrogates one issue in depth; between them, read
why something is unlabelled before assuming it was an oversight.

The build loop touches GitHub in exactly two places, both read-only: reading the queue,
and the spec-writer fetching an issue body. **This is also its sharpest blind spot** — the
spec-writer fetches only its *own* issue, so a dependency stated in a sibling issue is
invisible to every stage. `/factory groom` exists to read those siblings.

Everything else — specs, builds, reviews, decisions — is local files and local commits.

The **one outward-facing step is the handover**: pushing the branch and opening the PR,
after an APPROVE. That is the whole point of the loop, so it does not need asking each
time — but treat it as publication, because it is. A PR is visible to everyone with repo
access, it triggers CI, and it notifies reviewers. Nothing beyond it is ever automatic:
the factory does not merge the PR, does not close the issue, does not comment on the
issue, and does not push to `main`.

## Rules

- **Files, not conversation.** Every agent gets its context by reading files. Pass it
  paths, not content. Each agent writes exactly one file — its own.
- **Verdict first.** Every review file's first line is `VERDICT: PASS` or
  `VERDICT: CHANGES`. `decision.md`'s first line is `APPROVE` or `ESCALATE`.
- **WAIT FOR ALL is per job.** A round closes only when all four `review-*.md` files
  exist in **that job's** round folder. Another job's round is irrelevant to it. Never
  resume a builder mid-round, never read a partial round.
- **Never block.** A job waiting on its PR or on a blocker must not stop you from
  advancing a different job.
- **The builder owns the branch.** It works and commits only inside its own worktree.
  No one else commits, and nobody but you touches the main tree.
- **Never write to `main`.** No `git merge`, no commit, no `git push origin main`, no
  rebase of `main`. If you ever find yourself about to change `main`, stop — that is the
  signal you have taken over a decision that belongs to the user.
- **One job, one PR.** Never bundle two jobs into one PR, and never push one job's work
  onto another job's branch.
- **Never commit `factory/`.** `board.json` and `jobs/` are gitignored working state.

## Repo facts (pass these to every agent)

- Go 1.26, module `github.com/dejanradmanovic/event-spec`. Packages: `spec/`,
  `analytics/`, `provider/`, `codegen/`, `registry/`, `hooks/`, `generate/`,
  `validate/`, `audit/`, `testutil/`, CLI in `cmd/event-spec/`.
- Polyglot SDKs: `sdk/typescript/` (pnpm), `sdk/kotlin/` (gradlew). Docs:
  `docs/` (Docusaurus).
- Test: `go test ./...` (CI adds `-race`). Lint: `golangci-lint run --timeout=5m`.
  Format: `gofmt -w .`. All three via `make test` / `make lint` / `make fmt`.
- `node_modules/` is gitignored, so a fresh worktree has none: a job touching
  `sdk/typescript/` must run `pnpm install` inside its own worktree first. Gradle works
  off the shared `~/.gradle` cache and needs no setup.
- Commit style: `[scope] short imperative description`, e.g.
  `[provider/amplitude] fix batch payload encoding`.
- Conventions live in `CONTRIBUTING.md`; architecture in `ARCHITECTURE.md`.

## board.json

The dashboard reads this file and nothing else. Rewrite it in full after every stage
transition and every verdict that lands.

```json
{
  "updated": "2026-10-16T09:41:00Z",
  "repo": "dejanradmanovic/event-spec",
  "cap": 3,
  "jobs": [
    {
      "id": "168-rate-limiting",
      "title": "registry/server — per-API-key rate limiting on write endpoints",
      "issue": 168,
      "branch": "factory/168-rate-limiting",
      "worktree": ".worktrees/168-rate-limiting",
      "stage": "review",
      "round": 2,
      "reviews": { "security": "CHANGES", "ux": "PASS", "ui": "pending", "code": "PASS" },
      "history": [
        { "round": 1, "reviews": { "security": "CHANGES", "ux": "PASS", "ui": "PASS", "code": "CHANGES" } }
      ],
      "decision": null,
      "pr": null,
      "followups": [],
      "blockedBy": null,
      "builderAgent": "builder-168-rate-limiting",
      "note": "",
      "started": "2026-10-16T09:02:00Z"
    }
  ]
}
```

- `stage`: `queued` | `spec` | `build` | `review` | `decision` | `pr-open` | `needs-human`
  | `merged` — `pr-open` is terminal **for the factory**: the work is handed over and the
  PR is the user's to merge. `merged` is not a stage the loop ever reaches on its own;
  record it only once you have *confirmed* the merge on GitHub (`gh pr view <url> --json
  state`), the way `/factory clean` already does. You still never merge anything.
  The dashboard does not wait for that: it moves a job to its **shipped** rail as soon as
  GitHub reports the branch merged, whatever the board still says.
- `issue`: the GitHub issue number this job implements, or `null` for an ad-hoc job
- `branch` and `worktree`: `null` until the build stage creates them
- `reviews`: always the **current** round. Values `pending` | `PASS` | `CHANGES`
- `history`: every closed round, in order. Push the finished `reviews` here when a round
  ends and you open the next one
- `decision`: `null` | `APPROVE` | `ESCALATE`
- `pr`: the pull request URL once opened, else `null`. The dashboard links it, and it is
  how a human gets from a board card to the actual review surface. Set it the moment the
  PR exists — a card with a merged PR and `pr: null` is a card that cannot be traced.
- `blockedBy`: the id of the job this one is waiting on, or `null`
- `followups`: issue numbers filed from this job's deferred findings, `[]` if none yet.
  The dashboard shows them on the card, and they appear in the backlog rail as *not
  labelled* — complete, unqueued, waiting for your read.
- `note`: one line for a human — why it escalated, what rework was asked for
- Newest job first.

## The dispatch loop

Each in-flight job is either **working** (an agent is running for it) or **ready** (it
needs you to launch its next stage). Repeat until every job is `pr-open` or
`needs-human`:

1. **Admit.** While fewer than `cap` jobs are **working**, open the next queued job. A
   job is working when it is past `queued`, not terminal, and not `blockedBy` anything —
   a blocked job has no agent running and nothing to launch, so it does not hold a slot.
2. **Launch.** For every ready job, launch its next stage — all of them in a **single
   message**, so they start together.
3. **Wait.** Wait for the next agent to report. You are notified as each one finishes.
4. **Advance.** Route that report to its job, apply the transition below, and rewrite
   `board.json`. The job becomes ready again, or terminal.
5. Back to 1.

Never serialize what can overlap: four reviewers for job A and a builder for job B
belong in the same message.

## Opening a job

1. **Id.** An issue-backed job is `<issue-number>-<slug>` — `168-rate-limiting`. An
   ad-hoc job from `/factory <feature>` has no issue, so it is `adhoc-<slug>`; append
   `-2`, `-3` if that slug is taken. Create `factory/jobs/<id>/`.
2. Add the job to `board.json` with `issue` set to the number, or `null` when ad-hoc. At
   the cap, `stage: "queued"` and stop here. Otherwise `stage: "spec"`.
3. No branch and no worktree yet — the build stage creates both.
4. Launch `spec-writer` with the **absolute** job folder path, the main tree path to
   read, the repo facts, and the work itself: the issue number for an issue-backed job
   (it fetches the body with `gh` itself), or the feature text for an ad-hoc one.

## Stage transitions

**spec-writer reported** → Read `spec.md`. No testable acceptance criteria? Re-spawn it
once with that complaint. Then compare its `## Files` list against the `## Files` of
every in-flight job:

- **Overlap** → set `blockedBy` to that job's id and leave this one at `spec`. It waits
  there until the other job leaves the board. Two builders editing one file is a
  guaranteed merge conflict; waiting is cheaper than rework.
- **No overlap** → create the branch and the worktree in one step:
  ```
  git -C <root> worktree add .worktrees/<id> -b factory/<id> main
  ```
  Record `branch` and `worktree`, set `stage: "build"` and `round: 1`, and launch
  `builder` with a prompt that starts `You are builder-<id>.`, the absolute worktree
  path, and the absolute job folder path. Then call `ListAgents`, find the builder's
  address, and store it as `builderAgent` — that is how you resume it later, including
  after a restart.

**builder reported** → Confirm it committed:
`git -C <root>/.worktrees/<id> log main..factory/<id> --oneline`. An empty log is a
build failure, not a pass: set `needs-human` and say so. Otherwise create
`factory/jobs/<id>/round-<round>/`, set `stage: "review"`, set all four `reviews` to
`pending`, and spawn **all four reviewers for this job in a single message**:
`security-reviewer`, `ux-reviewer`, `ui-reviewer`, `code-reviewer`. Give each the
absolute job folder, the absolute round folder, the absolute worktree, and the repo
facts.

**a reviewer reported** → Read the first line of its file and record that one verdict in
`board.json`. Then check whether all four files exist in **this job's** round folder:

- **Not all four yet** → this job is still working. Go advance something else.
- **All four, any `CHANGES`, `round < 2`** → push this round's `reviews` onto `history`,
  `round++`, reset `reviews` to `pending`, `stage: "build"`, create the next round
  folder, and resume the **same** builder with `SendMessage` to `builderAgent`: name the
  previous round folder and tell it to address every `CHANGES` finding and append a
  `## Round <n>` section to `build.md`.
- **All four, any `CHANGES`, `round == 2`** → go to the approver anyway, and tell it the
  round cap was hit with findings outstanding.
- **All four `PASS`** → go to the approver.

**going to the approver** → `stage: "decision"`, launch `approver` with the absolute job
folder and the absolute worktree.

**approver reported** → Read the first line of `decision.md`.

- `ESCALATE` → `stage: "needs-human"`, copy the approver's reason into `note`. Keep the
  worktree — a human will want it. Anything `blockedBy` this job stays blocked until a
  human resolves it; say so in your report.
- `APPROVE` → open the pull request, below.

**a job left the board** (PR opened, or escalated and then resolved) → clear `blockedBy`
on every job waiting for it, so those become ready.

## Opening the pull request

This is the handover. It runs entirely against the job's own branch — `main` is never
touched, so PRs do not contend with each other and need no lock. Run it per job, as each
one is approved.

1. Last gate: `go test ./...` **in the job's worktree**. On failure, `stage:
   "needs-human"` with the real output in `note`, and no PR. A green run here is what the
   PR body gets to claim.
2. `git -C <root>/.worktrees/<id> status --porcelain` must be empty. Uncommitted work in
   a worktree means the builder left something behind — stop and say so.
3. Push the branch:
   `git -C <root>/.worktrees/<id> push -u origin factory/<id>`
   Expect pre-push hooks to run; they are part of the gate, so do not pass `--no-verify`.
   If the push is rejected because the remote branch moved, do **not** force — stop,
   `needs-human`, and report it.
4. Create the PR with `gh pr create --base main --head factory/<id>`, `--title
   "[factory] <id>: <title>"`, and `--body-file` pointing at a body you write to
   `<root>/factory/jobs/<id>/pr-body.md` (see below). Add `--label` for each label on the
   issue and `--milestone` from it, matching what `create-pr` does for hand-written PRs.
5. Record the URL in `pr`, set `stage: "pr-open"`, and report the URL and issue number.
6. Then check mergeability once, and only to report it:
   `gh pr view <url> --json mergeable,mergeStateStatus`. A conflicted PR is not a factory
   failure — the branch was cut from an older `main` — but the user should hear it in the
   same breath as the link. Do not rebase or resolve it yourself.

Do **not** merge the PR, do **not** delete the branch, and do **not** mark it ready-to-
merge on the user's behalf. The PR is where your work stops.

### The PR body

The job folder already contains everything a reviewer wants, so assemble it rather than
inventing prose. Write `pr-body.md` with:

- **What changed** — from the spec's `## Goal`, as bullets naming real files and
  functions, not "added the package".
- **Closes #&lt;issue&gt;** for an issue-backed job; omit entirely for an ad-hoc one.
- **How to test** — the spec's acceptance criteria as runnable commands, with the actual
  output the builder recorded. Do not restate a criterion you cannot name a command for.
- **Review trail** — a small table of the four verdicts per round, and the approver's
  one-line verdict. This is the factory's value: a human can see it was reviewed four
  ways before it reached them.
- **Caveats and limits** — every honest limit the build report or approver recorded:
  what could not be verified on this machine, what is assumed about unlanded work, which
  acceptance criteria are deliberately unticked. Put these *in* the PR, not only in the
  terminal. A caveat that lives only in chat is a caveat the reviewer never sees.
- **Deferred findings** — anything the reviewers raised and consciously did not block on.
  Cite the issue number tracking it when one already exists; the rest are filed right
  after this PR opens, by **Follow-up issues** below. Do not promise a number you have
  not created — that promise is how three merged PRs ended up citing follow-ups that
  were never filed.

Never claim a check you did not run. If the build report says something is unverifiable
here, the PR body says so too, in the same words.

### Follow-up issues

The PR is open, and the reviewers' deferred findings are now sitting in a **gitignored**
job folder. Once that worktree is cleaned, the only record of them is prose in a merged
PR body: not queryable, not assignable, invisible in the backlog. So the last stage of
the loop turns them into issues.

1. Launch `followup-writer` with the absolute job folder and worktree. It writes
   `followups.md`: one complete proposed issue per deferred finding, deduped against the
   live tracker, plus a `## Not filed` section with its reasoning. It never touches
   GitHub.
2. Read it. **Then ask the user once**, listing the proposed titles and the count, and
   say what it declined to file and why. Creating issues is a public write — it notifies
   watchers and it is the kind of thing that fills a tracker with noise if it runs
   unattended. One clear yes covers the whole batch for this job.
3. On a yes, file each with `gh issue create --title ... --body-file ...`, the labels the
   proposal names, and `--milestone` when it names one. Write each body to
   `factory/jobs/<id>/followup-<n>.md` first so there is a local record of exactly what
   was filed.
4. Record the numbers in the job's `followups` array in `board.json`, and report them with
   the PR link.

Two rules hold firm:

- **Never apply `ready-for-dev`.** These arrive complete but unlabelled, which is
  deliberate: the dashboard tiers them as *not labelled* so they surface for a human read
  rather than silently entering the build queue. Whether the factory works on something
  next is the user's decision, and a loop that queues its own follow-up work decides that
  for them.
- **Never file a NIT the reviewer withdrew.** When a builder answered a point with a
  measurement and the reviewer stood down, that argument is settled. Reopening it as an
  issue is noise, and noise is what makes a tracker unusable.

If `followups.md` starts with `NONE`, say so in one line and file nothing.

### The issue

The issue stays **open and uncommented** by the loop. `Closes #<issue>` in the PR body is
the only link, and it acts only if and when a human merges. Commenting on an issue is a
separate public action: report the PR, then ask once whether to comment or close. Act
only on a clear yes, and never for an ad-hoc job, which has no issue.

### Branches fall behind, and that is fine

Each branch is cut from the `main` that existed when its spec cleared, so once one PR
merges the others are behind. Leave them: their reviews were sound for what they changed,
and the file-overlap check at spec time is what keeps two open PRs from fighting. GitHub
will show "this branch is out of date" — updating a branch is a decision about merge
order, which belongs to whoever merges, not to the factory.

## Commands

### /factory `<feature>`
Open one job (see **Opening a job**) and run the dispatch loop until it is terminal.

### /factory next `[n]`
Open the first `n` issues from the queue in **The backlog** — `n` defaults to 1, and
anything beyond the cap starts `queued`. Skip any issue already on the board and any
epic. Run the dispatch loop until all of them are terminal. If the queue is empty, say
so and stop.

### /factory status
One line per job: id, issue, stage, round, the four verdicts, the PR URL if open, and
`blockedBy` or `note`. Then the counts — in flight, queued, needs-human, PRs open. Read
it from `board.json`; do not re-derive it from the filesystem.

### /factory approve `<job-id>`
For a `needs-human` job the user has decided to accept. Read `decision.md` so you can
restate what was escalated, then run the PR sequence above — and say in the PR body what
was escalated and that a human overrode it. If the tests fail, do not open the PR.

### /factory groom `[n | <issue-number> …]`
Grill issues **before** the factory spends a worktree on them. With issue numbers, grooms
exactly those. With a bare `n` (default 3), takes the first `n` issues that are not
`ready-for-dev`, lowest number first — the grooming backlog the dashboard shows.

For each, create `factory/groom/<issue>/` and launch `groomer` with the issue number, that
absolute folder, and the main tree path. Groom in parallel, all in one message — they only
read. Then report one line per issue: verdict, the deciding reason, and the proposed edits
file path. Never apply the edits and never label anything; `groom.md` is a draft for the
user to paste or ignore.

This exists because every other gate in the loop is post-build and diff-scoped. Four
reviewers reading a worktree cannot catch "the prerequisite has not landed" — which is how
#173 shipped `codegen/swift` generating calls against a Swift runtime absent from `main`,
with #172 saying so in its own body. The groomer reads sibling issues; nothing else does.

`factory/groom/` is gitignored working state, like `factory/jobs/`.

### /factory followups `<job-id>`
Run the follow-up stage on its own, for a job whose PR is already open or merged — including
one that shipped before this stage existed. Same sequence as **Follow-up issues**: launch
`followup-writer`, read `followups.md`, ask once with the titles, file on a yes, record the
numbers in `followups`. Safe to re-run: the agent searches the tracker first, so an
already-filed follow-up comes back as "already tracked as #N" rather than a duplicate.

### /factory rework `<job-id> <note>`
For a `needs-human` job the user wants another pass at. Write the note verbatim to
`factory/jobs/<job-id>/rework-<round+1>.md`, push the current `reviews` onto `history`,
bump `round`, set `stage: "build"`, clear `decision` and `note`, reset `reviews` to
`pending`, and resume `builderAgent` with `SendMessage` pointing at the rework file and
the prior round folder. If that agent is gone, spawn a fresh `builder` on the same
worktree and record the new address. A rework round gets a full four-reviewer round like
any other.

### /factory clean
For each `pr-open` job **whose PR has actually been merged or closed** — check with
`gh pr view <url> --json state`: `git -C <root> worktree remove .worktrees/<id>`, then
`git -C <root> worktree prune`. Leave the branches — they are the audit trail, and a
still-open PR needs its branch alive. Leave `needs-human` worktrees alone, and leave the
worktree of any PR still open: a reviewer may ask for a change, and that is a `/factory
rework` away only while the worktree exists. Report what you removed and what you kept,
with the reason. If a worktree refuses to go because it has uncommitted changes, name it
and do not force it.

## Dashboard

```
python factory/serve.py            # on Windows this repo's interpreter is `py`
```

Then open <http://localhost:8000/dashboard.html>. You do not need to tell it anything.

It shows three things, from two sources:

| Rail | Source | What it is |
|---|---|---|
| **backlog** | GitHub | open `ready-for-dev` issues, lowest number first — what `/factory next` will pick up. A row already on the board carries its stage as a chip that opens the job. |
| **pipeline** and **needs human** | `board.json` | the loop's own state, polled every 2s |
| **shipped** | GitHub | factory PRs that **actually merged** — PR number, merge commit, diff size, and when. Toggle to `all` for every merged PR in the repo. |

`serve.py` exists because the backlog and the merge truth are **not in this repo**. It
serves `factory/` and answers `GET /github.json` by shelling out to `gh` (the same two
read-only queries the loop uses, plus merged and open PRs), caching for 60s and mirroring
the result to `factory/github.json`.

That mirror is the fallback. Under plain `python -m http.server`, or when `gh` is missing,
unauthenticated or offline, the dashboard reads the last snapshot and says so in a banner
rather than passing stale data off as current. `factory/github.json` is a cache — it is
gitignored, and nothing but `serve.py` writes it.

The card a job gets in **shipped** is its own card, review chips and all, so the audit
trail stays one click away after the PR lands. A merged PR with no job on the board — the
job was cleaned up, or a human opened the PR — shows as the PR alone.
