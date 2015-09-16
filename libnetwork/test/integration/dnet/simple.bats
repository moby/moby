#!/usr/bin/env bats

load helpers

function setup() {
    if [ "${BATS_TEST_NUMBER}" -eq 1 ]; then
	start_consul
	start_dnet 1 simple multihost overlay
    fi
}

function teardown() {
    if [ "${BATS_TEST_NUMBER}" -eq 6 ]; then
	stop_dnet 1 simple
	stop_consul
    fi
}

@test "Test default network" {
    echo $(docker ps)
    run dnet_cmd $(inst_id2port 1) network ls
    [ "$status" -eq 0 ]
    echo ${output}
    echo ${lines[1]}
    name=$(echo ${lines[1]} | cut -d" " -f2)
    driver=$(echo ${lines[1]} | cut -d" " -f3)
    echo ${name} ${driver}
    [ "$name" = "multihost" ]
    [ "$driver" = "overlay" ]
}

@test "Test network create" {
    echo $(docker ps)
    run dnet_cmd $(inst_id2port 1) network create -d overlay mh1
    [ "$status" -eq 0 ]
    line=$(dnet_cmd $(inst_id2port 1) network ls | grep mh1)
    echo ${line}
    name=$(echo ${line} | cut -d" " -f2)
    driver=$(echo ${line} | cut -d" " -f3)
    echo ${name} ${driver}
    [ "$name" = "mh1" ]
    [ "$driver" = "overlay" ]
    dnet_cmd $(inst_id2port 1) network rm mh1
}

@test "Test network delete with id" {
    echo $(docker ps)
    run dnet_cmd $(inst_id2port 1) network create -d overlay mh1
    [ "$status" -eq 0 ]
    echo ${output}
    dnet_cmd $(inst_id2port 1) network rm ${output}
}

@test "Test service create" {
    echo $(docker ps)
    run dnet_cmd $(inst_id2port 1) service publish svc1.multihost
    [ "$status" -eq 0 ]
    echo ${output}
    run dnet_cmd $(inst_id2port 1) service ls
    [ "$status" -eq 0 ]
    echo ${output}
    echo ${lines[1]}
    svc=$(echo ${lines[1]} | cut -d" " -f2)
    network=$(echo ${lines[1]} | cut -d" " -f3)
    echo ${svc} ${network}
    [ "$network" = "multihost" ]
    [ "$svc" = "svc1" ]
    dnet_cmd $(inst_id2port 1) service unpublish svc1.multihost
}

@test "Test service delete with id" {
    echo $(docker ps)
    run dnet_cmd $(inst_id2port 1) service publish svc1.multihost
    [ "$status" -eq 0 ]
    echo ${output}
    run dnet_cmd $(inst_id2port 1) service ls
    [ "$status" -eq 0 ]
    echo ${output}
    echo ${lines[1]}
    id=$(echo ${lines[1]} | cut -d" " -f1)
    dnet_cmd $(inst_id2port 1) service unpublish ${id}
}

@test "Test service attach" {
    skip_for_circleci
    echo $(docker ps)
    dnet_cmd $(inst_id2port 1) service publish svc1.multihost
    dnet_cmd $(inst_id2port 1) container create container_1
    dnet_cmd $(inst_id2port 1) service attach container_1 svc1.multihost
    run dnet_cmd $(inst_id2port 1) service ls
    [ "$status" -eq 0 ]
    echo ${output}
    echo ${lines[1]}
    container=$(echo ${lines[1]} | cut -d" " -f4)
    [ "$container" = "container_1" ]
    dnet_cmd $(inst_id2port 1) service detach container_1 svc1.multihost
    dnet_cmd $(inst_id2port 1) container rm container_1
    dnet_cmd $(inst_id2port 1) service unpublish svc1.multihost
}
