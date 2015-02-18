package main

import (
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
