// +build !windows

package main

import (
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestSecretCreate(c *check.C) {
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
}

func (s *DockerSwarmSuite) TestSecretCreateWithLabels(c *check.C) {
	d := s.AddDaemon(c, true, true)

	testName := "test_secret"
	id := d.createSecret(c, swarm.SecretSpec{
		swarm.Annotations{
			Name: testName,
			Labels: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		[]byte("TESTINGDATA"),
	})
	c.Assert(id, checker.Not(checker.Equals), "", check.Commentf("secrets: %s", id))

	secret := d.getSecret(c, id)
	c.Assert(secret.Spec.Name, checker.Equals, testName)
	c.Assert(len(secret.Spec.Labels), checker.Equals, 2)
	c.Assert(secret.Spec.Labels["key1"], checker.Equals, "value1")
	c.Assert(secret.Spec.Labels["key2"], checker.Equals, "value2")
}
