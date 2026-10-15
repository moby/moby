#!/bin/bash

# Script to run all bitset fuzz tests for a configurable duration or number of iterations.
# Usage: ./run_fuzz_tests.sh [duration/iterations]
# Examples:
#   ./run_fuzz_tests.sh          # Uses default 60s
#   ./run_fuzz_tests.sh 10s      # Run each test for 10 seconds
#   ./run_fuzz_tests.sh 1000x    # Run each test for 1000 iterations

# Set default duration/iterations if not provided
FUZZTIME=${1:-60s}

echo "Running BitSet fuzz tests for: $FUZZTIME"
echo ""

# List of all available fuzz tests
FUZZ_TESTS=(
    FuzzCapacityAndGrowth
    FuzzIterationConsistency
    FuzzStringRepresentations
    FuzzBasicOps
    FuzzRange
    FuzzSetOperations
    FuzzNavigation
    FuzzShift
    FuzzModification
    FuzzCopy
    FuzzSerialization
    FuzzRandomOperations
)

# Run each fuzz test
for test in "${FUZZ_TESTS[@]}"; do
    echo "Testing $test..."
    go test -fuzz=$test -fuzztime=$FUZZTIME
    echo ""
done

echo ""
echo "All available fuzz tests completed for: $FUZZTIME"
