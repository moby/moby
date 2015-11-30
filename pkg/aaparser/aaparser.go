package aaparser

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
)

// GetVersion returns the major and minor version of apparmor_parser
func GetVersion() (int, int, error) {
	// get the apparmor_version version
	cmd := exec.Command("apparmor_parser", "--version")

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("getting apparmor_parser version failed: %s (%s)", err, output)
	}

	// parse the version from the output
	// output is in the form of the following:
	// AppArmor parser version 2.9.1
	// Copyright (C) 1999-2008 Novell Inc.
	// Copyright 2009-2012 Canonical Ltd.
	lines := strings.SplitN(string(output), "\n", 2)
	words := strings.Split(lines[0], " ")
	version := words[len(words)-1]
	// split by major minor version
	v := strings.Split(version, ".")
	if len(v) < 2 {
		return -1, -1, fmt.Errorf("parsing major minor version failed for %q", version)
	}

	majorVersion, err := strconv.Atoi(v[0])
	if err != nil {
		return -1, -1, err
	}
	minorVersion, err := strconv.Atoi(v[1])
	if err != nil {
		return -1, -1, err
	}

	return majorVersion, minorVersion, nil
}
