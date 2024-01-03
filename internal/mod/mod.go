package mod

import (
	"runtime/debug"
	"sync"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

var (
	buildInfoOnce sync.Once
	buildInfo     *debug.BuildInfo
)

func Version(name string) (modVersion string) {
	return moduleVersion(name, readBuildInfo())
}

func moduleVersion(name string, bi *debug.BuildInfo) (modVersion string) {
	if bi == nil {
		return
	}
	// iterate over all dependencies and find buildkit
	for _, dep := range bi.Deps {
		if dep.Path != name {
			continue
		}
		// get the version of buildkit dependency
		modVersion = dep.Version
		if dep.Replace != nil {
			// if the version is replaced, get the replaced version
			modVersion = dep.Replace.Version
		}
		if !module.IsPseudoVersion(modVersion) {
			return
		}
		// if the version is a pseudo version, get the base version
		// e.g. v0.10.7-0.20230306143919-70f2ad56d3e5 => v0.10.6
		if base, err := module.PseudoVersionBase(modVersion); err == nil && base != "" {
			// set canonical version of the base version (removes +incompatible suffix)
			// e.g. v2.1.2+incompatible => v2.1.2
			base = semver.Canonical(base)
			// if the version is a pseudo version, get the revision
			// e.g. v0.10.7-0.20230306143919-70f2ad56d3e5 => 70f2ad56d3e5
			if rev, err := module.PseudoVersionRev(modVersion); err == nil && rev != "" {
				// append the revision to the version
				// e.g. v0.10.7-0.20230306143919-70f2ad56d3e5 => v0.10.6+70f2ad56d3e5
				modVersion = base + "+" + rev
			} else {
				// if the revision is not available, use the base version
				modVersion = base
			}
		}
		break
	}
	return
}

func readBuildInfo() *debug.BuildInfo {
	buildInfoOnce.Do(func() {
		buildInfo, _ = debug.ReadBuildInfo()
	})
	return buildInfo
}
