# -*- mode: sh -*-
#!/usr/bin/env bats

load helpers

@test "Test overlay network hostmode with consul" {
    test_overlay_hostmode consul
}
