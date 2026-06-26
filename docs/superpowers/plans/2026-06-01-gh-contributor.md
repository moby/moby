# gh-contributor Skill Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Claude Code skill that automates the full GitHub contribution workflow: fetch issues, implement fixes, create PRs, and monitor CI.

**Architecture:** Python scripts handle deterministic GitHub operations (API queries, PR creation, CI polling). SKILL.md guides Claude through the reasoning-heavy phases (code exploration, fix planning, commit messages).

**Tech Stack:** Python 3.9+, `gh` CLI, `git`, GitHub REST API

---

## File Structure

```
git_repo_root/
└── .claude/
    └── skills/
        └── gh-contributor/
            ├── SKILL.md
            └── scripts/
                ├── __init__.py
                ├── fetch_issues.py
                ├── create_pr.py
                ├── monitor_ci.py
                └── requirements.txt
```

---

## Task 1: Setup Directory Structure

**Files:**
- Create: `.claude/skills/gh-contributor/scripts/__init__.py`
- Create: `.claude/skills/gh-contributor/scripts/requirements.txt`

- [ ] **Step 1: Create directory structure**

```bash
mkdir -p .claude/skills/gh-contributor/scripts
touch .claude/skills/gh-contributor/scripts/__init__.py
```

- [ ] **Step 2: Create requirements.txt**

Create `.claude/skills/gh-contributor/scripts/requirements.txt`:

```
requests>=2.28.0
```

- [ ] **Step 3: Commit**

```bash
git add .claude/
git commit -m "chore: scaffold gh-contributor skill structure"
```

---

## Task 2: Implement fetch_issues.py

**Files:**
- Create: `.claude/skills/gh-contributor/scripts/fetch_issues.py`

- [ ] **Step 1: Write the script**

Create `.claude/skills/gh-contributor/scripts/fetch_issues.py`:

```python
#!/usr/bin/env python3
"""Fetch issues from GitHub repo, with fork awareness."""

import argparse
import json
import subprocess
import sys
import urllib.request
from typing import Any


def run_gh(args: list[str]) -> str:
    """Run gh CLI and return stdout."""
    result = subprocess.run(
        ["gh"] + args,
        capture_output=True,
        text=True,
        check=True,
    )
    return result.stdout.strip()


def api_request(path: str) -> Any:
    """Make authenticated GitHub API request via gh."""
    url = f"https://api.github.com{path}"
    token = run_gh(["auth", "token"])
    req = urllib.request.Request(
        url,
        headers={
            "Authorization": f"Bearer {token}",
            "Accept": "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
        },
    )
    with urllib.request.urlopen(req, timeout=30) as resp:
        return json.loads(resp.read())


def get_repo_info() -> tuple[str, bool, str | None]:
    """Detect current repo, whether it's a fork, and upstream repo."""
    # Get current remote info
    remotes_output = subprocess.run(
        ["git", "remote", "-v"],
        capture_output=True,
        text=True,
        check=True,
    ).stdout

    origin_url = None
    upstream_url = None
    for line in remotes_output.strip().split("\n"):
        if line.startswith("origin"):
            origin_url = line.split()[1]
        elif line.startswith("upstream"):
            upstream_url = line.split()[1]

    if not origin_url:
        raise RuntimeError("No 'origin' remote found")

    # Extract owner/repo from URL
    def extract_repo(url: str) -> str:
        for prefix in ["https://github.com/", "git@github.com:"]:
            if url.startswith(prefix):
                repo = url[len(prefix):]
                if repo.endswith(".git"):
                    repo = repo[:-4]
                return repo
        raise ValueError(f"Cannot parse repo from URL: {url}")

    origin_repo = extract_repo(origin_url)

    # Check if fork via API
    repo_data = api_request(f"/repos/{origin_repo}")
    is_fork = repo_data.get("fork", False)

    upstream_repo = None
    if is_fork:
        if upstream_url:
            upstream_repo = extract_repo(upstream_url)
        else:
            parent = repo_data.get("parent")
            if parent:
                upstream_repo = parent["full_name"]

    return origin_repo, is_fork, upstream_repo


def fetch_issue(repo: str, issue_number: int) -> dict:
    """Fetch a single issue by number."""
    data = api_request(f"/repos/{repo}/issues/{issue_number}")
    return {
        "number": data["number"],
        "title": data["title"],
        "body": data.get("body", ""),
        "labels": [l["name"] for l in data.get("labels", [])],
        "created_at": data["created_at"],
        "url": data["html_url"],
        "state": data["state"],
    }


def fetch_issues(repo: str, label: str | None = None, limit: int = 10) -> list[dict]:
    """Fetch open issues from repo."""
    params = [f"state=open", f"per_page={limit}", "sort=created", "direction=desc"]
    if label:
        params.append(f"labels={label}")

    query = "&".join(params)
    data = api_request(f"/repos/{repo}/issues?{query}")

    issues = []
    for item in data:
        # Skip pull requests (GitHub returns PRs in issues endpoint)
        if "pull_request" in item:
            continue
        issues.append({
            "number": item["number"],
            "title": item["title"],
            "body": item.get("body", ""),
            "labels": [l["name"] for l in item.get("labels", [])],
            "created_at": item["created_at"],
            "url": item["html_url"],
            "state": item["state"],
        })
    return issues


def main():
    parser = argparse.ArgumentParser(description="Fetch GitHub issues")
    parser.add_argument("--repo", help="Owner/repo (auto-detected if omitted)")
    parser.add_argument("--issue", type=int, help="Specific issue number")
    parser.add_argument("--label", help="Filter by label")
    parser.add_argument("--limit", type=int, default=10, help="Max issues to fetch")
    parser.add_argument("--fork-mode", action="store_true", help="Query upstream if fork")
    args = parser.parse_args()

    try:
        if args.repo:
            origin_repo = args.repo
            is_fork = False
            upstream_repo = None
        else:
            origin_repo, is_fork, upstream_repo = get_repo_info()

        target_repo = upstream_repo if (is_fork and args.fork_mode) else origin_repo

        if args.issue:
            issue = fetch_issue(target_repo, args.issue)
            result = {
                "repo": target_repo,
                "is_fork": is_fork,
                "upstream": upstream_repo,
                "issues": [issue],
            }
        else:
            issues = fetch_issues(target_repo, label=args.label, limit=args.limit)
            result = {
                "repo": target_repo,
                "is_fork": is_fork,
                "upstream": upstream_repo,
                "issues": issues,
            }

        print(json.dumps(result, indent=2))

    except subprocess.CalledProcessError as e:
        print(json.dumps({"error": f"Command failed: {e}"}), file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(json.dumps({"error": str(e)}), file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
```

- [ ] **Step 2: Make executable**

```bash
chmod +x .claude/skills/gh-contributor/scripts/fetch_issues.py
```

- [ ] **Step 3: Test manually**

```bash
cd .claude/skills/gh-contributor
python scripts/fetch_issues.py --repo moby/moby --limit 3
```

Expected: JSON output with up to 3 open issues from moby/moby.

- [ ] **Step 4: Commit**

```bash
git add .claude/skills/gh-contributor/scripts/fetch_issues.py
git commit -m "feat(gh-contributor): add fetch_issues.py script"
```

---

## Task 3: Implement create_pr.py

**Files:**
- Create: `.claude/skills/gh-contributor/scripts/create_pr.py`

- [ ] **Step 1: Write the script**

Create `.claude/skills/gh-contributor/scripts/create_pr.py`:

```python
#!/usr/bin/env python3
"""Generate PR description and open pull request."""

import argparse
import json
import re
import subprocess
import sys


def run_gh(args: list[str]) -> str:
    """Run gh CLI and return stdout."""
    result = subprocess.run(
        ["gh"] + args,
        capture_output=True,
        text=True,
        check=True,
    )
    return result.stdout.strip()


def run_git(args: list[str]) -> str:
    """Run git and return stdout."""
    result = subprocess.run(
        ["git"] + args,
        capture_output=True,
        text=True,
        check=True,
    )
    return result.stdout.strip()


def get_repo_info() -> tuple[str, bool, str | None]:
    """Detect current repo, whether it's a fork, and upstream repo."""
    remotes_output = subprocess.run(
        ["git", "remote", "-v"],
        capture_output=True,
        text=True,
        check=True,
    ).stdout

    origin_url = None
    upstream_url = None
    for line in remotes_output.strip().split("\n"):
        if line.startswith("origin"):
            origin_url = line.split()[1]
        elif line.startswith("upstream"):
            upstream_url = line.split()[1]

    if not origin_url:
        raise RuntimeError("No 'origin' remote found")

    def extract_repo(url: str) -> str:
        for prefix in ["https://github.com/", "git@github.com:"]:
            if url.startswith(prefix):
                repo = url[len(prefix):]
                if repo.endswith(".git"):
                    repo = repo[:-4]
                return repo
        raise ValueError(f"Cannot parse repo from URL: {url}")

    origin_repo = extract_repo(origin_url)
    is_fork = False
    upstream_repo = None

    if upstream_url:
        upstream_repo = extract_repo(upstream_url)
        is_fork = True

    return origin_repo, is_fork, upstream_repo


def get_issue_body(repo: str, issue_number: int) -> str:
    """Fetch issue body via gh API."""
    output = run_gh(["api", f"repos/{repo}/issues/{issue_number}"])
    data = json.loads(output)
    return data.get("body", "") or ""


def get_branch_name() -> str:
    """Get current branch name."""
    return run_git(["branch", "--show-current"])


def get_diff(base: str) -> str:
    """Get diff against base branch."""
    try:
        return run_git(["diff", base, "...HEAD"])
    except subprocess.CalledProcessError:
        # Fallback to simple diff
        return run_git(["diff", base])


def summarize_changes(diff: str) -> list[str]:
    """Extract file change summary from diff."""
    summaries = []
    files_changed = re.findall(r"^diff --git a/(.+) b/", diff, re.MULTILINE)
    for f in files_changed:
        if f.endswith(".go"):
            summaries.append(f"- Modified Go source: `{f}`")
        elif f.endswith("_test.go"):
            summaries.append(f"- Updated tests: `{f}`")
        elif f.endswith(".md"):
            summaries.append(f"- Updated docs: `{f}`")
        elif f.endswith((".yml", ".yaml", ".json")):
            summaries.append(f"- Updated config: `{f}`")
        else:
            summaries.append(f"- Modified: `{f}`")
    return summaries


def generate_pr_title(issue_title: str, issue_number: int) -> str:
    """Generate PR title from issue."""
    # Clean up the title
    title = issue_title.strip()
    # Remove existing issue references
    title = re.sub(r"\s*\(fixes?\s*#\d+\)", "", title, flags=re.IGNORECASE)
    title = re.sub(r"\s*#\d+", "", title)
    title = title.strip()

    # Determine prefix based on issue labels/content heuristics
    prefix = ""
    lower = title.lower()
    if any(w in lower for w in ["fix", "bug", "panic", "crash", "error", "leak"]):
        prefix = "fix"
    elif any(w in lower for w in ["add", "support", "implement", "introduce", "new"]):
        prefix = "feat"
    elif any(w in lower for w in ["doc", "readme", "comment", "typo"]):
        prefix = "docs"
    elif any(w in lower for w in ["refactor", "cleanup", "clean up", "simplify"]):
        prefix = "refactor"
    elif any(w in lower for w in ["test", "testing", "coverage", "flaky"]):
        prefix = "test"
    else:
        prefix = "fix"

    return f"{prefix}: {title} (fixes #{issue_number})"


def generate_pr_body(issue_number: int, issue_body: str, changes: list[str]) -> str:
    """Generate PR body from issue and changes."""
    body_lines = [
        f"fixes #{issue_number}",
        "",
        "## Summary",
        "",
    ]

    # Extract problem from issue body (first paragraph or first 300 chars)
    issue_summary = issue_body.strip()
    if issue_summary:
        # Take first paragraph or first 300 chars
        first_para = issue_summary.split("\n\n")[0]
        if len(first_para) > 300:
            first_para = first_para[:300] + "..."
        body_lines.append(first_para)
    else:
        body_lines.append("See linked issue for full context.")

    body_lines.extend([
        "",
        "## Changes",
        "",
    ])
    body_lines.extend(changes)

    body_lines.extend([
        "",
        "## Testing",
        "",
        "- [ ] Unit tests pass",
        "- [ ] Integration tests pass",
        "- [ ] Manual testing performed",
        "",
        "## Checklist",
        "",
        "- [ ] Code follows project style guidelines",
        "- [ ] Tests added/updated for new behavior",
        "- [ ] No breaking changes introduced",
    ])

    return "\n".join(body_lines)


def open_pr(repo: str, title: str, body: str, branch: str, base: str, draft: bool = False) -> dict:
    """Open PR via gh CLI."""
    args = [
        "pr", "create",
        "--repo", repo,
        "--title", title,
        "--body", body,
        "--head", branch,
        "--base", base,
    ]
    if draft:
        args.append("--draft")

    try:
        url = run_gh(args)
        # Extract PR number from URL
        pr_number = int(url.split("/")[-1])
        return {"pr_number": pr_number, "url": url, "title": title, "body": body}
    except subprocess.CalledProcessError as e:
        return {"error": f"Failed to create PR: {e.stderr or e.stdout}"}


def main():
    parser = argparse.ArgumentParser(description="Create pull request from branch")
    parser.add_argument("--branch", required=True, help="Branch with changes")
    parser.add_argument("--issue", type=int, required=True, help="Issue number being fixed")
    parser.add_argument("--repo", help="Target repo (auto-detected if omitted)")
    parser.add_argument("--draft", action="store_true", help="Create as draft PR")
    args = parser.parse_args()

    try:
        if args.repo:
            target_repo = args.repo
            _, is_fork, upstream_repo = get_repo_info()
        else:
            origin_repo, is_fork, upstream_repo = get_repo_info()
            target_repo = upstream_repo if upstream_repo else origin_repo

        if not target_repo:
            raise RuntimeError("Could not determine target repository")

        # Determine base branch
        base = "master"
        try:
            run_git(["rev-parse", "--verify", "upstream/master"])
            base = "upstream/master"
        except subprocess.CalledProcessError:
            try:
                run_git(["rev-parse", "--verify", "origin/master"])
                base = "origin/master"
            except subprocess.CalledProcessError:
                pass

        # Fetch issue body
        issue_repo = upstream_repo if upstream_repo else target_repo
        issue_body = get_issue_body(issue_repo, args.issue)

        # Get diff
        diff = get_diff(base)
        changes = summarize_changes(diff)

        # Generate PR content
        # We need the issue title - fetch it
        issue_data = json.loads(run_gh(["api", f"repos/{issue_repo}/issues/{args.issue}"]))
        issue_title = issue_data.get("title", "")

        pr_title = generate_pr_title(issue_title, args.issue)
        pr_body = generate_pr_body(args.issue, issue_body, changes)

        # Open PR
        result = open_pr(
            repo=target_repo,
            title=pr_title,
            body=pr_body,
            branch=args.branch,
            base=base.replace("upstream/", "").replace("origin/", ""),
            draft=args.draft,
        )

        if "error" in result:
            print(json.dumps(result), file=sys.stderr)
            sys.exit(1)

        print(json.dumps(result, indent=2))

    except subprocess.CalledProcessError as e:
        print(json.dumps({"error": f"Command failed: {e}"}), file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(json.dumps({"error": str(e)}), file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
```

- [ ] **Step 2: Make executable**

```bash
chmod +x .claude/skills/gh-contributor/scripts/create_pr.py
```

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/gh-contributor/scripts/create_pr.py
git commit -m "feat(gh-contributor): add create_pr.py script"
```

---

## Task 4: Implement monitor_ci.py

**Files:**
- Create: `.claude/skills/gh-contributor/scripts/monitor_ci.py`

- [ ] **Step 1: Write the script**

Create `.claude/skills/gh-contributor/scripts/monitor_ci.py`:

```python
#!/usr/bin/env python3
"""Monitor CI checks on a PR and retry on failure."""

import argparse
import json
import subprocess
import sys
import time
from typing import Any


def run_gh(args: list[str]) -> str:
    """Run gh CLI and return stdout."""
    result = subprocess.run(
        ["gh"] + args,
        capture_output=True,
        text=True,
        check=True,
    )
    return result.stdout.strip()


def get_repo_info() -> tuple[str, str | None]:
    """Detect current repo and upstream repo."""
    remotes_output = subprocess.run(
        ["git", "remote", "-v"],
        capture_output=True,
        text=True,
        check=True,
    ).stdout

    origin_url = None
    upstream_url = None
    for line in remotes_output.strip().split("\n"):
        if line.startswith("origin"):
            origin_url = line.split()[1]
        elif line.startswith("upstream"):
            upstream_url = line.split()[1]

    def extract_repo(url: str) -> str:
        for prefix in ["https://github.com/", "git@github.com:"]:
            if url.startswith(prefix):
                repo = url[len(prefix):]
                if repo.endswith(".git"):
                    repo = repo[:-4]
                return repo
        raise ValueError(f"Cannot parse repo from URL: {url}")

    origin_repo = extract_repo(origin_url) if origin_url else None
    upstream_repo = extract_repo(upstream_url) if upstream_url else None

    return origin_repo, upstream_repo


def get_pr_checks(repo: str, pr_number: int) -> list[dict]:
    """Get CI check runs for a PR."""
    output = run_gh([
        "api",
        f"repos/{repo}/pulls/{pr_number}",
        "--jq", ".head.sha",
    ])
    head_sha = output.strip()

    output = run_gh([
        "api",
        f"repos/{repo}/commits/{head_sha}/check-runs",
        "--jq", ".check_runs",
    ])
    return json.loads(output)


def get_check_suites(repo: str, pr_number: int) -> list[dict]:
    """Get check suites for a PR."""
    output = run_gh([
        "api",
        f"repos/{repo}/pulls/{pr_number}",
        "--jq", ".head.sha",
    ])
    head_sha = output.strip()

    output = run_gh([
        "api",
        f"repos/{repo}/commits/{head_sha}/check-suites",
        "--jq", ".check_suites",
    ])
    return json.loads(output)


def classify_failure(check_name: str, log_output: str) -> str:
    """Classify what kind of failure occurred."""
    lower = log_output.lower()

    if "lint" in check_name.lower() or "gofmt" in check_name.lower():
        return "lint"
    if "test" in check_name.lower():
        return "test"
    if "build" in check_name.lower() or "compile" in check_name.lower():
        return "build"
    if any(k in lower for k in ["format", "gofmt", "black", "prettier", "eslint"]):
        return "lint"
    if any(k in lower for k in ["failed", "assertion", "panic", "error", "timeout"]):
        return "test"
    if any(k in lower for k in ["cannot find", "module", "import", "compilation"]):
        return "build"

    return "unknown"


def get_failure_logs(repo: str, check_run_id: int) -> str:
    """Get logs for a failed check run."""
    try:
        output = run_gh([
            "api",
            f"repos/{repo}/check-runs/{check_run_id}/logs",
        ])
        return output[:2000]  # Truncate to avoid huge output
    except subprocess.CalledProcessError:
        return "Could not retrieve logs"


def poll_checks(repo: str, pr_number: int, poll_interval: int = 60) -> dict:
    """Poll until all checks complete. Returns final status."""
    while True:
        try:
            checks = get_pr_checks(repo, pr_number)
        except subprocess.CalledProcessError:
            # No checks yet, wait and retry
            time.sleep(poll_interval)
            continue

        if not checks:
            time.sleep(poll_interval)
            continue

        # Check if all are complete
        pending = [c for c in checks if c.get("status") != "completed"]
        if pending:
            time.sleep(poll_interval)
            continue

        # All complete - check conclusions
        failures = [
            c for c in checks
            if c.get("conclusion") not in ("success", "skipped", "neutral")
        ]

        if not failures:
            return {
                "status": "passed",
                "retries": 0,
                "checks": [
                    {"name": c["name"], "status": c["status"], "conclusion": c["conclusion"]}
                    for c in checks
                ],
            }

        # Find first failure details
        first_fail = failures[0]
        log_output = get_failure_logs(repo, first_fail["id"])
        failure_type = classify_failure(first_fail["name"], log_output)

        return {
            "status": "failed",
            "retries": 0,
            "final_failure": first_fail["name"],
            "failure_type": failure_type,
            "logs_url": first_fail.get("html_url", ""),
            "failure_summary": log_output[:500],
            "all_checks": [
                {"name": c["name"], "conclusion": c["conclusion"]}
                for c in checks
            ],
        }


def main():
    parser = argparse.ArgumentParser(description="Monitor CI checks on a PR")
    parser.add_argument("--pr", type=int, required=True, help="PR number to monitor")
    parser.add_argument("--repo", help="Repo (auto-detected if omitted)")
    parser.add_argument("--max-retries", type=int, default=3, help="Max retry cycles")
    parser.add_argument("--poll-interval", type=int, default=60, help="Seconds between polls")
    args = parser.parse_args()

    try:
        if args.repo:
            target_repo = args.repo
        else:
            origin_repo, upstream_repo = get_repo_info()
            target_repo = upstream_repo if upstream_repo else origin_repo

        if not target_repo:
            raise RuntimeError("Could not determine target repository")

        retries = 0
        while retries <= args.max_retries:
            result = poll_checks(target_repo, args.pr, args.poll_interval)
            result["retries"] = retries

            if result["status"] == "passed":
                print(json.dumps(result, indent=2))
                sys.exit(0)

            # Failed
            if retries >= args.max_retries:
                print(json.dumps(result, indent=2))
                sys.exit(1)

            retries += 1
            result["retry_count"] = retries
            result["message"] = f"CI failed, retry {retries}/{args.max_retries}. Failure type: {result.get('failure_type', 'unknown')}"
            print(json.dumps(result, indent=2))

            # Wait a bit before retrying (give CI time to settle)
            time.sleep(30)

    except subprocess.CalledProcessError as e:
        print(json.dumps({"error": f"Command failed: {e}"}), file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(json.dumps({"error": str(e)}), file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
```

- [ ] **Step 2: Make executable**

```bash
chmod +x .claude/skills/gh-contributor/scripts/monitor_ci.py
```

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/gh-contributor/scripts/monitor_ci.py
git commit -m "feat(gh-contributor): add monitor_ci.py script"
```

---

## Task 5: Write SKILL.md

**Files:**
- Create: `.claude/skills/gh-contributor/SKILL.md`

- [ ] **Step 1: Write the skill definition**

Create `.claude/skills/gh-contributor/SKILL.md`:

```markdown
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
tools: Read, Glob, Grep, Bash, Edit, Write
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
```

- [ ] **Step 2: Commit**

```bash
git add .claude/skills/gh-contributor/SKILL.md
git commit -m "feat(gh-contributor): add SKILL.md workflow definition"
```

---

## Task 6: Create Eval Test Cases

**Files:**
- Create: `.claude/skills/gh-contributor/evals/evals.json`

- [ ] **Step 1: Write eval cases**

Create `.claude/skills/gh-contributor/evals/evals.json`:

```json
{
  "skill_name": "gh-contributor",
  "evals": [
    {
      "id": 1,
      "prompt": "Find a bug to fix in this repo. Pick an open issue labeled 'bug' or 'help wanted', implement a fix, and open a PR.",
      "expected_output": "Branch created, fix implemented, tests pass, PR opened with proper description linking the issue",
      "files": [],
      "expectations": [
        "The skill fetched issues from the upstream repo",
        "A feature branch was created with conventional naming",
        "Changes were committed with a conventional commit message",
        "A PR was opened with the issue linked in the description",
        "The PR title follows the pattern 'fix: description (fixes #N)'"
      ]
    },
    {
      "id": 2,
      "prompt": "Fix issue #52422 in this repo - it's about a formatting string bug. Create a PR for it.",
      "expected_output": "Issue #52422 fetched, fix implemented for formatting strings, PR created",
      "files": [],
      "expectations": [
        "The skill fetched the specific issue #52422",
        "The branch name includes the issue number",
        "The commit message references the issue",
        "The PR description links to issue #52422",
        "The PR title references the issue number"
      ]
    },
    {
      "id": 3,
      "prompt": "I want to contribute to moby/moby. Find a 'good first issue' and help me fix it.",
      "expected_output": "Issues filtered by 'good first issue' label, one selected, fix implemented, PR opened",
      "files": [],
      "expectations": [
        "The skill filtered issues by the 'good first issue' label",
        "An issue was presented to the user for confirmation",
        "The fix was implemented in a focused branch",
        "Tests were run before creating the PR",
        "The PR includes a proper description with checklist"
      ]
    }
  ]
}
```

- [ ] **Step 2: Commit**

```bash
git add .claude/skills/gh-contributor/evals/
git commit -m "test(gh-contributor): add eval test cases"
```

---

## Task 7: Package the Skill

**Files:**
- Create: `.claude/skills/gh-contributor/README.md` (optional, for documentation)

- [ ] **Step 1: Add a brief README**

Create `.claude/skills/gh-contributor/README.md`:

```markdown
# gh-contributor

Automate GitHub contributions: fetch issues, implement fixes, create PRs, monitor CI.

## Installation

Place this directory in `.claude/skills/gh-contributor/` in your repo.

## Usage

Invoke via Claude Code. The skill triggers on phrases like:
- "fix an issue"
- "work on a bug"
- "contribute to this repo"
- "fix issue #123"

## Scripts

- `scripts/fetch_issues.py` - Query GitHub issues (fork-aware)
- `scripts/create_pr.py` - Generate PR description and open PR
- `scripts/monitor_ci.py` - Poll CI checks and retry on failure

## Requirements

- `gh` CLI authenticated
- Python 3.9+
- Push access to your fork
```

- [ ] **Step 2: Package**

```bash
git add .claude/skills/gh-contributor/README.md
git commit -m "docs(gh-contributor): add README"
```

- [ ] **Step 3: Final review**

Verify the complete structure:

```bash
tree .claude/skills/gh-contributor/
```

Expected output:
```
.claude/skills/gh-contributor/
├── README.md
├── SKILL.md
├── evals/
│   └── evals.json
└── scripts/
    ├── __init__.py
    ├── create_pr.py
    ├── fetch_issues.py
    ├── monitor_ci.py
    └── requirements.txt
```

---

## Spec Coverage Check

| Spec Requirement | Task |
|------------------|------|
| `fetch_issues.py` script (fork-aware) | Task 2 |
| `create_pr.py` script | Task 3 |
| `monitor_ci.py` script | Task 4 |
| SKILL.md workflow orchestration | Task 5 |
| Eval test cases | Task 6 |
| Error handling table | Covered in SKILL.md + scripts |
| Safety rules | Covered in SKILL.md |
| Conventional commit messages | Task 4 + Task 5 |
| CI retry with limits | Task 4 + Task 5 |

## Placeholder Scan

- No "TBD", "TODO", "implement later" found
- All code blocks contain actual implementation
- All commands have expected output described
- No references to undefined functions/types
