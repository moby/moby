#!/usr/bin/env bash

set -e -o pipefail

if [ -z "$VALIDATE_UPSTREAM" ]; then
	# this is kind of an expensive check, so let's not do this twice if we
	# are running more than one validate bundlescript

	VALIDATE_REPO="${VALIDATE_REPO:-https://github.com/docker/docker.git}"
	VALIDATE_BRANCH="${VALIDATE_BRANCH:-master}"

	VALIDATE_HEAD="$(git rev-parse --verify HEAD)"

	if [ -z "$VALIDATE_ORIGIN_BRANCH" ]; then
		git fetch -q "$VALIDATE_REPO" "refs/heads/$VALIDATE_BRANCH"
		VALIDATE_ORIGIN_BRANCH=FETCH_HEAD
	fi
	VALIDATE_UPSTREAM="$(git rev-parse --verify $VALIDATE_ORIGIN_BRANCH)"

	VALIDATE_COMMIT_LOG="$VALIDATE_UPSTREAM..$VALIDATE_HEAD"
	VALIDATE_COMMIT_DIFF="$VALIDATE_UPSTREAM...$VALIDATE_HEAD"

	validate_diff() {
		if [ "$VALIDATE_UPSTREAM" != "$VALIDATE_HEAD" ]; then
			git diff "$VALIDATE_COMMIT_DIFF" "$@"
		fi
	}
	validate_log() {
		if [ "$VALIDATE_UPSTREAM" != "$VALIDATE_HEAD" ]; then
			git log "$VALIDATE_COMMIT_LOG" "$@"
		fi
	}
fi
