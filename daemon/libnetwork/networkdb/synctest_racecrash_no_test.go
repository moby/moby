//go:build go1.27 || !race

package networkdb

// synctestCrashesUnderRace reports whether running a [testing/synctest] bubble
// kills this test binary; see synctest_racecrash_test.go.
const synctestCrashesUnderRace = false
