# -*- mode: sh -*-
#!/usr/bin/env bats

load helpers

@test "Test overlay network with consul" {
    skip_for_circleci
    run test_overlay consul
    [ "$status" -eq 0 ]
    run dnet_cmd $(inst_id2port 2) network rm multihost
}
