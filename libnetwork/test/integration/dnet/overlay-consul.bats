# -*- mode: sh -*-
#!/usr/bin/env bats

load helpers

@test "Test overlay network with consul" {
    skip_for_circleci
    test_overlay consul
}

@test "Test overlay network singlehost with consul" {
    skip_for_circleci
    test_overlay_singlehost consul
}

@test "test overlay network etc hosts with consul" {
    skip_for_circleci
    test_overlay_etchosts consul
}

@test "Test overlay network with dnet restart" {
    skip_for_circleci
    test_overlay consul skip_rm
    docker restart dnet-1-consul
    wait_for_dnet $(inst_id2port 1) dnet-1-consul
    docker restart dnet-2-consul
    wait_for_dnet $(inst_id2port 2) dnet-2-consul
    docker restart dnet-3-consul
    wait_for_dnet $(inst_id2port 3) dnet-3-consul
    test_overlay consul skip_add
}

@test "Test overlay network internal network with consul" {
    skip_for_circleci
    test_overlay consul internal
}