# -*- mode: sh -*-
#!/usr/bin/env bats

load helpers

@test "Test overlay network with zookeeper" {
    test_overlay zookeeper
}
