// Runtime guard for this package's synctest-based tests on race-enabled builds
// before Go 1.27, where running them kills the test binary.
//
// Before Go 1.27 the runtime installs a single per-bubble race context into
// gp.racectx while running any of that bubble's timers (runtime/time.go). Timers
// within a bubble execute concurrently on several threads, so several threads
// drive the same ThreadSanitizer thread state at once. That corrupts TSan's
// stack-depot hash table, and the next insertion dereferences a bogus pointer
// and kills the process inside __sanitizer::StackDepotBase::Put -- SIGBUS on
// darwin/arm64, SIGSEGV elsewhere. No data race is reported and the traceback
// points at whatever goroutine happened to be running, so the failure gives no
// hint of its own cause.
//
// These tests are unusually good at provoking it. Every node runs memberlist's
// per-probe ack timers and its suspicion timers, and the in-memory transport's
// net.Pipe deadlines are time.AfterFunc timers too, so a 25-node bubble has
// hundreds of timer functions executing concurrently. Observed at roughly one
// run in three.
//
// The guard is a run-time skip rather than a build constraint on the files
// themselves. Only *running* a bubble crashes -- the code compiles fine under
// -race -- and constraining the files meant naming every affected test a second
// time in a stub file under the inverse constraint. Nothing enforced that pairing:
// a test added or renamed without updating the stubs was excluded from race builds
// by its file's constraint and had no stub to report it, so it silently ceased to
// exist there and `go test -race` still passed. A forgotten requireSynctest fails
// loudly instead, which is the direction this should fail in.
//
// See golang.org/issue/76691, fixed by golang.org/cl/753200, which has not been
// backported to Go 1.26. Once the toolchain carries the fix, deleting this file
// and its two synctestCrashesUnderRace definitions is the whole of it.

package networkdb

import "testing"

// requireSynctest skips t if running a [testing/synctest] bubble would crash this
// test binary. Call it before entering a bubble.
func requireSynctest(t *testing.T) {
	t.Helper()
	if synctestCrashesUnderRace {
		t.Skip("-race with Go < 1.27 crashes the test binary on synctest timers: " +
			"golang.org/issue/76691, fixed by golang.org/cl/753200, not backported to 1.26")
	}
}
