#!/usr/bin/env bash
#
# Generate man pages for docker/docker
#

set -eu

mkdir -p ./man/man1

# Generate man pages from cobra commands
go build -o /tmp/gen-manpages ./man
/tmp/gen-manpages --root . --target ./man/man1

# Generate legacy pages from markdown
./man/md2man-all.sh -q
