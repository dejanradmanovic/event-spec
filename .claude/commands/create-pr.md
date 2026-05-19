Create a pull request for the current branch following the project's PR conventions.

## Steps

1. Run these in parallel:
   - `git log main..HEAD --oneline` — commits ahead of main
   - `git diff main...HEAD` — full diff
   - `git branch --show-current` — current branch name

2. Extract the issue number from the branch name. Branch names follow the pattern `{author}/{issue-number}` (e.g. `dejan/1` → issue `1`). If `$ARGUMENTS` is provided, treat it as an override issue number.

3. Fetch the GitHub issue using `gh issue view {issue-number} --repo dejanradmanovic/event-spec --json title,labels,body,milestone`.

4. Determine the milestone/phase label from the issue (e.g. `phase-1` → `[P1]`, `phase-2` → `[P2]`, etc.) to use as the PR title prefix.

5. Build the PR title: `[P{N}] {issue title stripped of its own [P1] prefix if present}`.

6. Build the PR body using the template below. Fill every section — do not leave placeholders. Base the content on the actual diff and commits.

7. Run `gh pr create --repo dejanradmanovic/event-spec --base main --title "{title}" --body "{body}"` and return the PR URL.

---

## PR body template

```
## What changed

Closes #{issue-number}

{2-4 bullet points summarising what was implemented. Be specific: name the files and functions added, not just "added the package".}

## How to test

{Concrete commands to verify the work. Include the test command and what the expected output or behaviour is. If there is no automated test, describe the manual steps.}

## Checklist

- [x] Tests added or updated
- [x] `go test ./...` passes
- [ ] `golangci-lint run` passes
- [ ] No new `TODO`/`FIXME` left without a tracking issue
```

Mark lint checkbox as `[x]` only if you have confirmed lint passes. Leave it `[ ]` if you have not run it.
