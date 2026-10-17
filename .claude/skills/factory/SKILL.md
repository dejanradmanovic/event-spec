---
name: factory
description: Run GitHub issues through the software factory loop — spec, build, four parallel reviews, approval, merge. Up to three jobs run at once, each in its own git worktree. Use for /factory <feature>, /factory next [n], /factory approve <job-id>, /factory rework <job-id> <note>, /factory status, /factory clean.
---

# The software factory

You are the **orchestrator**. You run the loop, spawn every agent, and you are the
**only writer of `factory/board.json`**. You never write code yourself and you never
write an agent's file for it.

**Up to 3 jobs run at once.** Each job owns a branch and its own git worktree. You stay
in the main tree, and the main tree always stays on `main`.

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
              │merge to main│ │ needs-human │ ◄── /factory approve
              └─────────────┘ └─────────────┘
```

Every job runs its own copy of that loop, and the copies overlap in time:

```
168-rate-limiting  spec─build─review r1─build─review r2─decision──► merged
167-validate-json       spec─build─review r1──────────────────────► needs-human
169-pii-redaction            spec─build─review r1─ …
170-docs-lint          queued — at the cap, waiting for a slot
```

## Geography

| What | Where | Who writes it |
|---|---|---|
| `main` | the main tree, `<root>/` | you, by merging |
| job code | `<root>/.worktrees/<job-id>/`, branch `factory/<job-id>` | that job's builder |
| job artifacts | `<root>/factory/jobs/<job-id>/` | spec-writer, builder, reviewers, approver |
| the board | `<root>/factory/board.json` | you, and only you |

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
small enough for one autonomous job. Before opening an issue, read its body: more than
three `## Step` headings, or more than a dozen checklist items, means it is an epic —
#101 (the Swift SDK, six steps) is the live example. Do not open it. Say it needs
splitting into one issue per deliverable, and move to the next in the queue.

Nothing in the loop writes to GitHub. **Merging** has the one place a public action is
offered, and it is always the user's call.

## Rules

- **Files, not conversation.** Every agent gets its context by reading files. Pass it
  paths, not content. Each agent writes exactly one file — its own.
- **Verdict first.** Every review file's first line is `VERDICT: PASS` or
  `VERDICT: CHANGES`. `decision.md`'s first line is `APPROVE` or `ESCALATE`.
- **WAIT FOR ALL is per job.** A round closes only when all four `review-*.md` files
  exist in **that job's** round folder. Another job's round is irrelevant to it. Never
  resume a builder mid-round, never read a partial round.
- **Never block.** A job waiting on a merge or on a blocker must not stop you from
  advancing a different job.
- **The builder owns the branch.** It works and commits only inside its own worktree.
  No one else commits, and nobody but you touches the main tree.
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
      "blockedBy": null,
      "builderAgent": "builder-168-rate-limiting",
      "note": "",
      "started": "2026-10-16T09:02:00Z"
    }
  ]
}
```

- `stage`: `queued` | `spec` | `build` | `review` | `decision` | `merged` | `needs-human`
- `issue`: the GitHub issue number this job implements, or `null` for an ad-hoc job
- `branch` and `worktree`: `null` until the build stage creates them
- `reviews`: always the **current** round. Values `pending` | `PASS` | `CHANGES`
- `history`: every closed round, in order. Push the finished `reviews` here when a round
  ends and you open the next one
- `decision`: `null` | `APPROVE` | `ESCALATE`
- `blockedBy`: the id of the job this one is waiting on, or `null`
- `note`: one line for a human — why it escalated, what rework was asked for
- Newest job first.

## The dispatch loop

Each in-flight job is either **working** (an agent is running for it) or **ready** (it
needs you to launch its next stage). Repeat until every job is `merged` or
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
- `APPROVE` → join the merge queue below.

**a job left the board** (merged, or escalated and then resolved) → clear `blockedBy` on
every job waiting for it, so those become ready.

## Merging (one at a time)

Merges are the one thing that must never overlap. Hold a mental lock: one merge, start
to finish, before the next. It all runs in the main tree, which is already on `main`.

1. `git -C <root> status --porcelain` — the main tree must be clean. If it is not, stop
   and tell the user.
2. Last gate: `go test ./...` **in the job's worktree**. On failure, `needs-human` with
   the real output in `note`, and no merge.
3. `git -C <root> merge --no-ff factory/<id> -m "[factory] <id>: <title> (#<issue>)"`
   — drop the `(#<issue>)` for an ad-hoc job. The reference makes the merge commit
   cross-link the issue once the user pushes; it does not close anything.
4. **Conflict** → `git -C <root> merge --abort`, `stage: "needs-human"`, and put the
   conflicting paths in `note`. Do not resolve it yourself.
5. `stage: "merged"`. Do **not** push and do **not** delete the branch — pushing is the
   user's call. Report the merge commit and the issue number.

The issue stays open and uncommented. Commenting on or closing an issue is a public
action on someone's repo, so it is never part of the loop: report what merged, then ask
once whether to comment the merge commit on the issue and close it. Act only on a clear
yes, and never on an ad-hoc job, which has no issue.

After a merge the other in-flight branches are behind `main`. Leave them there: their
reviews were sound for what they changed, and the overlap check at spec time is what
keeps the next merge clean.

## Commands

### /factory `<feature>`
Open one job (see **Opening a job**) and run the dispatch loop until it is terminal.

### /factory next `[n]`
Open the first `n` issues from the queue in **The backlog** — `n` defaults to 1, and
anything beyond the cap starts `queued`. Skip any issue already on the board and any
epic. Run the dispatch loop until all of them are terminal. If the queue is empty, say
so and stop.

### /factory status
One line per job: id, issue, stage, round, the four verdicts, and `blockedBy` or `note`.
Then the counts — in flight, queued, needs-human, merged. Read it from `board.json`; do
not re-derive it from the filesystem.

### /factory approve `<job-id>`
For a `needs-human` job the user has decided to accept. Read `decision.md` so you can
restate what was escalated, then run the merge sequence above. If the tests fail, do not
merge.

### /factory rework `<job-id> <note>`
For a `needs-human` job the user wants another pass at. Write the note verbatim to
`factory/jobs/<job-id>/rework-<round+1>.md`, push the current `reviews` onto `history`,
bump `round`, set `stage: "build"`, clear `decision` and `note`, reset `reviews` to
`pending`, and resume `builderAgent` with `SendMessage` pointing at the rework file and
the prior round folder. If that agent is gone, spawn a fresh `builder` on the same
worktree and record the new address. A rework round gets a full four-reviewer round like
any other.

### /factory clean
For each `merged` job: `git -C <root> worktree remove .worktrees/<id>`, then
`git -C <root> worktree prune`. Leave the branches — they are the audit trail — and
leave `needs-human` worktrees alone. Report what you removed. If a worktree refuses to
go because it has uncommitted changes, name it and do not force it.

## Dashboard

```
cd factory && python3 -m http.server 8000   # or: python -m http.server 8000
```

Then open <http://localhost:8000/dashboard.html>. It polls `board.json`; you do not need
to tell it anything.
