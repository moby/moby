# Agent Guidelines for go-toml

This file provides guidelines for AI agents contributing to go-toml. All agents must follow these rules derived from [CONTRIBUTING.md](./CONTRIBUTING.md).

## Project Overview

go-toml is a TOML library for Go. The goal is to provide an easy-to-use and efficient TOML implementation that gets the job done without getting in the way.

## Code Change Rules

### Backward Compatibility

- **No backward-incompatible changes** unless explicitly discussed and approved
- Avoid breaking people's programs unless absolutely necessary

### Testing Requirements

- **All bug fixes must include regression tests**
- **All new code must be tested**
- Run tests before submitting: `go test -race ./...`
- Test coverage must not decrease. Check with:
  ```bash
  go test -covermode=atomic -coverprofile=coverage.out
  go tool cover -func=coverage.out
  ```
- All lines of code touched by changes should be covered by tests

### Performance Requirements

- go-toml aims to stay efficient; avoid performance regressions
- Run benchmarks to verify: `go test ./... -bench=. -count=10`
- Compare results using [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat)

### Documentation

- New features or feature extensions must include documentation
- Documentation lives in [README.md](./README.md) and throughout source code

### Code Style

- Follow existing code format and structure
- Code must pass `go fmt`
- Code must pass linting with the same golangci-lint version as CI (see version in `.github/workflows/lint.yml`):
  ```bash
  # Install specific version (check lint.yml for current version)
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin <version>
  # Run linter
  golangci-lint run ./...
  ```

### Commit Messages

- Commit messages must explain **why** the change is needed
- Keep messages clear and informative even if details are in the PR description

## Pull Request Checklist

Before submitting:

1. Tests pass (`go test -race ./...`)
2. No backward-incompatible changes (unless discussed)
3. Relevant documentation added/updated
4. No performance regression (verify with benchmarks)
5. Title is clear and understandable for changelog
