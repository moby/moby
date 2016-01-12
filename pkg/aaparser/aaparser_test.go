package aaparser

import (
	"testing"
)

type versionExpected struct {
	output string
	major  int
	minor  int
}

func TestParseVersion(t *testing.T) {
	versions := []versionExpected{
		{
			output: `AppArmor parser version 2.10
Copyright (C) 1999-2008 Novell Inc.
Copyright 2009-2012 Canonical Ltd.

`,
			major: 2,
			minor: 10,
		},
		{
			output: `AppArmor parser version 2.8
Copyright (C) 1999-2008 Novell Inc.
Copyright 2009-2012 Canonical Ltd.

`,
			major: 2,
			minor: 8,
		},
		{
			output: `AppArmor parser version 2.20
Copyright (C) 1999-2008 Novell Inc.
Copyright 2009-2012 Canonical Ltd.

`,
			major: 2,
			minor: 20,
		},
		{
			output: `AppArmor parser version 2.05
Copyright (C) 1999-2008 Novell Inc.
Copyright 2009-2012 Canonical Ltd.

`,
			major: 2,
			minor: 5,
		},
	}

	for _, v := range versions {
		major, minor, err := parseVersion(v.output)
		if err != nil {
			t.Fatalf("expected error to be nil for %#v, got: %v", v, err)
		}
		if major != v.major {
			t.Fatalf("expected major version to be %d, was %d, for: %#v\n", v.major, major, v)
		}
		if minor != v.minor {
			t.Fatalf("expected minor version to be %d, was %d, for: %#v\n", v.minor, minor, v)
		}
	}
}
