#!/bin/bash
#
# Generate man pages for docker/docker
#

set -eu

mkdir -p ./yaml/yaml1

# Generate man pages from cobra commands
go build -o /tmp/gen-yamlfiles ./yaml
/tmp/gen-yamlfiles --root . --target ./yaml/yaml1
