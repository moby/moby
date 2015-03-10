#!/usr/bin/env bash
set -e

# This script runs all validations

validate() {
    sed -i 's!docker/docker!docker/libcontainer!' /go/src/github.com/docker/docker/hack/make/.validate
    bash /go/src/github.com/docker/docker/hack/make/validate-dco
    bash /go/src/github.com/docker/docker/hack/make/validate-gofmt
}

# run validations
validate
