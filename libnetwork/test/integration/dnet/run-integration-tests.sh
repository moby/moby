#!/usr/bin/env bash

set -e

export INTEGRATION_ROOT=./integration-tmp
export TMPC_ROOT=./integration-tmp/tmpc

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

function run_bridge_tests() {
    ## Setup
    start_dnet 1 bridge 1>>${INTEGRATION_ROOT}/test.log 2>&1
    cmap[dnet-1-bridge]=dnet-1-bridge

    ## Run the test cases
    ./integration-tmp/bin/bats ./test/integration/dnet/bridge.bats

    ## Teardown
    stop_dnet 1 bridge 1>>${INTEGRATION_ROOT}/test.log 2>&1
    unset cmap[dnet-1-bridge]
}

function run_overlay_consul_tests() {
    ## Test overlay network with consul
    ## Setup
    start_dnet 1 consul 1>>${INTEGRATION_ROOT}/test.log 2>&1
    cmap[dnet-1-consul]=dnet-1-consul
    start_dnet 2 consul 1>>${INTEGRATION_ROOT}/test.log 2>&1
    cmap[dnet-2-consul]=dnet-2-consul
    start_dnet 3 consul 1>>${INTEGRATION_ROOT}/test.log 2>&1
    cmap[dnet-3-consul]=dnet-3-consul

    ## Run the test cases
    ./integration-tmp/bin/bats ./test/integration/dnet/overlay-consul.bats

    ## Teardown
    stop_dnet 1 consul 1>>${INTEGRATION_ROOT}/test.log 2>&1
    unset cmap[dnet-1-consul]
    stop_dnet 2 consul 1>>${INTEGRATION_ROOT}/test.log 2>&1
    unset cmap[dnet-2-consul]
    stop_dnet 3 consul 1>>${INTEGRATION_ROOT}/test.log 2>&1
    unset cmap[dnet-3-consul]
}

function run_overlay_zk_tests() {
    ## Test overlay network with zookeeper
    start_zookeeper 1>>${INTEGRATION_ROOT}/test.log 2>&1
    cmap[zookeeper_server]=zookeeper_server

    start_dnet 1 zookeeper 1>>${INTEGRATION_ROOT}/test.log 2>&1
    cmap[dnet-1-zookeeper]=dnet-1-zookeeper
    start_dnet 2 zookeeper 1>>${INTEGRATION_ROOT}/test.log 2>&1
    cmap[dnet-2-zookeeper]=dnet-2-zookeeper
    start_dnet 3 zookeeper 1>>${INTEGRATION_ROOT}/test.log 2>&1
    cmap[dnet-3-zookeeper]=dnet-3-zookeeper

    ./integration-tmp/bin/bats ./test/integration/dnet/overlay-zookeeper.bats

    stop_dnet 1 zookeeper 1>>${INTEGRATION_ROOT}/test.log 2>&1
    unset cmap[dnet-1-zookeeper]
    stop_dnet 2 zookeeper 1>>${INTEGRATION_ROOT}/test.log 2>&1
    unset cmap[dnet-2-zookeeper]
    stop_dnet 3 zookeeper 1>>${INTEGRATION_ROOT}/test.log 2>&1
    unset cmap[dnet-3-zookeeper]

    stop_zookeeper 1>>${INTEGRATION_ROOT}/test.log 2>&1
    unset cmap[zookeeper_server]
}

function run_dnet_tests() {
    # Test dnet configuration options
    ./integration-tmp/bin/bats ./test/integration/dnet/dnet.bats
}

function run_simple_tests() {
    # Test a single node configuration with a global scope test driver
    ## Setup
    start_dnet 1 simple 1>>${INTEGRATION_ROOT}/test.log 2>&1
    cmap[dnet-1-simple]=dnet-1-simple

    ## Run the test cases
    ./integration-tmp/bin/bats ./test/integration/dnet/simple.bats

    ## Teardown
    stop_dnet 1 simple 1>>${INTEGRATION_ROOT}/test.log 2>&1
    unset cmap[dnet-1-simple]
}

function run_multi_tests() {
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
}

source ./test/integration/dnet/helpers.bash

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

# Suite setup
start_consul 1>>${INTEGRATION_ROOT}/test.log 2>&1
cmap[pr_consul]=pr_consul

if [ -z "$SUITES" ]; then
    if [ -n "$CIRCLECI" ]
    then
	# We can only run a limited list of suites in circleci because of the
	# old kernel and limited docker environment.
	suites="dnet simple multi"
    else
	suites="dnet simple multi bridge overlay_consul overlay_zk"
    fi
else
    suites="$SUITES"
fi

for suite in ${suites};
do
    suite_func=run_${suite}_tests
    declare -F $suite_func >/dev/null && $suite_func
done

# Suite teardowm
stop_consul 1>>${INTEGRATION_ROOT}/test.log 2>&1
unset cmap[pr_consul]
