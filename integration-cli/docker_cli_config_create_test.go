// +build !windows

package main

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/docker/docker/integration-cli/checker"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestConfigCreateWithFile(c *check.C) {
	d := s.AddDaemon(c, true, true)

	testFile, err := ioutil.TempFile("", "configCreateTest")
	c.Assert(err, checker.IsNil) // ensure temp file is created
	defer os.Remove(testFile.Name())

	testData := "TESTINGDATA"
	_, err = testFile.Write([]byte(testData))
	c.Assert(err, checker.IsNil) // ensure temp file is written

	testName := "test_config"
	out, err := d.Cmd("config", "create", testName, testFile.Name())
	c.Assert(err, checker.IsNil, check.Commentf("%s", out))
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	id := strings.TrimSpace(out)
	config := d.GetConfig(c, id)
	c.Assert(config.Spec.Name, checker.Equals, testName)
}
