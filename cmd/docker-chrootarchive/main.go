//go:build !windows
// +build !windows

// docker-chrootarchive is an implementation detail of package
// github.com/docker/docker/pkg/chrootarchive. It must always be compiled to a
// static binary without libc as a hardening measure to ensure that dynamic
// shared objects are never loaded from the chroot environment, even when
// dockerd is dynamically linked. See also CVE-2019-14271.
package main

import (
	"fmt"
	"os"
)

var entrypoints = map[string]func(){
	"docker-tar":        tar,
	"docker-untar":      untar,
	"docker-applyLayer": applyLayer,
}

func main() {
	// Intentionally make it inconvenient to run interactively to discourage
	// people from using it elsewhere.
	cmd := entrypoints[os.Args[0]]
	if cmd == nil {
		fmt.Fprintln(os.Stderr, "This command is reserved for internal use by dockerd")
		os.Exit(1)
	}
	cmd()
}
