package main

import (
	"fmt"
	"os/exec"
	"testing"
)

type TestCondition func() bool

type TestRequirement struct {
	Condition   TestCondition
	SkipMessage string
}

// List test requirements
var (
	SameHostDaemon = TestRequirement{
		func() bool { return isLocalDaemon },
		"Test requires docker daemon to runs on the same machine as CLI",
	}
	UnixCli = TestRequirement{
		func() bool { return isUnixCli },
		"Test requires posix utilities or functionality to run.",
	}
	ExecSupport = TestRequirement{
		func() bool { return supportsExec },
		"Test requires 'docker exec' capabilities on the tested daemon.",
	}
	RegistryHosting = TestRequirement{
		func() bool {
			// for now registry binary is built only if we're running inside
			// container through `make test`. Figure that out by testing if
			// registry binary is in PATH.
			_, err := exec.LookPath(v2binary)
			return err == nil
		},
		fmt.Sprintf("Test requires an environment that can host %s in the same host", v2binary),
	}
)

// testRequires checks if the environment satisfies the requirements
// for the test to run or skips the tests.
func testRequires(t *testing.T, requirements ...TestRequirement) {
	for _, r := range requirements {
		if !r.Condition() {
			t.Skip(r.SkipMessage)
		}
	}
}
