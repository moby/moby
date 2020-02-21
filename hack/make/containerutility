#!/usr/bin/env bash
set -e

: "${CONTAINER_UTILITY_COMMIT:=aa1ba87e99b68e0113bd27ec26c60b88f9d4ccd9}"

(
	git clone https://github.com/docker/windows-container-utility.git "$GOPATH/src/github.com/docker/windows-container-utility"
	cd "$GOPATH/src/github.com/docker/windows-container-utility"
	git checkout -q "$CONTAINER_UTILITY_COMMIT"

	echo Building: ${DEST}/containerutility.exe

	(
		make
	)

	mkdir -p ${ABS_DEST}

	cp containerutility.exe ${ABS_DEST}/containerutility.exe
)
