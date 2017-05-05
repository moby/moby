#!/usr/bin/env bash
set -e

# get into this script's directory
cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")"

[ "$1" = '-q' ] || {
	set -x
	pwd
}

for FILE in *.md; do
	base="$(basename "$FILE")"
	name="${base%.md}"
	num="${name##*.}"
	if [ -z "$num" ] || [ "$name" = "$num" ]; then
		# skip files that aren't of the format xxxx.N.md (like README.md)
		continue
	fi
	mkdir -p "./man${num}"
	go-md2man -in "$FILE" -out "./man${num}/${name}"
done
