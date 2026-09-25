// Package reexecoverride provides test utilities for overriding argv0 as
// observed by reexec.Self within the current process.

package reexecoverride

import "sync/atomic"

// argv0Override holds an optional override for os.Args[0] used by reexec.Self.
var argv0Override atomic.Pointer[string]

// Argv0 returns the overridden argv0 if set.
func Argv0() (string, bool) {
	p := argv0Override.Load()
	if p == nil {
		return "", false
	}
	return *p, true
}

// TestingTB is the minimal subset of [testing.TB] used by this package.
type TestingTB interface {
	Helper()
	Cleanup(func())
}

// OverrideArgv0 overrides the argv0 value observed by reexec.Self for the
// lifetime of the calling test and restores it via [testing.TB.Cleanup].
//
// The override is process-global. Tests using OverrideArgv0 must not run in
// parallel with other tests that call reexec.Self. OverrideArgv0 panics if an
// override is already active.
func OverrideArgv0(t TestingTB, argv0 string) {
	t.Helper()

	s := argv0
	if !argv0Override.CompareAndSwap(nil, &s) {
		panic("testing: test using reexecoverride.OverrideArgv0 cannot use t.Parallel")
	}

	t.Cleanup(func() {
		if !argv0Override.CompareAndSwap(&s, nil) {
			panic("testing: cleanup for reexecoverride.OverrideArgv0 detected parallel use of reexec.Self")
		}
	})
}
