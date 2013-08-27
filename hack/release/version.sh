#!/bin/sh

# This script sets environment variables for the build version. It is used by the other build scripts.
#
# The following variables are set:
#
# - PKGVERSION
#   this is the build version. It is derived from the content of the VERSION file, at the root of the
#   repository. If the repository is not clean It also contains the GITCOMMIT and a timestamp
#
# - GITCOMMIT
#   The hash of the git commit with the suffix -dirty if the repository isn't clean.
#


VERSION=$(cat ./VERSION)
PKGVERSION="$VERSION"
GITCOMMIT=$(git rev-parse --short HEAD)
if test -n "$(git status --porcelain)"
then
	GITCOMMIT="$GITCOMMIT-dirty"
	PKGVERSION="$PKGVERSION-$(date +%Y%m%d%H%M%S)-$GITCOMMIT"
fi
