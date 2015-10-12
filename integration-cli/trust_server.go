package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/pkg/tlsconfig"
	"github.com/go-check/check"
)

var notaryBinary = "notary-server"

type testNotary struct {
	cmd *exec.Cmd
	dir string
}

const notaryHost = "localhost:4443"
const notaryURL = "https://" + notaryHost

func newTestNotary(c *check.C) (*testNotary, error) {
	template := `{
	"server": {
		"addr": "%s",
		"tls_key_file": "fixtures/notary/localhost.key",
		"tls_cert_file": "fixtures/notary/localhost.cert"
	},
	"trust_service": {
		"type": "local",
		"hostname": "",
		"port": "",
		"key_algorithm": "ed25519"
	},
	"logging": {
		"level": 5
	}
}`
	tmp, err := ioutil.TempDir("", "notary-test-")
	if err != nil {
		return nil, err
	}
	confPath := filepath.Join(tmp, "config.json")
	config, err := os.Create(confPath)
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(config, template, notaryHost); err != nil {
		os.RemoveAll(tmp)
		return nil, err
	}

	cmd := exec.Command(notaryBinary, "-config", confPath)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(tmp)
		if os.IsNotExist(err) {
			c.Skip(err.Error())
		}
		return nil, err
	}

	testNotary := &testNotary{
		cmd: cmd,
		dir: tmp,
	}

	// Wait for notary to be ready to serve requests.
	for i := 1; i <= 5; i++ {
		if err = testNotary.Ping(); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond * time.Duration(i*i))
	}

	if err != nil {
		c.Fatalf("Timeout waiting for test notary to become available: %s", err)
	}

	return testNotary, nil
}

func (t *testNotary) Ping() error {
	tlsConfig := tlsconfig.ClientDefault
	tlsConfig.InsecureSkipVerify = true
	client := http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig:     &tlsConfig,
		},
	}
	resp, err := client.Get(fmt.Sprintf("%s/v2/", notaryURL))
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("notary ping replied with an unexpected status code %d", resp.StatusCode)
	}
	return nil
}

func (t *testNotary) Close() {
	t.cmd.Process.Kill()
	os.RemoveAll(t.dir)
}

func (s *DockerTrustSuite) trustedCmd(cmd *exec.Cmd) {
	pwd := "12345678"
	trustCmdEnv(cmd, notaryURL, pwd, pwd)
}

func (s *DockerTrustSuite) trustedCmdWithServer(cmd *exec.Cmd, server string) {
	pwd := "12345678"
	trustCmdEnv(cmd, server, pwd, pwd)
}

func (s *DockerTrustSuite) trustedCmdWithPassphrases(cmd *exec.Cmd, rootPwd, repositoryPwd string) {
	trustCmdEnv(cmd, notaryURL, rootPwd, repositoryPwd)
}

func (s *DockerTrustSuite) trustedCmdWithDeprecatedEnvPassphrases(cmd *exec.Cmd, offlinePwd, taggingPwd string) {
	trustCmdDeprecatedEnv(cmd, notaryURL, offlinePwd, taggingPwd)
}

func trustCmdEnv(cmd *exec.Cmd, server, rootPwd, repositoryPwd string) {
	env := []string{
		"DOCKER_CONTENT_TRUST=1",
		fmt.Sprintf("DOCKER_CONTENT_TRUST_SERVER=%s", server),
		fmt.Sprintf("DOCKER_CONTENT_TRUST_ROOT_PASSPHRASE=%s", rootPwd),
		fmt.Sprintf("DOCKER_CONTENT_TRUST_REPOSITORY_PASSPHRASE=%s", repositoryPwd),
	}
	cmd.Env = append(os.Environ(), env...)
}

// Helper method to test the old env variables OFFLINE and TAGGING that will
// be deprecated by 1.10
func trustCmdDeprecatedEnv(cmd *exec.Cmd, server, offlinePwd, taggingPwd string) {
	env := []string{
		"DOCKER_CONTENT_TRUST=1",
		fmt.Sprintf("DOCKER_CONTENT_TRUST_SERVER=%s", server),
		fmt.Sprintf("DOCKER_CONTENT_TRUST_OFFLINE_PASSPHRASE=%s", offlinePwd),
		fmt.Sprintf("DOCKER_CONTENT_TRUST_TAGGING_PASSPHRASE=%s", taggingPwd),
	}
	cmd.Env = append(os.Environ(), env...)
}

func (s *DockerTrustSuite) setupTrustedImage(c *check.C, name string) string {
	repoName := fmt.Sprintf("%v/dockercli/%s:latest", privateRegistryURL, name)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("Error running trusted push: %s\n%s", err, out)
	}
	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	if out, status := dockerCmd(c, "rmi", repoName); status != 0 {
		c.Fatalf("Error removing image %q\n%s", repoName, out)
	}

	return repoName
}
