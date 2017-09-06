#!/bin/bash
set -o errexit

CDIR=$(cd `dirname "$0"` && pwd)
cd "$CDIR"

ORG_PATH="github.com/cloudflare"
REPO_PATH="${ORG_PATH}/cfssl"

export GOPATH="${CDIR}/gopath"

export PATH="${PATH}:${GOPATH}/bin"

eval $(go env)

if [ ! -h gopath/src/${REPO_PATH} ]; then
    mkdir -p gopath/src/${ORG_PATH}
    ln -s ../../../.. gopath/src/${REPO_PATH} || exit 255
fi

ls "${GOPATH}/src/${REPO_PATH}"

PACKAGES=""
if [ "$#" != 0 ]; then
    for pkg in "$@"; do
        PACKAGES="$PACKAGES $REPO_PATH/$pkg"
    done
else
    PACKAGES=$(go list ./... | grep -v /vendor/ | grep ^_)
    # Escape current cirectory
    CDIR_ESC=$(printf "%q" "$CDIR/")
    # Remove current directory from the package path
    PACKAGES=${PACKAGES//$CDIR_ESC/}
    # Remove underscores
    PACKAGES=${PACKAGES//_/}
    # split PACKAGES into an array and prepend REPO_PATH to each local package
    split=(${PACKAGES// / })
    PACKAGES=${split[@]/#/${REPO_PATH}/}
fi

go vet $PACKAGES
if ! which fgt > /dev/null ; then
    go get github.com/GeertJohan/fgt
fi

if ! which golint > /dev/null ; then
    go get github.com/golang/lint/golint
fi

for package in $PACKAGES
do
    fgt golint "${package}"
done

# check go fmt
for package in $PACKAGES
do
    test -z "$(gofmt -s -l $GOPATH/src/$package/ | tee /dev/stderr)"
done

# Build and install cfssl executable in PATH
go install -tags "$BUILD_TAGS" ${REPO_PATH}/cmd/cfssl

COVPROFILES=""
for package in $(go list -f '{{if len .TestGoFiles}}{{.ImportPath}}{{end}}' $PACKAGES)
do
    profile="$GOPATH/src/$package/.coverprofile"
    go test -race -tags "$BUILD_TAGS" --coverprofile=$profile $package
    [ -s $profile ] && COVPROFILES="$COVPROFILES $profile"
done
cat $COVPROFILES > coverprofile.txt
