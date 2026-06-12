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
        return output[:2000]
    except subprocess.CalledProcessError:
        return "Could not retrieve logs"


def poll_checks(repo: str, pr_number: int, poll_interval: int = 60) -> dict:
    """Poll until all checks complete. Returns final status."""
    while True:
        try:
            checks = get_pr_checks(repo, pr_number)
        except subprocess.CalledProcessError:
            time.sleep(poll_interval)
            continue

        if not checks:
            time.sleep(poll_interval)
            continue

        pending = [c for c in checks if c.get("status") != "completed"]
        if pending:
            time.sleep(poll_interval)
            continue

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

            if retries >= args.max_retries:
                print(json.dumps(result, indent=2))
                sys.exit(1)

            retries += 1
            result["retry_count"] = retries
            result["message"] = f"CI failed, retry {retries}/{args.max_retries}. Failure type: {result.get('failure_type', 'unknown')}"
            print(json.dumps(result, indent=2))
            time.sleep(30)

    except subprocess.CalledProcessError as e:
        print(json.dumps({"error": f"Command failed: {e}"}), file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(json.dumps({"error": str(e)}), file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
