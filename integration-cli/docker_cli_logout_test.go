package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/docker/docker/integration-cli/checker"
	"github.com/go-check/check"
)

func (s *DockerRegistryAuthHtpasswdSuite) TestLogoutWithExternalAuth(c *check.C) {
	osPath := os.Getenv("PATH")
	defer os.Setenv("PATH", osPath)

	workingDir, err := os.Getwd()
	c.Assert(err, checker.IsNil)
	absolute, err := filepath.Abs(filepath.Join(workingDir, "fixtures", "auth"))
	c.Assert(err, checker.IsNil)
	testPath := fmt.Sprintf("%s%c%s", osPath, filepath.ListSeparator, absolute)

	os.Setenv("PATH", testPath)

	repoName := fmt.Sprintf("%v/dockercli/busybox:authtest", privateRegistryURL)

	tmp, err := ioutil.TempDir("", "integration-cli-")
	c.Assert(err, checker.IsNil)

	externalAuthConfig := `{ "credsStore": "shell-test" }`

	configPath := filepath.Join(tmp, "config.json")
	err = ioutil.WriteFile(configPath, []byte(externalAuthConfig), 0644)
	c.Assert(err, checker.IsNil)

	dockerCmd(c, "--config", tmp, "login", "-u", s.reg.Username(), "-p", s.reg.Password(), privateRegistryURL)

	b, err := ioutil.ReadFile(configPath)
	c.Assert(err, checker.IsNil)
	c.Assert(string(b), checker.Not(checker.Contains), "\"auth\":")
	c.Assert(string(b), checker.Contains, privateRegistryURL)

	dockerCmd(c, "--config", tmp, "tag", "busybox", repoName)
	dockerCmd(c, "--config", tmp, "push", repoName)

	dockerCmd(c, "--config", tmp, "logout", privateRegistryURL)

	b, err = ioutil.ReadFile(configPath)
	c.Assert(err, checker.IsNil)
	c.Assert(string(b), checker.Not(checker.Contains), privateRegistryURL)

	// check I cannot pull anymore
	out, _, err := dockerCmdWithError("--config", tmp, "pull", repoName)
	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "Error: image dockercli/busybox:authtest not found")
}

// #23100
func (s *DockerRegistryAuthHtpasswdSuite) TestLogoutWithWrongHostnamesStored(c *check.C) {
	osPath := os.Getenv("PATH")
	defer os.Setenv("PATH", osPath)

	workingDir, err := os.Getwd()
	c.Assert(err, checker.IsNil)
	absolute, err := filepath.Abs(filepath.Join(workingDir, "fixtures", "auth"))
	c.Assert(err, checker.IsNil)
	testPath := fmt.Sprintf("%s%c%s", osPath, filepath.ListSeparator, absolute)

	os.Setenv("PATH", testPath)

	cmd := exec.Command("docker-credential-shell-test", "store")
	stdin := bytes.NewReader([]byte(fmt.Sprintf(`{"ServerURL": "https://%s", "Username": "%s", "Secret": "%s"}`, privateRegistryURL, s.reg.Username(), s.reg.Password())))
	cmd.Stdin = stdin
	c.Assert(cmd.Run(), checker.IsNil)

	tmp, err := ioutil.TempDir("", "integration-cli-")
	c.Assert(err, checker.IsNil)

	externalAuthConfig := fmt.Sprintf(`{ "auths": {"https://%s": {}}, "credsStore": "shell-test" }`, privateRegistryURL)

	configPath := filepath.Join(tmp, "config.json")
	err = ioutil.WriteFile(configPath, []byte(externalAuthConfig), 0644)
	c.Assert(err, checker.IsNil)

	dockerCmd(c, "--config", tmp, "login", "-u", s.reg.Username(), "-p", s.reg.Password(), privateRegistryURL)

	b, err := ioutil.ReadFile(configPath)
	c.Assert(err, checker.IsNil)
	c.Assert(string(b), checker.Contains, fmt.Sprintf("\"https://%s\": {}", privateRegistryURL))
	c.Assert(string(b), checker.Contains, fmt.Sprintf("\"%s\": {}", privateRegistryURL))

	dockerCmd(c, "--config", tmp, "logout", privateRegistryURL)

	b, err = ioutil.ReadFile(configPath)
	c.Assert(err, checker.IsNil)
	c.Assert(string(b), checker.Not(checker.Contains), fmt.Sprintf("\"https://%s\": {}", privateRegistryURL))
	c.Assert(string(b), checker.Not(checker.Contains), fmt.Sprintf("\"%s\": {}", privateRegistryURL))
}
