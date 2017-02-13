// +build !windows

package main

import (
	"net/http"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestAPISwarmSecretsEmptyList(c *check.C) {
	d := s.AddDaemon(c, true, true)

	secrets := d.ListSecrets(c)
	c.Assert(secrets, checker.NotNil)
	c.Assert(len(secrets), checker.Equals, 0, check.Commentf("secrets: %#v", secrets))
}

func (s *DockerSwarmSuite) TestAPISwarmSecretsCreate(c *check.C) {
	d := s.AddDaemon(c, true, true)

	testName := "test_secret"
	id := d.CreateSecret(c, swarm.SecretSpec{
		swarm.Annotations{
			Name: testName,
		},
		[]byte("TESTINGDATA"),
	})
	c.Assert(id, checker.Not(checker.Equals), "", check.Commentf("secrets: %s", id))

	secrets := d.ListSecrets(c)
	c.Assert(len(secrets), checker.Equals, 1, check.Commentf("secrets: %#v", secrets))
	name := secrets[0].Spec.Annotations.Name
	c.Assert(name, checker.Equals, testName, check.Commentf("secret: %s", name))
}

func (s *DockerSwarmSuite) TestAPISwarmSecretsDelete(c *check.C) {
	d := s.AddDaemon(c, true, true)

	testName := "test_secret"
	id := d.CreateSecret(c, swarm.SecretSpec{
		swarm.Annotations{
			Name: testName,
		},
		[]byte("TESTINGDATA"),
	})
	c.Assert(id, checker.Not(checker.Equals), "", check.Commentf("secrets: %s", id))

	secret := d.GetSecret(c, id)
	c.Assert(secret.ID, checker.Equals, id, check.Commentf("secret: %v", secret))

	d.DeleteSecret(c, secret.ID)
	status, out, err := d.SockRequest("GET", "/secrets/"+id, nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNotFound, check.Commentf("secret delete: %s", string(out)))
}
