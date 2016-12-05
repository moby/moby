// +build !windows

package main

import (
	"strings"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestSecretRm(c *check.C) {
	d := s.AddDaemon(c, true, true)

	testName := "test_secret"
	id := d.createSecret(c, swarm.SecretSpec{
		swarm.Annotations{
			Name: testName,
		},
		[]byte("TESTINGDATA"),
	})
	c.Assert(id, checker.Not(checker.Equals), "", check.Commentf("secrets: %s", id))

	secret := d.getSecret(c, id)
	c.Assert(secret.Spec.Name, checker.Equals, testName)

	out, _ := d.Cmd("secret", "rm", "test_secret", "non-exist", "non-exist2")
	c.Assert(strings.TrimSpace(out), checker.Contains, id)
	c.Assert(strings.TrimSpace(out), checker.Contains, "could not find secret non-exist")
	c.Assert(strings.TrimSpace(out), checker.Contains, "could not find secret non-exist2")
}
