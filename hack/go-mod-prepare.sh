#!/usr/bin/env bash

ROOTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cat > "${ROOTDIR}/go.mod" << EOF
module github.com/docker/docker

go 1.17
EOF

cat > "${ROOTDIR}/hack/make/.resources-windows/go.mod" << EOF
module github.com/docker/docker/autogen/winresources/dockerd

go 1.17
EOF
