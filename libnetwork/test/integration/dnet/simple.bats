#!/usr/bin/env bats

load helpers

export BATS_TEST_CNT=0

function setup() {
    if [ "${BATS_TEST_CNT}" -eq 0 ]; then
	start_consul
	start_dnet 1 multihost overlay
	export BATS_TEST_CNT=$((${BATS_TEST_CNT}+1))
    fi
}

function teardown() {
    export BATS_TEST_CNT=$((${BATS_TEST_CNT}-1))
    if [ "${BATS_TEST_CNT}" -eq 0 ]; then
	stop_dnet 1
	stop_consul
    fi
}


@test "Test default network" {
    echo $(docker ps)
    run dnet_cmd 1 network ls
    echo ${output}
    echo ${lines[1]}
    name=$(echo ${lines[1]} | cut -d" " -f2)
    driver=$(echo ${lines[1]} | cut -d" " -f3)
    echo ${name} ${driver}
    [ "$name" = "multihost" ]
    [ "$driver" = "overlay" ]
}
