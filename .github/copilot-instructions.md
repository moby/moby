# Moby Project Development Guide

Always reference these instructions first and fallback to search or bash commands only when you encounter unexpected information that does not match the info here.

## Critical Build and Test Environment Constraints

**IMPORTANT LIMITATIONS**: The development environment has significant constraints that prevent full Docker-based development container usage:
- Docker container builds fail due to SSL certificate verification issues when downloading external dependencies
- Integration tests require Docker daemon access which may not be available
- Some validation tools (shfmt, gotestsum, golangci-lint) are not installed in the base environment
- Privileged operations (chown, mount, network namespaces) are restricted

**DO NOT attempt the following**:
- `make build` or `make shell` (Docker container build will fail)
- Full integration test suites without Docker daemon access
- Commands requiring privileged operations or Docker-in-Docker

## Working Effectively

### Bootstrap and Basic Build (WORKING METHODS)
```bash
# Navigate to repository
cd /path/to/moby

# Download Go dependencies - takes 5-10 seconds
go mod download

# Build main binaries directly with Go (RECOMMENDED)
time go build ./cmd/dockerd        # ~6-7 seconds, produces dockerd binary
time go build ./cmd/docker-proxy   # ~0.3 seconds, produces docker-proxy binary

# Verify binaries work
./dockerd --version    # Should show version info
```

### Unit Testing
```bash
# Run unit tests for specific packages (FASTEST - ~1 second)
time go test -short ./pkg/...

# Run with specific test directory
TESTDIRS='./pkg/...' go test -short $TESTDIRS

# Run specific tests with timeout
go test -short -timeout=5m ./pkg/authorization

# NEVER CANCEL: Full unit test suite can take 4+ minutes
# Note: Many tests fail due to environment constraints (expected)
time go test -short ./...   # Takes ~4 minutes, set timeout to 10+ minutes
```

### Validation and Linting
```bash
# Format checking (takes ~2 seconds, mostly clean except vendor/)
gofmt -l .

# Go vet validation (takes ~1 second per package)
go vet ./cmd/dockerd
go vet ./pkg/...

# YAML linting (works but shows many style violations - takes ~40 seconds)
yamllint .   # NEVER CANCEL: Set timeout to 60+ minutes

# Check imports and basic structure
go list -f '{{ join .Deps "\n" }}' ./cmd/dockerd | head -10
```

## Manual Validation Scenarios

**ALWAYS test these scenarios after making changes**:

1. **Binary Build Validation**:
   ```bash
   # Clean previous builds
   rm -f dockerd docker-proxy
   
   # Build and verify
   go build ./cmd/dockerd && ./dockerd --version
   go build ./cmd/docker-proxy
   ```

2. **Unit Test Validation** (for code in pkg/):
   ```bash
   # Test specific package you modified
   go test -v ./pkg/[modified-package]
   
   # Test all pkg packages (limited but fast)
   go test -short ./pkg/...
   ```

3. **Code Quality Validation**:
   ```bash
   # Check formatting
   gofmt -l . | grep -v vendor | head -5
   
   # Verify imports work
   go list ./cmd/dockerd
   go list ./pkg/...
   ```

## Repository Structure and Navigation

### Key Directories
- `cmd/dockerd/` - Main Docker daemon binary
- `cmd/docker-proxy/` - Docker proxy binary  
- `daemon/` - Core daemon implementation
- `api/` - API definitions and client code
- `client/` - Docker client library
- `pkg/` - Shared packages (safest for testing)
- `integration/` - Integration tests (require Docker daemon)
- `hack/` - Build scripts and validation tools
- `.github/workflows/` - CI/CD pipeline definitions

### Build System
- `Makefile` - Main build interface (Docker-based, mostly non-functional in this environment)
- `hack/make.sh` - Build script (requires Docker container)
- `hack/test/unit` - Unit test script (requires gotestsum)
- `hack/validate/` - Validation scripts (many require special tools)

### Go Modules
```bash
# Main module
cat go.mod  # Shows github.com/moby/moby/v2 

# Submodules
ls */go.mod  # api/go.mod, client/go.mod, man/go.mod
```

## Common Validation Tasks

### Pre-commit Validation (USE THESE)
```bash
# Quick validation workflow (~10 seconds total)
gofmt -l . | grep -v vendor | head -5  # Check formatting
go vet ./cmd/dockerd                   # Vet main package
go build ./cmd/dockerd                 # Ensure it builds
go test -short ./pkg/...              # Test shared packages

# Extended validation (~4 minutes total) 
go test -short ./...                   # NEVER CANCEL: Set timeout to 10+ minutes
```

### Code Changes Validation
```bash
# After modifying daemon code
go build ./cmd/dockerd && ./dockerd --version

# After modifying pkg code  
go test -v ./pkg/[modified-package]

# After modifying client code
cd client && go test ./...
cd ..

# After modifying API definitions
cd api && go test ./...
cd ..
```

## Timing Expectations and Timeouts

**CRITICAL**: Always set appropriate timeouts and NEVER CANCEL these operations:

- `go mod download`: 5-10 seconds (rarely needed, cached)
- `go build ./cmd/dockerd`: 6-7 seconds (NEVER CANCEL - set 2+ minute timeout)
- `go build ./cmd/docker-proxy`: 0.3 seconds  
- `go test -short ./pkg/...`: 1 second (cached), 5-10 seconds (first run)
- `go test -short ./...`: 4+ minutes (NEVER CANCEL - set 10+ minute timeout)
- `gofmt -l .`: 2 seconds
- `yamllint .`: 40+ seconds (NEVER CANCEL - set 60+ minute timeout)

**WARNING**: Do not attempt these operations (they will fail):
- `make build`: Fails due to Docker build issues (~15+ minutes timeout, will fail)
- `make test`: Requires Docker container
- Integration tests: Require Docker daemon access

## Architectural Overview

Moby is a container platform written in Go with these key components:

- **dockerd**: Main daemon process managing containers, images, networks
- **docker-proxy**: Network proxy for container port mapping
- **containerd**: Container runtime (external dependency)
- **runc**: Low-level container runtime (external dependency)

### API Structure
- REST API defined in `api/` directory
- Client library in `client/` directory  
- Daemon implementation in `daemon/` directory
- Shared utilities in `pkg/` directory

### Testing Strategy
- Unit tests: Test individual packages and functions
- Integration tests: Test API interactions (require Docker daemon)
- End-to-end tests: Test complete workflows (require full environment)

## Environment Setup Notes

This repository uses:
- **Go 1.24.6** (verified working)
- **Docker 28.0.4** (CLI available, daemon access limited)
- **Module system**: Multiple Go modules (main, api, client, man)
- **Build tags**: Uses netgo, osusergo, static_build tags
- **Cross-compilation**: Supports multiple platforms

## Troubleshooting

### Build Issues
- If `go build` fails: Check Go version, run `go mod download`
- If tests fail with permission errors: Expected in restricted environment
- If Docker commands fail: Use Go build commands instead

### Test Issues  
- Network-related test failures: Expected (namespace restrictions)
- Permission errors: Expected (privilege restrictions)
- Docker API errors: Expected (daemon not accessible)

### Working Around Limitations
- Use `go test -short` to skip long-running tests
- Focus on `./pkg/...` tests which have fewer dependencies
- Use `go build` directly instead of Make targets
- Validate formatting with `gofmt` instead of full validation suite

Always validate your changes work with the core Go commands before submitting.