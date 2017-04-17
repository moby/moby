#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

set -x
./generate.sh
for d in */; do
	docker build -t "dockercore/builder-rpm:$(basename "$d")" "$d"
done
