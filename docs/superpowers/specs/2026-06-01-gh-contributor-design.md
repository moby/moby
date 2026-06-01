# gh-contributor Skill Design

## Overview

A Claude Code skill that automates the full GitHub contribution workflow: fetch issues from the current or upstream repository, implement fixes, open pull requests with quality descriptions, and monitor CI until it passes.

## Goals

- Enable users to fix open issues with minimal manual steps
- Support both user-specified issue numbers and automatic issue discovery
- Work correctly whether the local repo is a fork or the original
- Produce high-quality PR descriptions that reference issues and explain changes
- Handle CI failures automatically with bounded retry logic

## Non-Goals

- Auto-merging PRs (stops at CI pass, human merges)
- Working on issues that require external coordination or design decisions
- Contributing to repos the user does not have push access to

## Architecture

```
gh-contributor/
├── SKILL.md                          # Workflow orchestration + reasoning guidance
└── scripts/
    ├── fetch_issues.py               # Query issues (fork-aware)
    ├── create_pr.py                  # Generate PR description & open PR
    └── monitor_ci.py                 # Poll CI, retry on failure
```

### Design Rationale: Script-Backed Workflow

Deterministic GitHub operations (API queries, PR creation, CI polling) are bundled as Python scripts invoked by the skill. This keeps SKILL.md focused on what Claude does best — reasoning about code, planning fixes, and writing good commit messages — while scripts handle the mechanical, error-prone operations reliably.

## Skill Workflow (SKILL.md)

### Phase 1: Setup

1. Verify `gh` CLI is installed and authenticated (`gh auth status`)
2. Detect if current repo is a fork via `git remote -v` + GitHub API
3. If fork with no `upstream` remote, add it from GitHub API data
4. Ensure working directory is clean (stash uncommitted changes or abort)
5. If on a feature branch, decide whether to continue or start fresh

### Phase 2: Issue Selection

1. If user provided issue number: fetch that specific issue via `fetch_issues.py --issue N`
2. Else: call `fetch_issues.py` to list open issues, sorted by recency (newest first)
3. Present the selected issue (title + body summary truncated to ~500 chars) for confirmation
4. If user rejects, show next issue in list

### Phase 3: Branch & Implement

1. Create branch from upstream/master (if fork) or origin/master: `fix-<issue-num>-<short-desc>` or `feat-<issue-num>-<short-desc>`
2. Read issue description carefully — extract: problem statement, expected behavior, actual behavior, reproduction steps
3. Explore codebase to understand the problem:
   - Search for relevant files using issue keywords
   - Read related code and tests
   - Identify the minimal change needed
4. Plan the fix and implement it
5. Run relevant tests:
   - If Go repo: `go test ./...` for affected packages
   - If Node: `npm test` or `yarn test`
   - If Python: `pytest` or `python -m unittest`
   - Prefer targeted tests over full suite for speed
6. If tests fail, attempt to fix; if still failing after reasonable effort, abort with report

### Phase 4: Commit & Push

1. Stage changes selectively (avoid `.env`, build artifacts, IDE files)
2. Write conventional commit: `fix(scope): description (#issue)` or `feat(scope): description (#issue)`
3. Push branch to `origin` (user's fork, not upstream)

### Phase 5: PR Creation

1. Call `create_pr.py` which:
   - Reads the issue body
   - Runs `git diff upstream/master...HEAD` to capture changes
   - Generates PR title: `[Fix/Feature] Brief description (fixes #issue)`
   - Generates PR body with:
     - Linked issue ("Closes #N" or "Fixes #N")
     - Summary of changes (bullet points from diff)
     - Testing notes
     - Checklist (tests pass, no breaking changes, etc.)
2. Open PR against upstream/master (if fork) or origin/master
3. Optionally mark as draft if uncertain about completeness

### Phase 6: CI Monitoring

1. Call `monitor_ci.py` to poll CI checks on the PR
2. Poll every 60 seconds until all required checks pass
3. On failure:
   - Examine failure logs via `gh run view` or GitHub API
   - Categorize failure type: lint, test, build, other
   - Attempt auto-fix (e.g., run `gofmt`, fix obvious test failures)
   - Commit fix, push, increment retry counter
4. Max 3 retry cycles; after that, report failure with:
   - PR link
   - Failure summary
   - Suggested next steps

## Script Interfaces

### `fetch_issues.py`

```bash
python scripts/fetch_issues.py \
  [--repo owner/repo] \
  [--issue NUMBER] \
  [--label LABEL] \
  [--limit N] \
  [--fork-mode]
```

**Output** (JSON):
```json
{
  "repo": "owner/repo",
  "is_fork": true,
  "upstream": "upstream-owner/upstream-repo",
  "issues": [
    {
      "number": 123,
      "title": "Fix panic in container restart",
      "body": "...",
      "labels": ["bug", "help wanted"],
      "created_at": "2026-05-20T10:00:00Z",
      "url": "https://github.com/owner/repo/issues/123"
    }
  ]
}
```

**Behavior**:
- If `--issue` provided, return single issue or error if not found
- If `--fork-mode`, detect upstream repo and query upstream issues instead of fork issues
- Sort by `created_at` desc (newest first)
- Filter out issues with labels like "blocked", "needs-design", "in-progress" if no explicit label filter

### `create_pr.py`

```bash
python scripts/create_pr.py \
  --branch BRANCH \
  --issue NUMBER \
  [--repo owner/repo] \
  [--draft]
```

**Output** (JSON):
```json
{
  "pr_number": 456,
  "url": "https://github.com/owner/repo/pull/456",
  "title": "Fix panic in container restart (fixes #123)",
  "body": "..."
}
```

**Behavior**:
- Generate PR title from issue title
- Generate PR body from issue body + diff summary
- Use `gh pr create` to open the PR
- Target upstream if fork, else origin

### `monitor_ci.py`

```bash
python scripts/monitor_ci.py \
  --pr NUMBER \
  [--repo owner/repo] \
  [--max-retries 3] \
  [--poll-interval 60]
```

**Output** (JSON):
```json
{
  "status": "passed",
  "retries": 0,
  "checks": [
    {"name": "test-unit", "status": "completed", "conclusion": "success"}
  ]
}
```

Or on failure:
```json
{
  "status": "failed",
  "retries": 3,
  "final_failure": "test-unit",
  "logs_url": "https://github.com/owner/repo/actions/runs/...",
  "failure_summary": "Test timeout in pkg/container/..."
}
```

**Behavior**:
- Poll until all checks complete or max retries exhausted
- On failure, fetch logs and attempt to categorize
- Return structured data for SKILL.md to act on

## Error Handling

| Scenario | Behavior |
|----------|----------|
| `gh` CLI not installed | Abort with install link |
| `gh` not authenticated | Abort with `gh auth login` instructions |
| No upstream remote on fork | Auto-add from GitHub API (repo parent info) |
| Dirty working directory | Stash changes, proceed, restore on completion |
| No issues found | Report clearly, suggest broader filters |
| Issue not found | Report error, suggest checking issue number |
| Tests fail before PR | Attempt auto-fix; if still failing, abort with report |
| No push access | Abort early with clear message |
| CI fails after max retries | Report failure with logs, PR link, and suggested next steps |
| Merge conflicts during rebase | Abort and report |

## Security & Safety

- Never push to upstream directly (always to user's fork)
- Never auto-merge PRs
- Respect `.gitignore` when staging
- Do not commit secrets, `.env` files, or IDE configs
- Confirm before creating branches with potentially destructive names

## Success Criteria

1. Can fetch issues from upstream repo when local is a fork
2. Can create a branch, implement a fix, and push it
3. PR description clearly links to the issue and explains the change
4. CI monitoring works: polls checks, retries on failure, reports final status
5. Graceful handling of all error scenarios in the table above
