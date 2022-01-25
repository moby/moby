//go:build !windows && !darwin
// +build !windows,!darwin

package main

import (
	"debug/elf"
	"fmt"
	"os"
)

var _ = thisPackageMustNotBeBuiltWithCgo

func init() {
	isStatic, err := isExecutableStaticallyLinked()
	if err != nil {
		fatal(fmt.Errorf("could not determine if docker-chrootarchive is statically or dynamically linked: %w", err))
	}
	if !isStatic {
		fatal(fmt.Errorf("docker-chrootarchive is dynamically linked"))
	}
}

func isExecutableStaticallyLinked() (bool, error) {
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	f, err := elf.Open(exe)
	if err != nil {
		return false, err
	}
	defer f.Close()
	return f.SectionByType(elf.SHT_DYNAMIC) == nil, nil
}
