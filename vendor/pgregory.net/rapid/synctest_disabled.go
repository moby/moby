//go:build !go1.25

package rapid

// SyncTest is only available on Go 1.25+.
func SyncTest(t *T, _ func(*T)) {
	if t == nil {
		panic("rapid.SyncTest requires *rapid.T")
	}
	t.Helper()
	t.Fatalf("[rapid] SyncTest requires Go 1.25 or newer")
}
