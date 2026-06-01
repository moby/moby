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


def get_diff(base: str) -> str:
    """Get diff against base branch."""
    try:
        return run_git(["diff", base, "...HEAD"])
    except subprocess.CalledProcessError:
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
    title = issue_title.strip()
    title = re.sub(r"\s*\(fixes?\s*#\d+\)", "", title, flags=re.IGNORECASE)
    title = re.sub(r"\s*#\d+", "", title)
    title = title.strip()

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

    issue_summary = issue_body.strip()
    if issue_summary:
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

        issue_repo = upstream_repo if upstream_repo else target_repo
        issue_body = get_issue_body(issue_repo, args.issue)

        diff = get_diff(base)
        changes = summarize_changes(diff)

        issue_data = json.loads(run_gh(["api", f"repos/{issue_repo}/issues/{args.issue}"]))
        issue_title = issue_data.get("title", "")

        pr_title = generate_pr_title(issue_title, args.issue)
        pr_body = generate_pr_body(args.issue, issue_body, changes)

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
