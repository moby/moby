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

	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/pkg/tlsconfig"
	"github.com/docker/notary/client"
	"github.com/docker/notary/passphrase"
	"github.com/docker/notary/tuf/data"
	"github.com/go-check/check"
)

var notaryBinary = "notary-server"
var notaryClientBinary = "notary"

type testNotary struct {
	cmd *exec.Cmd
	dir string
}

const notaryHost = "localhost:4443"
const notaryURL = "https://" + notaryHost

func newTestNotary(c *check.C) (*testNotary, error) {
	// generate server config
	template := `{
	"server": {
		"http_addr": "%s",
		"tls_key_file": "%s",
		"tls_cert_file": "%s"
	},
	"trust_service": {
		"type": "local",
		"hostname": "",
		"port": "",
		"key_algorithm": "ed25519"
	},
	"logging": {
		"level": "debug"
	},
	"storage": {
        "backend": "memory"
    }
}`
	tmp, err := ioutil.TempDir("", "notary-test-")
	if err != nil {
		return nil, err
	}
	confPath := filepath.Join(tmp, "config.json")
	config, err := os.Create(confPath)
	defer config.Close()
	if err != nil {
		return nil, err
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(config, template, notaryHost, filepath.Join(workingDir, "fixtures/notary/localhost.key"), filepath.Join(workingDir, "fixtures/notary/localhost.cert")); err != nil {
		os.RemoveAll(tmp)
		return nil, err
	}

	// generate client config
	clientConfPath := filepath.Join(tmp, "client-config.json")
	clientConfig, err := os.Create(clientConfPath)
	defer clientConfig.Close()
	if err != nil {
		return nil, err
	}
	template = `{
	"trust_dir" : "%s",
	"remote_server": {
		"url": "%s",
		"skipTLSVerify": true
	}
}`
	if _, err = fmt.Fprintf(clientConfig, template, filepath.Join(cliconfig.ConfigDir(), "trust"), notaryURL); err != nil {
		os.RemoveAll(tmp)
		return nil, err
	}

	// run notary-server
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

func notaryClientEnv(cmd *exec.Cmd, rootPwd, repositoryPwd string) {
	env := []string{
		fmt.Sprintf("NOTARY_ROOT_PASSPHRASE=%s", rootPwd),
		fmt.Sprintf("NOTARY_TARGETS_PASSPHRASE=%s", repositoryPwd),
		fmt.Sprintf("NOTARY_SNAPSHOT_PASSPHRASE=%s", repositoryPwd),
	}
	cmd.Env = append(os.Environ(), env...)
}

func (s *DockerTrustSuite) setupDelegations(c *check.C, repoName, pwd string) {
	initCmd := exec.Command(notaryClientBinary, "-c", filepath.Join(s.not.dir, "client-config.json"), "init", repoName)
	notaryClientEnv(initCmd, pwd, pwd)
	out, _, err := runCommandWithOutput(initCmd)
	if err != nil {
		c.Fatalf("Error initializing notary repository: %s\n", out)
	}

	// no command line for this, so build by hand
	nRepo, err := client.NewNotaryRepository(filepath.Join(cliconfig.ConfigDir(), "trust"), repoName, notaryURL, nil, passphrase.ConstantRetriever(pwd))
	if err != nil {
		c.Fatalf("Error creating notary repository: %s\n", err)
	}
	delgKey, err := nRepo.CryptoService.Create("targets/releases", data.ECDSAKey)
	if err != nil {
		c.Fatalf("Error creating delegation key: %s\n", err)
	}
	err = nRepo.AddDelegation("targets/releases", 1, []data.PublicKey{delgKey}, []string{""})
	if err != nil {
		c.Fatalf("Error creating delegation: %s\n", err)
	}

	// publishing first simulates the client pushing to a repo that they have been given delegated access to
	pubCmd := exec.Command(notaryClientBinary, "-c", filepath.Join(s.not.dir, "client-config.json"), "publish", repoName)
	notaryClientEnv(pubCmd, pwd, pwd)
	out, _, err = runCommandWithOutput(pubCmd)
	if err != nil {
		c.Fatalf("Error publishing notary repository: %s\n", out)
	}
}
