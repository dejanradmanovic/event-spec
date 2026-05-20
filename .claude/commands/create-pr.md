Create a pull request for the current branch following the project's PR conventions.

## Steps

1. Run these in parallel:
   - `git log main..HEAD --oneline` — commits ahead of main
   - `git diff main...HEAD` — full diff
   - `git branch --show-current` — current branch name
   - `gh api user --jq '.login'` — get the authenticated GitHub username

2. Extract the issue number from the branch name. Branch names follow the pattern `{author}/{issue-number}` (e.g. `dejan/1` → issue `1`). If `$ARGUMENTS` is provided, treat it as an override issue number.

3. Fetch the GitHub issue:
   ```
   gh issue view {issue-number} --repo dejanradmanovic/event-spec --json title,labels,body,milestone
   ```

4. Determine the phase prefix from the issue labels (e.g. label `phase-1` → `[P1]`, `phase-2` → `[P2]`, etc.).

5. Build the PR title: `[P{N}] {issue title stripped of its own [P1] prefix if present}`.

6. Collect the label names from the issue to apply to the PR. Build a `--label` flag for each one.

7. Build the PR body using the template below. Fill every section — do not leave placeholders. Base the content on the actual diff and commits.

8. Create the PR:
   ```
   gh pr create \
     --repo dejanradmanovic/event-spec \
     --base main \
     --title "{title}" \
     --body "{body}"
   ```
   Then immediately patch it with assignee, labels, milestone, and project in one call:
   ```
   gh pr edit {pr-url} \
     --add-assignee "@me" \
     --add-label "{label1}" --add-label "{label2}" ... \
     --milestone "{milestone-title}" \
     --add-project "event-spec Implementation Roadmap"
   ```
   Use the `milestone.title` field from the issue JSON fetched in step 3. If the issue has no milestone, omit the `--milestone` flag.

9. Return the PR URL.

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
