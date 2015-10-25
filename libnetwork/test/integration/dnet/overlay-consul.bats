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
