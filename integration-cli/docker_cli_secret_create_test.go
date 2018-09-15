// +build !windows

package main

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/docker/docker/integration-cli/checker"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestSecretCreateWithFile(c *check.C) {
	d := s.AddDaemon(c, true, true)

	testFile, err := ioutil.TempFile("", "secretCreateTest")
	c.Assert(err, checker.IsNil, check.Commentf("failed to create temporary file"))
	defer os.Remove(testFile.Name())

	testData := "TESTINGDATA"
	_, err = testFile.Write([]byte(testData))
	c.Assert(err, checker.IsNil, check.Commentf("failed to write to temporary file"))

	testName := "test_secret"
	out, err := d.Cmd("secret", "create", testName, testFile.Name())
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "", check.Commentf("%s", out))

	id := strings.TrimSpace(out)
	secret := d.GetSecret(c, id)
	c.Assert(secret.Spec.Name, checker.Equals, testName)
}
