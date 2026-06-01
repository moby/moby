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
