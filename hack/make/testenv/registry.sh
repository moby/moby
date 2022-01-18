#!/usr/bin/env bash

set -e -x -u -o pipefail

: ${REGISTRY_REPO:="https://github.com/docker/distribution"}
: ${REGISTRY_COMMIT:=47a064d4195a9b56133891bbb13620c3ac83a827}
: ${REGISTRY_COMMIT_SCHEMA1:=ec87e9b6971d831f0eff752ddb54fb64693e51cd}


(
	export GO111MODULE=off
	export GOPATH="$(mktemp -d)"
	trap "rm -rf \"${GOPATH}\"" EXIT

	dir="${GOPATH}/src/github.com/docker/distribution"
	mkdir -p "${dir}"
	cd "${dir}"

	git clone --depth=1 "${REGISTRY_REPO}" .

	git fetch origin "${REGISTRY_COMMIT}"
	git checkout "${REGISTRY_COMMIT}"
	GOPATH="${GOPATH}/src/github.com/docker/distribution/Godeps/_workspace:${GOPATH}" go build -buildmode=pie -o "${PREFIX}/registry-v2" github.com/docker/distribution/cmd/registry

	git fetch origin "${REGISTRY_COMMIT_SCHEMA1}"
	git checkout "${REGISTRY_COMMIT_SCHEMA1}"
	GOPATH="${GOPATH}/src/github.com/docker/distribution/Godeps/_workspace:${GOPATH}" go build -buildmode=pie -o "${PREFIX}/registry-v2-schema1" github.com/docker/distribution/cmd/registry
)