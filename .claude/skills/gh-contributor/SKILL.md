---
name: gh-contributor
description: |
  Automate the full GitHub contribution workflow: fetch issues from the current
  or upstream repository, implement fixes, create pull requests with quality
  descriptions, and monitor CI until it passes. Use this skill whenever the user
  says "fix an issue", "contribute to", "work on an issue", "fix a bug",
  "implement a feature", "create a PR", "help with open source", or mentions
  wanting to contribute to a GitHub project. Also use when the user mentions
  "good first issue", "help wanted", or wants to automate their open source
  contributions.
---

# gh-contributor

Automate GitHub contributions from issue selection to CI pass.

## Prerequisites

- `gh` CLI installed and authenticated (`gh auth status`)
- `git` configured with push access to your fork
- Python 3.9+ available

## Workflow

### Phase 1: Setup

1. Verify `gh auth status` passes. If not, abort with instructions.
2. Detect if repo is a fork:
   ```bash
   git remote -v
   ```
   If fork with no `upstream` remote:
   ```bash
   gh api repos/$(git remote get-url origin | sed 's/.*github.com[:\/]//' | sed 's/\.git$//') --jq '.parent.clone_url' | xargs git remote add upstream
   git fetch upstream
   ```
3. Check working directory is clean:
   ```bash
   git status --short
   ```
   If dirty, stash changes:
   ```bash
   git stash push -m "gh-contributor auto-stash"
   ```

### Phase 2: Issue Selection

1. If user provided issue number, fetch it:
   ```bash
   python .claude/skills/gh-contributor/scripts/fetch_issues.py --issue <NUMBER> --fork-mode
   ```
2. Else, list recent open issues:
   ```bash
   python .claude/skills/gh-contributor/scripts/fetch_issues.py --limit 10 --fork-mode
   ```
3. Present the issue to the user:
   - Title
   - Body (first 500 chars)
   - Labels
   - URL
4. If user rejects, show next issue. Repeat until accepted or list exhausted.

### Phase 3: Branch & Implement

1. Create branch from upstream/master (or origin/master):
   ```bash
   git checkout -b fix-<issue-num>-<short-desc>
   ```
   Branch naming:
   - Bug fix: `fix-<issue-num>-<short-desc>`
   - Feature: `feat-<issue-num>-<short-desc>`
   - Docs: `docs-<issue-num>-<short-desc>`
   - Test: `test-<issue-num>-<short-desc>`

2. **Understand the issue**:
   - Read the issue body carefully
   - Extract: problem statement, expected behavior, actual behavior, reproduction steps
   - Search codebase for relevant files using issue keywords

3. **Explore and fix**:
   - Use `Grep` to find relevant code
   - Use `Read` to understand the codebase
   - Plan minimal change needed
   - Implement the fix

4. **Run tests**:
   - Go: `go test ./<affected-package>`
   - Node: `npm test` or `yarn test`
   - Python: `pytest` or `python -m unittest`
   - Prefer targeted tests over full suite
   - If tests fail, attempt to fix; if still failing, abort with report

### Phase 4: Commit & Push

1. Stage changes selectively (respect .gitignore):
   ```bash
   git add -A  # then review with git diff --cached
   ```
   Remove any accidentally staged files (`.env`, IDE configs, build artifacts).

2. Write conventional commit:
   ```
   fix(scope): description (#issue)
   ```
   Examples:
   - `fix(container): handle nil pointer in restart (#52283)`
   - `feat(api): add new endpoint for volume stats (#12345)`
   - `docs(readme): fix typo in build instructions (#67890)`

3. Push to origin (your fork):
   ```bash
   git push -u origin <branch-name>
   ```

### Phase 5: PR Creation

1. Generate and open PR:
   ```bash
   python .claude/skills/gh-contributor/scripts/create_pr.py \
     --branch $(git branch --show-current) \
     --issue <ISSUE_NUMBER>
   ```

2. The script generates:
   - Title: `fix: Brief description (fixes #issue)`
   - Body with: linked issue, change summary, testing notes, checklist

3. If uncertain, create as draft PR with `--draft` flag.

### Phase 6: CI Monitoring

1. Monitor CI:
   ```bash
   python .claude/skills/gh-contributor/scripts/monitor_ci.py \
     --pr <PR_NUMBER> \
     --max-retries 3 \
     --poll-interval 60
   ```

2. On failure, the script reports:
   - Which check failed
   - Failure type (lint, test, build, unknown)
   - Log excerpt

3. **Auto-retry logic** (handled by SKILL.md, not the script):
   - If failure type is `lint`: run formatter (`gofmt`, `black`, `prettier`), commit, push
   - If failure type is `test`: examine test output, attempt fix, commit, push
   - If failure type is `build`: check for compilation errors, fix imports/dependencies
   - Increment retry counter, re-run monitor_ci.py
   - After 3 retries, report failure with PR link and logs

4. On success, report: "CI passed for PR #N: <url>"

## Error Handling

| Scenario | Behavior |
|----------|----------|
| `gh` not installed | Abort, link to installation docs |
| `gh` not authenticated | Abort, run `gh auth login` |
| No upstream on fork | Auto-add from GitHub API |
| Dirty working dir | Stash, proceed, restore on completion |
| No issues found | Report, suggest broader filters |
| Issue not found | Report, suggest checking number |
| Tests fail pre-PR | Attempt fix; if still failing, abort |
| No push access | Abort early |
| CI fails after 3 retries | Report with logs and PR link |
| Stash exists at end | Pop stash to restore working dir |

## Safety Rules

- Always push to `origin` (your fork), never to `upstream`
- Never auto-merge PRs
- Respect `.gitignore` when staging
- Do not commit secrets, `.env`, or IDE configs
- Confirm branch name before creating
- On abort, restore stashed changes if any
- Do not commit the `.claude` folder or any claude related files