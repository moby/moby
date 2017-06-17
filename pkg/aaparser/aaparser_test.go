package aaparser

import (
	"flag"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type versionExpected struct {
	output  string
	version int
}

const (
	defaultApparmorProfile = "docker-default"
)

var rootEnabled = false

func init() {
	flag.BoolVar(&rootEnabled, "test.root", false, "enable tests that require root")
}

func TestParseVersion(t *testing.T) {
	versions := []versionExpected{
		{
			output: `AppArmor parser version 2.10
Copyright (C) 1999-2008 Novell Inc.
Copyright 2009-2012 Canonical Ltd.

`,
			version: 210000,
		},
		{
			output: `AppArmor parser version 2.8
Copyright (C) 1999-2008 Novell Inc.
Copyright 2009-2012 Canonical Ltd.

`,
			version: 208000,
		},
		{
			output: `AppArmor parser version 2.20
Copyright (C) 1999-2008 Novell Inc.
Copyright 2009-2012 Canonical Ltd.

`,
			version: 220000,
		},
		{
			output: `AppArmor parser version 2.05
Copyright (C) 1999-2008 Novell Inc.
Copyright 2009-2012 Canonical Ltd.

`,
			version: 205000,
		},
		{
			output: `AppArmor parser version 2.9.95
Copyright (C) 1999-2008 Novell Inc.
Copyright 2009-2012 Canonical Ltd.

`,
			version: 209095,
		},
		{
			output: `AppArmor parser version 3.14.159
Copyright (C) 1999-2008 Novell Inc.
Copyright 2009-2012 Canonical Ltd.

`,
			version: 314159,
		},
	}

	for _, v := range versions {
		version, err := parseVersion(v.output)
		if err != nil {
			t.Fatalf("expected error to be nil for %#v, got: %v", v, err)
		}
		if version != v.version {
			t.Fatalf("expected version to be %d, was %d, for: %#v\n", v.version, version, v)
		}
	}
}

func requiresRoot(t *testing.T) {
	if !rootEnabled {
		t.Skip("skipping test that requires root")
		return
	}
	require.Equal(t, 0, os.Getuid(), "This test must be run as root.")
}

func TestGetVersion(t *testing.T) {
	output, err := cmd("", "--version")
	require.NotNil(t, output)
	version, err := GetVersion()
	require.NoError(t, err)
	require.NotEqual(t, -1, version)
}

func TestLoadProfile(t *testing.T) {
	requiresRoot(t)

	file, err := ioutil.TempFile("", defaultApparmorProfile)
	require.NoError(t, err)

	profilePath := file.Name()
	defer file.Close()
	defer os.Remove(profilePath)

	err = LoadProfile(profilePath)
	require.NoError(t, err)
}
