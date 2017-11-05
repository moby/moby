// Package autoversion contains versions information that is injected at
// build time
package autoversion

// Default build-time variable. These values will be overridden with build-time
// values.
var (
	GitCommit          = "library-import"
	Version            = "library-import"
	BuildTime          = "library-import"
	IAmStatic          = "library-import"
	ContainerdCommitID = "library-import"
	RuncCommitID       = "library-import"
	InitCommitID       = "library-import"
	EngineName         = "moby-engine"
)
