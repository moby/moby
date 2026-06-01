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
