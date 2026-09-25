#!/bin/sh

set -e
version="$1"
repository="$2"
outdir="$3"

DOWNLOAD_URL="https://download.docker.com/linux/static/stable/$(xx-info march)/docker-${version#v}.tgz"

mkdir "$outdir"
if curl --head --silent --fail "${DOWNLOAD_URL}" 1> /dev/null 2>&1; then
	curl -fsSL "${DOWNLOAD_URL}" | tar -xz docker/docker
	mv docker/docker "${outdir}/docker"
else
	git init -q .
	git remote remove origin 2> /dev/null || true
	git remote add origin "${repository}"
	git fetch -q --depth 1 origin "${version}" +refs/tags/*:refs/tags/*
	git checkout -fq "${version}"
	if [ -d ./components/cli ]; then
		mv ./components/cli/* ./
		CGO_ENABLED=0 xx-go build -o "${outdir}/docker" ./cmd/docker
		git reset --hard "${version}"
	else
		xx-go --wrap && CGO_ENABLED=0 TARGET="${outdir}" ./scripts/build/binary
	fi
fi

xx-verify "${outdir}/docker"
