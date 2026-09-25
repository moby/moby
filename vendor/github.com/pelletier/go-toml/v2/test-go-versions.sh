#!/usr/bin/env bash

set -uo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Go versions to test (1.11 through 1.26)
GO_VERSIONS=(
    "1.11"
    "1.12"
    "1.13"
    "1.14"
    "1.15"
    "1.16"
    "1.17"
    "1.18"
    "1.19"
    "1.20"
    "1.21"
    "1.22"
    "1.23"
    "1.24"
    "1.25"
    "1.26"
)

# Default values
PARALLEL=true
VERBOSE=false
OUTPUT_DIR="test-results"
DOCKER_TIMEOUT="10m"

usage() {
    cat << EOF
Usage: $0 [OPTIONS] [GO_VERSIONS...]

Test go-toml across multiple Go versions using Docker containers.

The script reports the lowest continuous supported Go version (where all subsequent 
versions pass) and only exits with non-zero status if either of the two most recent 
Go versions fail, indicating immediate attention is needed.

Note: For Go versions < 1.21, the script automatically updates go.mod to match the 
target version, but older versions may still fail due to missing standard library 
features (e.g., the 'slices' package introduced in Go 1.21).

OPTIONS:
    -h, --help          Show this help message
    -s, --sequential    Run tests sequentially instead of in parallel
    -v, --verbose       Enable verbose output
    -o, --output DIR    Output directory for test results (default: test-results)
    -t, --timeout TIME  Docker timeout for each test (default: 10m)
    --list              List available Go versions and exit

ARGUMENTS:
    GO_VERSIONS         Specific Go versions to test (default: all supported versions)
                        Examples: 1.21 1.22 1.23

EXAMPLES:
    $0                          # Test all Go versions in parallel
    $0 --sequential             # Test all Go versions sequentially
    $0 1.21 1.22 1.23          # Test specific versions
    $0 --verbose --output ./results 1.25 1.26  # Verbose output to custom directory

EXIT CODES:
    0                   Recent Go versions pass (good compatibility)
    1                   Recent Go versions fail (needs attention) or script error

EOF
}

log() {
    echo -e "${BLUE}[$(date +'%H:%M:%S')]${NC} $*" >&2
}

log_success() {
    echo -e "${GREEN}[$(date +'%H:%M:%S')] ✓${NC} $*" >&2
}

log_error() {
    echo -e "${RED}[$(date +'%H:%M:%S')] ✗${NC} $*" >&2
}

log_warning() {
    echo -e "${YELLOW}[$(date +'%H:%M:%S')] ⚠${NC} $*" >&2
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            usage
            exit 0
            ;;
        -s|--sequential)
            PARALLEL=false
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -o|--output)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        -t|--timeout)
            DOCKER_TIMEOUT="$2"
            shift 2
            ;;
        --list)
            echo "Available Go versions:"
            printf '%s\n' "${GO_VERSIONS[@]}"
            exit 0
            ;;
        -*)
            echo "Unknown option: $1" >&2
            usage
            exit 1
            ;;
        *)
            # Remaining arguments are Go versions
            break
            ;;
    esac
done

# If specific versions provided, use those instead of defaults
if [[ $# -gt 0 ]]; then
    GO_VERSIONS=("$@")
fi

# Validate Go versions
for version in "${GO_VERSIONS[@]}"; do
    if ! [[ "$version" =~ ^1\.(1[1-9]|2[0-6])$ ]]; then
        log_error "Invalid Go version: $version. Supported versions: 1.11-1.26"
        exit 1
    fi
done

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    log_error "Docker is required but not installed or not in PATH"
    exit 1
fi

# Check if Docker daemon is running
if ! docker info &> /dev/null; then
    log_error "Docker daemon is not running"
    exit 1
fi

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Function to test a single Go version
test_go_version() {
    local go_version="$1"
    local container_name="go-toml-test-${go_version}"
    local result_file="${OUTPUT_DIR}/go-${go_version}.txt"
    local dockerfile_content

    log "Testing Go $go_version..."

    # Create a temporary Dockerfile for this version
    # For Go versions < 1.21, we need to update go.mod to match the Go version
    local needs_go_mod_update=false
    if [[ $(echo "$go_version 1.21" | tr ' ' '\n' | sort -V | head -n1) == "$go_version" && "$go_version" != "1.21" ]]; then
        needs_go_mod_update=true
    fi
    
    dockerfile_content="FROM golang:${go_version}-alpine

# Install git (required for go mod)
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy source code
COPY . ."

    # Add go.mod update step for older Go versions
    if [[ "$needs_go_mod_update" == true ]]; then
        dockerfile_content="$dockerfile_content

# Update go.mod to match Go version (required for Go < 1.21)
RUN if [ -f go.mod ]; then sed -i 's/^go [0-9]\\+\\.[0-9]\\+\\(\\.[0-9]\\+\\)\\?/go $go_version/' go.mod; fi

# Note: Go versions < 1.21 may fail due to missing standard library packages (e.g., slices)
# This is expected for projects that use Go 1.21+ features"
    fi
    
    dockerfile_content="$dockerfile_content

# Run tests
CMD [\"sh\", \"-c\", \"go version && echo '--- Running go test ./... ---' && go test ./...\"]"

    # Create temporary directory for this test
    local temp_dir
    temp_dir=$(mktemp -d)

    # Copy source to temp directory (excluding test results and git)
    rsync -a --exclude="$OUTPUT_DIR" --exclude=".git" --exclude="*.test" . "$temp_dir/"

    # Create Dockerfile in temp directory
    echo "$dockerfile_content" > "$temp_dir/Dockerfile"

    # Build and run container
    local exit_code=0
    local output

    if $VERBOSE; then
        log "Building Docker image for Go $go_version..."
    fi

    # Capture both stdout and stderr, and the exit code
    if output=$(cd "$temp_dir" && timeout "$DOCKER_TIMEOUT" docker build -t "$container_name" . 2>&1 && \
                timeout "$DOCKER_TIMEOUT" docker run --rm "$container_name" 2>&1); then
        log_success "Go $go_version: PASSED"
        echo "PASSED" > "${result_file}.status"
    else
        exit_code=$?
        log_error "Go $go_version: FAILED (exit code: $exit_code)"
        echo "FAILED" > "${result_file}.status"
    fi

    # Save full output
    echo "$output" > "$result_file"

    # Clean up
    docker rmi "$container_name" &> /dev/null || true
    rm -rf "$temp_dir"

    if $VERBOSE; then
        echo "--- Go $go_version output ---"
        echo "$output"
        echo "--- End Go $go_version output ---"
    fi

    return $exit_code
}

# Function to run tests in parallel
run_parallel() {
    local pids=()
    local failed_versions=()

    log "Starting parallel tests for ${#GO_VERSIONS[@]} Go versions..."

    # Start all tests in background
    for version in "${GO_VERSIONS[@]}"; do
        test_go_version "$version" &
        pids+=($!)
    done

    # Wait for all tests to complete
    for i in "${!pids[@]}"; do
        local pid=${pids[$i]}
        local version=${GO_VERSIONS[$i]}

        if ! wait $pid; then
            failed_versions+=("$version")
        fi
    done

    return ${#failed_versions[@]}
}

# Function to run tests sequentially
run_sequential() {
    local failed_versions=()

    log "Starting sequential tests for ${#GO_VERSIONS[@]} Go versions..."

    for version in "${GO_VERSIONS[@]}"; do
        if ! test_go_version "$version"; then
            failed_versions+=("$version")
        fi
    done

    return ${#failed_versions[@]}
}

# Main execution
main() {
    local start_time
    start_time=$(date +%s)

    log "Starting Go version compatibility tests..."
    log "Testing versions: ${GO_VERSIONS[*]}"
    log "Output directory: $OUTPUT_DIR"
    log "Parallel execution: $PARALLEL"

    local failed_count
    if $PARALLEL; then
        run_parallel
        failed_count=$?
    else
        run_sequential
        failed_count=$?
    fi

    local end_time
    end_time=$(date +%s)
    local duration=$((end_time - start_time))

    # Collect results for display
    local passed_versions=()
    local failed_versions=()
    local unknown_versions=()
    local passed_count=0

    for version in "${GO_VERSIONS[@]}"; do
        local status_file="${OUTPUT_DIR}/go-${version}.txt.status"
        if [[ -f "$status_file" ]]; then
            local status
            status=$(cat "$status_file")
            if [[ "$status" == "PASSED" ]]; then
                passed_versions+=("$version")
                ((passed_count++))
            else
                failed_versions+=("$version")
            fi
        else
            unknown_versions+=("$version")
        fi
    done

    # Generate summary report
    local summary_file="${OUTPUT_DIR}/summary.txt"
    {
        echo "Go Version Compatibility Test Summary"
        echo "====================================="
        echo "Date: $(date)"
        echo "Duration: ${duration}s"
        echo "Parallel: $PARALLEL"
        echo ""
        echo "Results:"

        for version in "${GO_VERSIONS[@]}"; do
            local status_file="${OUTPUT_DIR}/go-${version}.txt.status"
            if [[ -f "$status_file" ]]; then
                local status
                status=$(cat "$status_file")
                if [[ "$status" == "PASSED" ]]; then
                    echo "  Go $version: ✓ PASSED"
                else
                    echo "  Go $version: ✗ FAILED"
                fi
            else
                echo "  Go $version: ? UNKNOWN (no status file)"
            fi
        done

        echo ""
        echo "Summary: $passed_count/${#GO_VERSIONS[@]} versions passed"

        if [[ $failed_count -gt 0 ]]; then
            echo ""
            echo "Failed versions details:"
            for version in "${failed_versions[@]}"; do
                echo ""
                echo "--- Go $version (FAILED) ---"
                local result_file="${OUTPUT_DIR}/go-${version}.txt"
                if [[ -f "$result_file" ]]; then
                    tail -n 30 "$result_file"
                fi
            done
        fi
    } > "$summary_file"

        # Find lowest continuous supported version and check recent versions
    local lowest_continuous_version=""
    local recent_versions_failed=false
    
    # Sort versions to ensure proper order
    local sorted_versions=()
    for version in "${GO_VERSIONS[@]}"; do
        sorted_versions+=("$version")
    done
    # Sort versions numerically (1.11, 1.12, ..., 1.25)
    IFS=$'\n' sorted_versions=($(sort -V <<< "${sorted_versions[*]}"))
    
    # Find lowest continuous supported version (all versions from this point onwards pass)
    for version in "${sorted_versions[@]}"; do
        local status_file="${OUTPUT_DIR}/go-${version}.txt.status"
        local all_subsequent_pass=true
        
        # Check if this version and all subsequent versions pass
        local found_current=false
        for check_version in "${sorted_versions[@]}"; do
            if [[ "$check_version" == "$version" ]]; then
                found_current=true
            fi
            
            if [[ "$found_current" == true ]]; then
                local check_status_file="${OUTPUT_DIR}/go-${check_version}.txt.status"
                if [[ -f "$check_status_file" ]]; then
                    local status
                    status=$(cat "$check_status_file")
                    if [[ "$status" != "PASSED" ]]; then
                        all_subsequent_pass=false
                        break
                    fi
                else
                    all_subsequent_pass=false
                    break
                fi
            fi
        done
        
        if [[ "$all_subsequent_pass" == true ]]; then
            lowest_continuous_version="$version"
            break
        fi
    done
    
    # Check if the two most recent versions failed
    local num_versions=${#sorted_versions[@]}
    if [[ $num_versions -ge 2 ]]; then
        local second_recent="${sorted_versions[$((num_versions-2))]}"
        local most_recent="${sorted_versions[$((num_versions-1))]}"
        
        local second_recent_status_file="${OUTPUT_DIR}/go-${second_recent}.txt.status"
        local most_recent_status_file="${OUTPUT_DIR}/go-${most_recent}.txt.status"
        
        local second_recent_failed=false
        local most_recent_failed=false
        
        if [[ -f "$second_recent_status_file" ]]; then
            local status
            status=$(cat "$second_recent_status_file")
            if [[ "$status" != "PASSED" ]]; then
                second_recent_failed=true
            fi
        else
            second_recent_failed=true
        fi
        
        if [[ -f "$most_recent_status_file" ]]; then
            local status
            status=$(cat "$most_recent_status_file")
            if [[ "$status" != "PASSED" ]]; then
                most_recent_failed=true
            fi
        else
            most_recent_failed=true
        fi
        
        if [[ "$second_recent_failed" == true || "$most_recent_failed" == true ]]; then
            recent_versions_failed=true
        fi
    elif [[ $num_versions -eq 1 ]]; then
        # Only one version tested, check if it's the most recent and failed
        local only_version="${sorted_versions[0]}"
        local only_status_file="${OUTPUT_DIR}/go-${only_version}.txt.status"
        
        if [[ -f "$only_status_file" ]]; then
            local status
            status=$(cat "$only_status_file")
            if [[ "$status" != "PASSED" ]]; then
                recent_versions_failed=true
            fi
        else
            recent_versions_failed=true
        fi
    fi
    
    # Display summary
    echo ""
    log "Test completed in ${duration}s"
    log "Summary report: $summary_file"
    
    echo ""
    echo "========================================"
    echo "           FINAL RESULTS"
    echo "========================================"
    echo ""
    
    # Display passed versions
    if [[ ${#passed_versions[@]} -gt 0 ]]; then
        log_success "PASSED (${#passed_versions[@]}/${#GO_VERSIONS[@]}):"
        # Sort passed versions for display
        local sorted_passed=()
        for version in "${sorted_versions[@]}"; do
            for passed_version in "${passed_versions[@]}"; do
                if [[ "$version" == "$passed_version" ]]; then
                    sorted_passed+=("$version")
                    break
                fi
            done
        done
        for version in "${sorted_passed[@]}"; do
            echo -e "  ${GREEN}✓${NC} Go $version"
        done
        echo ""
    fi
    
    # Display failed versions
    if [[ ${#failed_versions[@]} -gt 0 ]]; then
        log_error "FAILED (${#failed_versions[@]}/${#GO_VERSIONS[@]}):"
        # Sort failed versions for display
        local sorted_failed=()
        for version in "${sorted_versions[@]}"; do
            for failed_version in "${failed_versions[@]}"; do
                if [[ "$version" == "$failed_version" ]]; then
                    sorted_failed+=("$version")
                    break
                fi
            done
        done
        for version in "${sorted_failed[@]}"; do
            echo -e "  ${RED}✗${NC} Go $version"
        done
        echo ""
        
        # Show failure details
        echo "========================================"
        echo "         FAILURE DETAILS"
        echo "========================================"
        echo ""
        
        for version in "${sorted_failed[@]}"; do
            echo -e "${RED}--- Go $version FAILURE LOGS (last 30 lines) ---${NC}"
            local result_file="${OUTPUT_DIR}/go-${version}.txt"
            if [[ -f "$result_file" ]]; then
                tail -n 30 "$result_file" | sed 's/^/  /'
            else
                echo "  No log file found: $result_file"
            fi
            echo ""
        done
    fi
    
    # Display unknown versions
    if [[ ${#unknown_versions[@]} -gt 0 ]]; then
        log_warning "UNKNOWN (${#unknown_versions[@]}/${#GO_VERSIONS[@]}):"
        for version in "${unknown_versions[@]}"; do
            echo -e "  ${YELLOW}?${NC} Go $version (no status file)"
        done
        echo ""
    fi
    
    echo "========================================"
    echo "         COMPATIBILITY SUMMARY"
    echo "========================================"
    echo ""
    
    if [[ -n "$lowest_continuous_version" ]]; then
        log_success "Lowest continuous supported version: Go $lowest_continuous_version"
        echo "  (All versions from Go $lowest_continuous_version onwards pass)"
    else
        log_error "No continuous version support found"
        echo "  (No version has all subsequent versions passing)"
    fi
    
    echo ""
    echo "========================================"
    echo "Full detailed logs available in: $OUTPUT_DIR"
    echo "========================================"
    
    # Determine exit code based on recent versions
    if [[ "$recent_versions_failed" == true ]]; then
        log_error "OVERALL RESULT: Recent Go versions failed - this needs attention!"
        if [[ -n "$lowest_continuous_version" ]]; then
            echo "Note: Continuous support starts from Go $lowest_continuous_version"
        fi
        exit 1
    else
        log_success "OVERALL RESULT: Recent Go versions pass - compatibility looks good!"
        if [[ -n "$lowest_continuous_version" ]]; then
            echo "Continuous support starts from Go $lowest_continuous_version"
        fi
        exit 0
    fi
}

# Trap to clean up on exit
cleanup() {
    # Kill any remaining background processes
    jobs -p | xargs -r kill 2>/dev/null || true

    # Clean up any remaining Docker containers
    docker ps -q --filter "name=go-toml-test-" | xargs -r docker stop 2>/dev/null || true
    docker images -q --filter "reference=go-toml-test-*" | xargs -r docker rmi 2>/dev/null || true
}

trap cleanup EXIT

# Run main function
main
