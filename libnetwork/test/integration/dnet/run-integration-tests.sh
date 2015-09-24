#!/usr/bin/env bash

set -e

export INTEGRATION_ROOT=./integration-tmp
export TMPC_ROOT=./integration-tmp/tmpc

if [ ! -d ${INTEGRATION_ROOT} ]; then
    mkdir -p ${INTEGRATION_ROOT}
    git clone https://github.com/sstephenson/bats.git ${INTEGRATION_ROOT}/bats
    ./integration-tmp/bats/install.sh ./integration-tmp
fi

if [ ! -d ${TMPC_ROOT} ]; then
    mkdir -p ${TMPC_ROOT}
    docker pull busybox:ubuntu
    docker export $(docker create busybox:ubuntu) > ${TMPC_ROOT}/busybox.tar
    mkdir -p ${TMPC_ROOT}/rootfs
    tar -C ${TMPC_ROOT}/rootfs -xf ${TMPC_ROOT}/busybox.tar
fi

declare -A cmap

trap "cleanup_containers" EXIT SIGINT

function cleanup_containers() {
    for c in "${!cmap[@]}";
    do
	docker stop $c || true
	if [ -z "$CIRCLECI" ]; then
	    docker rm -f $c || true
	fi
    done

    unset cmap
}

source ./test/integration/dnet/helpers.bash

# Suite setup
start_consul 1>>${INTEGRATION_ROOT}/test.log 2>&1
cmap[pr_consul]=pr_consul

# Test dnet configuration options
./integration-tmp/bin/bats ./test/integration/dnet/dnet.bats

# Test a single node configuration with a global scope test driver

## Setup
start_dnet 1 simple 1>>${INTEGRATION_ROOT}/test.log 2>&1
cmap[dnet-1-simple]=dnet-1-simple

## Run the test cases
./integration-tmp/bin/bats ./test/integration/dnet/simple.bats

## Teardown
stop_dnet 1 simple 1>>${INTEGRATION_ROOT}/test.log 2>&1
unset cmap[dnet-1-simple]

# Test multi node configuration with a global scope test driver

## Setup
start_dnet 1 multi 1>>${INTEGRATION_ROOT}/test.log 2>&1
cmap[dnet-1-multi]=dnet-1-multi
start_dnet 2 multi 1>>${INTEGRATION_ROOT}/test.log 2>&1
cmap[dnet-2-multi]=dnet-2-multi
start_dnet 3 multi 1>>${INTEGRATION_ROOT}/test.log 2>&1
cmap[dnet-3-multi]=dnet-3-multi

## Run the test cases
./integration-tmp/bin/bats ./test/integration/dnet/multi.bats

## Teardown
stop_dnet 1 multi 1>>${INTEGRATION_ROOT}/test.log 2>&1
unset cmap[dnet-1-multi]
stop_dnet 2 multi 1>>${INTEGRATION_ROOT}/test.log 2>&1
unset cmap[dnet-2-multi]
stop_dnet 3 multi 1>>${INTEGRATION_ROOT}/test.log 2>&1
unset cmap[dnet-3-multi]

## Setup
start_dnet 1 overlay 1>>${INTEGRATION_ROOT}/test.log 2>&1
cmap[dnet-1-overlay]=dnet-1-overlay
start_dnet 2 overlay $(docker inspect --format '{{.NetworkSettings.IPAddress}}' dnet-1-overlay) 1>>${INTEGRATION_ROOT}/test.log 2>&1
cmap[dnet-2-overlay]=dnet-2-overlay
start_dnet 3 overlay $(docker inspect --format '{{.NetworkSettings.IPAddress}}' dnet-2-overlay) 1>>${INTEGRATION_ROOT}/test.log 2>&1
cmap[dnet-3-overlay]=dnet-3-overlay

## Run the test cases
./integration-tmp/bin/bats ./test/integration/dnet/overlay.bats

## Teardown
stop_dnet 1 overlay 1>>${INTEGRATION_ROOT}/test.log 2>&1
unset cmap[dnet-1-overlay]
stop_dnet 2 overlay 1>>${INTEGRATION_ROOT}/test.log 2>&1
unset cmap[dnet-2-overlay]
stop_dnet 3 overlay 1>>${INTEGRATION_ROOT}/test.log 2>&1
unset cmap[dnet-3-overlay]

# Suite teardowm
stop_consul 1>>${INTEGRATION_ROOT}/test.log 2>&1
unset cmap[pr_consul]
