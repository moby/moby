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

function run_overlay_etcd_tests() {
	## Test overlay network with etcd
	start_dnet 1 etcd 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	cmap[dnet - 1 - etcd]=dnet-1-etcd
	start_dnet 2 etcd 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	cmap[dnet - 2 - etcd]=dnet-2-etcd
	start_dnet 3 etcd 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	cmap[dnet - 3 - etcd]=dnet-3-etcd

	./integration-tmp/bin/bats ./test/integration/dnet/overlay-etcd.bats

	stop_dnet 1 etcd 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	unset cmap[dnet-1-etcd]
	stop_dnet 2 etcd 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	unset cmap[dnet-2-etcd]
	stop_dnet 3 etcd 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	unset cmap[dnet-3-etcd]
}

function run_dnet_tests() {
	# Test dnet configuration options
	./integration-tmp/bin/bats ./test/integration/dnet/dnet.bats
}

function run_multi_etcd_tests() {
	# Test multi node configuration with a global scope test driver backed by etcd

	## Setup
	start_dnet 1 multi_etcd etcd 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	cmap[dnet - 1 - multi_etcd]=dnet-1-multi_etcd
	start_dnet 2 multi_etcd etcd 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	cmap[dnet - 2 - multi_etcd]=dnet-2-multi_etcd
	start_dnet 3 multi_etcd etcd 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	cmap[dnet - 3 - multi_etcd]=dnet-3-multi_etcd

	## Run the test cases
	./integration-tmp/bin/bats ./test/integration/dnet/multi.bats

	## Teardown
	stop_dnet 1 multi_etcd 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	unset cmap[dnet-1-multi_etcd]
	stop_dnet 2 multi_etcd 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	unset cmap[dnet-2-multi_etcd]
	stop_dnet 3 multi_etcd 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	unset cmap[dnet-3-multi_etcd]
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
	suites="dnet multi_etcd  bridge overlay_etcd"
else
	suites="$SUITES"
fi

if [[ "$suites" =~ .*etcd.* ]]; then
	echo "Starting etcd ..."
	start_etcd 1>> ${INTEGRATION_ROOT}/test.log 2>&1
	cmap[dn_etcd]=dn_etcd
fi

echo ""

for suite in ${suites}; do
	suite_func=run_${suite}_tests
	echo "Running ${suite}_tests ..."
	declare -F $suite_func > /dev/null && $suite_func
	echo ""
done
