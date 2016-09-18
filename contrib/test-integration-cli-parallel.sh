#!/bin/bash
### Usage:
###     $ sudo apt install parallel
###     $ ./contrib/test-integration-cli-parallel.sh
set -e

: ${NJOBS=$(nproc)}
D=$(pwd)/bundles-parallel
RUNNER=$(pwd)/contrib/.test-integration-cli-parallel
PARALLEL=parallel

echo "Running tests in parallel, using $NJOBS job containers"
echo "See $D/results for the results"
rm -rf $D/results
mkdir -p $D
grep -oPh '^func .*\KTest[^(]+' integration-cli/*_test.go | sort > $D/input

$PARALLEL \
    --no-notice \
    --jobs $NJOBS \
    --max-args $(( $(wc -l < $D/input) / $NJOBS + 1)) \
    --pipe \
    --results $D/results \
    --joblog  $D/joblog \
    $RUNNER < $D/input
