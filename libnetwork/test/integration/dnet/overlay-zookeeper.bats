# -*- mode: sh -*-
#!/usr/bin/env bats

load helpers

@test "Test overlay network with zookeeper" {
    skip_for_circleci
    run test_overlay zookeeper
    [ "$status" -eq 0 ]
    run dnet_cmd $(inst_id2port 2) network rm multihost
}

