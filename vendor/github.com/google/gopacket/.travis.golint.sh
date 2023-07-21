#!/bin/bash

cd "$(dirname $0)"

go get golang.org/x/lint/golint
DIRS=". tcpassembly tcpassembly/tcpreader ip4defrag reassembly macs pcapgo pcap afpacket pfring routing defrag/lcmdefrag"
# Add subdirectories here as we clean up golint on each.
for subdir in $DIRS; do
  pushd $subdir
  if golint |
      grep -v CannotSetRFMon |  # pcap exported error name
      grep -v DataLost |        # tcpassembly/tcpreader exported error name
      grep .; then
    exit 1
  fi
  popd
done

pushd layers
for file in *.go; do
  if cat .lint_blacklist | grep -q $file; then
    echo "Skipping lint of $file due to .lint_blacklist"
  elif golint $file | grep .; then
    echo "Lint error in file $file"
    exit 1
  fi
done
popd
