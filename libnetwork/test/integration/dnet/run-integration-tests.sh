#!/usr/bin/env bash

set -e

export INTEGRATION_ROOT=./integration-tmp
export TMPC_ROOT=./integration-tmp/tmpc

declare -A cmap

trap "cleanup_containers" EXIT SIGINT

function cleanup_containers() {
	for c in "${!cmap[@]}"; do
		docker rm -f $c 1>> ${INTEGRATION_ROOT}/test.log 2>&1 || true
	done

	unset cmap
}

function run_bridge_tests() {
	## Setup
	start_dnet 1 bridge 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	cmap[dnet - 1 - bridge]=dnet-1-bridge

	## Run the test cases
	./integration-tmp/bin/bats ./test/integration/dnet/bridge.bats

	## Teardown
	stop_dnet 1 bridge 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	unset cmap[dnet-1-bridge]
}

function run_overlay_local_tests() {
	## Test overlay network in local scope
	## Setup
	start_dnet 1 local 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	cmap[dnet - 1 - local]=dnet-1-local
	start_dnet 2 local:$(dnet_container_ip 1 local) 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	cmap[dnet - 2 - local]=dnet-2-local
	start_dnet 3 local:$(dnet_container_ip 1 local) 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	cmap[dnet - 3 - local]=dnet-3-local

	## Run the test cases
	./integration-tmp/bin/bats ./test/integration/dnet/overlay-local.bats

	## Teardown
	stop_dnet 1 local 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	unset cmap[dnet-1-local]
	stop_dnet 2 local 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	unset cmap[dnet-2-local]
	stop_dnet 3 local 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	unset cmap[dnet-3-local]
}

function run_dnet_tests() {
	# Test dnet configuration options
	./integration-tmp/bin/bats ./test/integration/dnet/dnet.bats
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

if [ -z "$SUITES" ]; then
	suites="dnet bridge"
else
	suites="$SUITES"
fi

echo ""

for suite in ${suites}; do
	suite_func=run_${suite}_tests
	echo "Running ${suite}_tests ..."
	declare -F $suite_func > /dev/null && $suite_func
	echo ""
done
