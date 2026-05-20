Implement a GitHub issue end-to-end: branch, code, commit, PR.

The issue number is provided as `$ARGUMENTS`. Fail immediately with a clear message if no issue number is given.

---

## Steps

### 1. Resolve identity and fetch the issue

Run all of these in parallel:

- `gh api user --jq '.login'` — authenticated GitHub username (used for the branch name)
- `gh issue view $ARGUMENTS --repo dejanradmanovic/event-spec --json title,labels,body,milestone,assignees` — full issue metadata
- `git status --short` — confirm the working tree is clean before touching branches

If the working tree is dirty, stop and tell the user to stash or commit their changes first.

### 2. Prepare the branch

```
git checkout main
git pull origin main
git checkout -b {login}/{issue-number}
```

Branch name format: `{gh-login}/{issue-number}` (e.g. `dejan/7`).

### 3. Read the architecture doc

Read `ARCHITECTURE.md` in full — it is the authoritative design reference for every implementation decision. Cross-reference the issue description against the relevant sections before writing any code.

### 4. Understand the codebase

Explore existing packages that are relevant to the issue (use Glob/Grep/Read). Pay attention to:
- Established patterns (error sentinel names, constructor conventions, test file layout, package-level comments)
- Existing types the new code must satisfy or extend
- Go module name (`go.mod`) for import paths

### 5. Implement the issue

Follow the acceptance criteria in the issue body exactly. Implementation rules:

- Match the file path(s) stated in the issue (`## File` section).
- Default zero-value config fields using the same `if x <= 0 { x = default }` pattern as the rest of the codebase.
- Write tests in a `_test.go` file alongside the implementation; use `package foo_test` (external test package) unless white-box access is required.
- Do not add comments that restate what the code does; only comment non-obvious invariants or constraints.
- Do not introduce abstractions beyond what the issue requires.

### 6. Verify

```
go build ./...
go test ./...
```

Fix any compilation errors or test failures before proceeding.

### 7. Commit

Stage only the files introduced or modified for this issue:

```
git add {file1} {file2} ...
```

Commit message format (match the project style from `git log --oneline -5`):

```
[P{N}] {short description matching the issue title, stripped of its own [P1] prefix}
```

Include the co-author trailer:

```
Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
```

### 8. Open the PR

Invoke the `/create-pr` skill — it handles formatting, lint, push, and PR creation automatically.

---

## Constraints

- Never force-push, reset --hard, or amend a commit that has already been pushed.
- Never skip pre-commit hooks (`--no-verify`).
- If lint or tests fail after implementation, fix the root cause — do not suppress linter rules to make them pass.
- If the issue is ambiguous, implement the most conservative interpretation and note the assumption in the PR body.
