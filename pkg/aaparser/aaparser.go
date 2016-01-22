// Package aaparser is a convenience package interacting with `apparmor_parser`.
package aaparser

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	binary = "apparmor_parser"
)

// GetVersion returns the major and minor version of apparmor_parser.
func GetVersion() (int, int, error) {
	output, err := cmd("", "--version")
	if err != nil {
		return -1, -1, err
	}

	return parseVersion(string(output))
}

// LoadProfile runs `apparmor_parser -r -W` on a specified apparmor profile to
// replace and write it to disk.
func LoadProfile(profilePath string) error {
	_, err := cmd(filepath.Dir(profilePath), "-r", "-W", filepath.Base(profilePath))
	if err != nil {
		return err
	}
	return nil
}

// cmd runs `apparmor_parser` with the passed arguments.
func cmd(dir string, arg ...string) (string, error) {
	c := exec.Command(binary, arg...)
	c.Dir = dir

	output, err := c.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("running `%s %s` failed with output: %s\nerror: %v", c.Path, strings.Join(c.Args, " "), string(output), err)
	}

	return string(output), nil
}

// parseVersion takes the output from `apparmor_parser --version` and returns
// the major and minor version for `apparor_parser`.
func parseVersion(output string) (int, int, error) {
	// output is in the form of the following:
	// AppArmor parser version 2.9.1
	// Copyright (C) 1999-2008 Novell Inc.
	// Copyright 2009-2012 Canonical Ltd.
	lines := strings.SplitN(output, "\n", 2)
	words := strings.Split(lines[0], " ")
	version := words[len(words)-1]

	// split by major minor version
	v := strings.Split(version, ".")
	if len(v) < 2 {
		return -1, -1, fmt.Errorf("parsing major minor version failed for output: `%s`", output)
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
