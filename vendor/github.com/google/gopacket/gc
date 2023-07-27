#!/bin/bash
# Copyright 2012 Google, Inc. All rights reserved.

# This script provides a simple way to run benchmarks against previous code and
# keep a log of how benchmarks change over time.  When used with the --benchmark
# flag, it runs benchmarks from the current code and from the last commit run
# with --benchmark, then stores the results in the git commit description.  We
# rerun the old benchmarks along with the new ones, since there's no guarantee
# that git commits will happen on the same machine, so machine differences could
# cause wildly inaccurate results.
#
# If you're making changes to 'gopacket' which could cause performance changes,
# you may be requested to use this commit script to make sure your changes don't
# have large detrimental effects (or to show off how awesome your performance
# improvements are).
#
# If not run with the --benchmark flag, this script is still very useful... it
# makes sure all the correct go formatting, building, and testing work as
# expected.

function Usage {
  cat <<EOF
USAGE:  $0 [--benchmark regexp] [--root] [--gen] <git commit flags...>

--benchmark:  Run benchmark comparisons against last benchmark'd commit
--root:  Run tests that require root priviledges
--gen:  Generate code for MACs/ports by pulling down external data

Note, some 'git commit' flags are necessary, if all else fails, pass in -a
EOF
  exit 1
}

BENCH=""
GEN=""
ROOT=""
while [ ! -z "$1" ]; do
  case "$1" in
    "--benchmark")
      BENCH="$2"
      shift
      shift
      ;;
    "--gen")
      GEN="yes"
      shift
      ;;
    "--root")
      ROOT="yes"
      shift
      ;;
    "--help")
      Usage
      ;;
    "-h")
      Usage
      ;;
    "help")
      Usage
      ;;
    *)
      break
      ;;
  esac
done

function Root {
  if [ ! -z "$ROOT" ]; then
    local exec="$1"
    # Some folks (like me) keep source code in places inaccessible by root (like
    # NFS), so to make sure things run smoothly we copy them to a /tmp location.
    local tmpfile="$(mktemp -t gopacket_XXXXXXXX)"
    echo "Running root test executable $exec as $tmpfile"
    cp "$exec" "$tmpfile"
    chmod a+x "$tmpfile"
    shift
    sudo "$tmpfile" "$@"
  fi
}

if [ "$#" -eq "0" ]; then
  Usage
fi

cd $(dirname $0)

# Check for copyright notices.
for filename in $(find ./ -type f -name '*.go'); do
  if ! head -n 1 "$filename" | grep -q Copyright; then
    echo "File '$filename' may not have copyright notice"
    exit 1
  fi
done

set -e
set -x

if [ ! -z "$ROOT" ]; then
  echo "Running SUDO to get root priviledges for root tests"
  sudo echo "have root"
fi

if [ ! -z "$GEN" ]; then
  pushd macs
  go run gen.go | gofmt > valid_mac_prefixes.go
  popd
  pushd layers
  go run gen.go | gofmt > iana_ports.go
  go run gen2.go | gofmt > enums_generated.go
  popd
fi

# Make sure everything is formatted, compiles, and tests pass.
go fmt ./...
go test -i ./... 2>/dev/null >/dev/null || true
go test
go build
pushd examples/bytediff
go build
popd
if [ -f /usr/include/pcap.h ]; then
  pushd pcap
  go test ./...
  go build ./...
  go build pcap_tester.go
  Root pcap_tester --mode=basic
  Root pcap_tester --mode=filtered
  Root pcap_tester --mode=timestamp || echo "You might not support timestamp sources"
  popd
  pushd examples/afpacket
  go build
  popd
  pushd examples/pcapdump
  go build
  popd
  pushd examples/arpscan
  go build
  popd
  pushd examples/bidirectional
  go build
  popd
  pushd examples/synscan
  go build
  popd
  pushd examples/httpassembly
  go build
  popd
  pushd examples/statsassembly
  go build
  popd
fi
pushd macs
go test ./...
gofmt -w gen.go
go build gen.go
popd
pushd tcpassembly
go test ./...
popd
pushd reassembly
go test ./...
popd
pushd layers
gofmt -w gen.go
go build gen.go
go test ./...
popd
pushd pcapgo
go test ./...
go build ./...
popd
if [ -f /usr/include/linux/if_packet.h ]; then
  if grep -q TPACKET_V3 /usr/include/linux/if_packet.h; then
    pushd afpacket
    go build ./...
    go test ./...
    popd
  fi
fi
if [ -f /usr/include/pfring.h ]; then
  pushd pfring
  go test ./...
  go build ./...
  popd
  pushd examples/pfdump
  go build
  popd
fi
pushd ip4defrag
go test ./...
popd
pushd defrag
go test ./...
popd

for travis_script in `ls .travis.*.sh`; do
  ./$travis_script
done

# Run our initial commit
git commit "$@"

if [ -z "$BENCH" ]; then
  set +x
  echo "We're not benchmarking and we've committed... we're done!"
  exit
fi

### If we get here, we want to run benchmarks from current commit, and compare
### then to benchmarks from the last --benchmark commit.

# Get our current branch.
BRANCH="$(git branch | grep '^*' | awk '{print $2}')"

# File we're going to build our commit description in.
COMMIT_FILE="$(mktemp /tmp/tmp.XXXXXXXX)"

# Add the word "BENCH" to the start of the git commit.
echo -n "BENCH " > $COMMIT_FILE

# Get the current description... there must be an easier way.
git log -n 1 | grep '^ ' | sed 's/^    //' >> $COMMIT_FILE

# Get the commit sha for the last benchmark commit
PREV=$(git log -n 1 --grep='BENCHMARK_MARKER_DO_NOT_CHANGE' | head -n 1 | awk '{print $2}')

## Run current benchmarks

cat >> $COMMIT_FILE <<EOF


----------------------------------------------------------
BENCHMARK_MARKER_DO_NOT_CHANGE
----------------------------------------------------------

Go version $(go version)


TEST BENCHMARKS "$BENCH"
EOF
# go seems to have trouble with 'go test --bench=. ./...'
go test --test.bench="$BENCH" 2>&1 | tee -a $COMMIT_FILE
pushd layers
go test --test.bench="$BENCH" 2>&1 | tee -a $COMMIT_FILE
popd
cat >> $COMMIT_FILE <<EOF


PCAP BENCHMARK
EOF
if [ "$BENCH" -eq ".*" ]; then
  go run pcap/gopacket_benchmark/*.go 2>&1 | tee -a $COMMIT_FILE
fi



## Reset to last benchmark commit, run benchmarks

git checkout $PREV

cat >> $COMMIT_FILE <<EOF
----------------------------------------------------------
BENCHMARKING AGAINST COMMIT $PREV
----------------------------------------------------------


OLD TEST BENCHMARKS
EOF
# go seems to have trouble with 'go test --bench=. ./...'
go test --test.bench="$BENCH" 2>&1 | tee -a $COMMIT_FILE
pushd layers
go test --test.bench="$BENCH" 2>&1 | tee -a $COMMIT_FILE
popd
cat >> $COMMIT_FILE <<EOF


OLD PCAP BENCHMARK
EOF
if [ "$BENCH" -eq ".*" ]; then
  go run pcap/gopacket_benchmark/*.go 2>&1 | tee -a $COMMIT_FILE
fi



## Reset back to the most recent commit, edit the commit message by appending
## benchmark results.
git checkout $BRANCH
git commit --amend -F $COMMIT_FILE
