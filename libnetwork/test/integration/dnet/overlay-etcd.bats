# -*- mode: sh -*-
#!/usr/bin/env bats

load helpers

@test "Test overlay network with etcd" {
    test_overlay etcd
}
