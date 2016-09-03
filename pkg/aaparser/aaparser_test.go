package aaparser

import (
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

type versionExpected struct {
	output  string
	version int
}

func (s *DockerSuite) TestParseVersion(c *check.C) {
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
			c.Fatalf("expected error to be nil for %#v, got: %v", v, err)
		}
		if version != v.version {
			c.Fatalf("expected version to be %d, was %d, for: %#v\n", v.version, version, v)
		}
	}
}
