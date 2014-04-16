#!/bin/bash
set -e

# get into this script's directory
cd "$(dirname "$(readlink -f "$BASH_SOURCE")")"

[ "$1" = '-q' ] || {
	set -x
	pwd
}

mkdir -p ../man1

for FILE in docker*.md; do
	pandoc -s -t man "$FILE" -o "../man1/${FILE%.*}.1"
done
