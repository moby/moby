#!/usr/bin/env bash
set -e

if [ $# -ne 1 ]; then
	echo >&2 "Usage: $0 PATH"
	echo >&2 "Show the primary and secondary maintainers for a given path"
	exit 1
fi

set -e

DEST=$1
DESTFILE=""
if [ ! -d $DEST ]; then
	DESTFILE=$(basename $DEST)
	DEST=$(dirname $DEST)
fi

MAINTAINERS=()
cd $DEST
while true; do
	if [ -e ./MAINTAINERS ]; then
		{
			while read line; do
				re='^([^:]*): *(.*)$'
				file=$(echo $line | sed -E -n "s/$re/\1/p")
				if [ ! -z "$file" ]; then
					if [ "$file" = "$DESTFILE" ]; then
						echo "Override: $line"
						maintainer=$(echo $line | sed -E -n "s/$re/\2/p")
						MAINTAINERS=("$maintainer" "${MAINTAINERS[@]}")
					fi
				else
					MAINTAINERS+=("$line");
				fi
			done;
		} < MAINTAINERS
		break
	fi
	if [ -d .git ]; then
		break
	fi
	if [ "$(pwd)" = "/" ]; then
		break
	fi
	cd ..
done

PRIMARY="${MAINTAINERS[0]}"
PRIMARY_FIRSTNAME=$(echo $PRIMARY | cut -d' ' -f1)
LGTM_COUNT=${#MAINTAINERS[@]}
LGTM_COUNT=$((LGTM_COUNT%2 +1))

firstname() {
	echo $1 | cut -d' ' -f1
}

echo "A pull request in $1 will need $LGTM_COUNT LGTM's to be merged."
echo "--- $PRIMARY is the PRIMARY MAINTAINER of $1."
for SECONDARY in "${MAINTAINERS[@]:1}"; do
	echo "--- $SECONDARY"
done
