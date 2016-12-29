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

	cliconfig "github.com/docker/docker/cli/config"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/go-check/check"
)

var notaryBinary = "notary"
var notaryServerBinary = "notary-server"

type keyPair struct {
	Public  string
	Private string
}

type testNotary struct {
	cmd  *exec.Cmd
	dir  string
	keys []keyPair
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
	if err != nil {
		return nil, err
	}
	defer config.Close()

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
	if err != nil {
		return nil, err
	}
	defer clientConfig.Close()

	template = `{
	"trust_dir" : "%s",
	"remote_server": {
		"url": "%s",
		"skipTLSVerify": true
	}
}`
	if _, err = fmt.Fprintf(clientConfig, template, filepath.Join(cliconfig.Dir(), "trust"), notaryURL); err != nil {
		os.RemoveAll(tmp)
		return nil, err
	}

	// load key fixture filenames
	var keys []keyPair
	for i := 1; i < 5; i++ {
		keys = append(keys, keyPair{
			Public:  filepath.Join(workingDir, fmt.Sprintf("fixtures/notary/delgkey%v.crt", i)),
			Private: filepath.Join(workingDir, fmt.Sprintf("fixtures/notary/delgkey%v.key", i)),
		})
	}

	// run notary-server
	cmd := exec.Command(notaryServerBinary, "-config", confPath)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(tmp)
		if os.IsNotExist(err) {
			c.Skip(err.Error())
		}
		return nil, err
	}

	testNotary := &testNotary{
		cmd:  cmd,
		dir:  tmp,
		keys: keys,
	}

	// Wait for notary to be ready to serve requests.
	for i := 1; i <= 20; i++ {
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
	tlsConfig := tlsconfig.ClientDefault()
	tlsConfig.InsecureSkipVerify = true
	client := http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig:     tlsConfig,
		},
	}
	resp, err := client.Get(fmt.Sprintf("%s/v2/", notaryURL))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
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

func trustCmdEnv(cmd *exec.Cmd, server, rootPwd, repositoryPwd string) {
	env := []string{
		"DOCKER_CONTENT_TRUST=1",
		fmt.Sprintf("DOCKER_CONTENT_TRUST_SERVER=%s", server),
		fmt.Sprintf("DOCKER_CONTENT_TRUST_ROOT_PASSPHRASE=%s", rootPwd),
		fmt.Sprintf("DOCKER_CONTENT_TRUST_REPOSITORY_PASSPHRASE=%s", repositoryPwd),
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

func (s *DockerTrustSuite) setupTrustedplugin(c *check.C, source, name string) string {
	repoName := fmt.Sprintf("%v/dockercli/%s:latest", privateRegistryURL, name)
	// tag the image and upload it to the private registry
	dockerCmd(c, "plugin", "install", "--grant-all-permissions", "--alias", repoName, source)

	pushCmd := exec.Command(dockerBinary, "plugin", "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)

	if err != nil {
		c.Fatalf("Error running trusted plugin push: %s\n%s", err, out)
	}
	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	if out, status := dockerCmd(c, "plugin", "rm", "-f", repoName); status != 0 {
		c.Fatalf("Error removing plugin %q\n%s", repoName, out)
	}

	return repoName
}

func notaryClientEnv(cmd *exec.Cmd) {
	pwd := "12345678"
	env := []string{
		fmt.Sprintf("NOTARY_ROOT_PASSPHRASE=%s", pwd),
		fmt.Sprintf("NOTARY_TARGETS_PASSPHRASE=%s", pwd),
		fmt.Sprintf("NOTARY_SNAPSHOT_PASSPHRASE=%s", pwd),
		fmt.Sprintf("NOTARY_DELEGATION_PASSPHRASE=%s", pwd),
	}
	cmd.Env = append(os.Environ(), env...)
}

func (s *DockerTrustSuite) notaryInitRepo(c *check.C, repoName string) {
	initCmd := exec.Command(notaryBinary, "-c", filepath.Join(s.not.dir, "client-config.json"), "init", repoName)
	notaryClientEnv(initCmd)
	out, _, err := runCommandWithOutput(initCmd)
	if err != nil {
		c.Fatalf("Error initializing notary repository: %s\n", out)
	}
}

func (s *DockerTrustSuite) notaryCreateDelegation(c *check.C, repoName, role string, pubKey string, paths ...string) {
	pathsArg := "--all-paths"
	if len(paths) > 0 {
		pathsArg = "--paths=" + strings.Join(paths, ",")
	}

	delgCmd := exec.Command(notaryBinary, "-c", filepath.Join(s.not.dir, "client-config.json"),
		"delegation", "add", repoName, role, pubKey, pathsArg)
	notaryClientEnv(delgCmd)
	out, _, err := runCommandWithOutput(delgCmd)
	if err != nil {
		c.Fatalf("Error adding %s role to notary repository: %s\n", role, out)
	}
}

func (s *DockerTrustSuite) notaryPublish(c *check.C, repoName string) {
	pubCmd := exec.Command(notaryBinary, "-c", filepath.Join(s.not.dir, "client-config.json"), "publish", repoName)
	notaryClientEnv(pubCmd)
	out, _, err := runCommandWithOutput(pubCmd)
	if err != nil {
		c.Fatalf("Error publishing notary repository: %s\n", out)
	}
}

func (s *DockerTrustSuite) notaryImportKey(c *check.C, repoName, role string, privKey string) {
	impCmd := exec.Command(notaryBinary, "-c", filepath.Join(s.not.dir, "client-config.json"), "key",
		"import", privKey, "-g", repoName, "-r", role)
	notaryClientEnv(impCmd)
	out, _, err := runCommandWithOutput(impCmd)
	if err != nil {
		c.Fatalf("Error importing key to notary repository: %s\n", out)
	}
}

func (s *DockerTrustSuite) notaryListTargetsInRole(c *check.C, repoName, role string) map[string]string {
	listCmd := exec.Command(notaryBinary, "-c", filepath.Join(s.not.dir, "client-config.json"), "list",
		repoName, "-r", role)
	notaryClientEnv(listCmd)
	out, _, err := runCommandWithOutput(listCmd)
	if err != nil {
		c.Fatalf("Error listing targets in notary repository: %s\n", out)
	}

	// should look something like:
	//    NAME                                 DIGEST                                SIZE (BYTES)    ROLE
	// ------------------------------------------------------------------------------------------------------
	//   latest   24a36bbc059b1345b7e8be0df20f1b23caa3602e85d42fff7ecd9d0bd255de56   1377           targets

	targets := make(map[string]string)

	// no target
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 1 && strings.Contains(out, "No targets present in this repository.") {
		return targets
	}

	// otherwise, there is at least one target
	c.Assert(len(lines), checker.GreaterOrEqualThan, 3)

	for _, line := range lines[2:] {
		tokens := strings.Fields(line)
		c.Assert(tokens, checker.HasLen, 4)
		targets[tokens[0]] = tokens[3]
	}

	return targets
}

func (s *DockerTrustSuite) assertTargetInRoles(c *check.C, repoName, target string, roles ...string) {
	// check all the roles
	for _, role := range roles {
		targets := s.notaryListTargetsInRole(c, repoName, role)
		roleName, ok := targets[target]
		c.Assert(ok, checker.True)
		c.Assert(roleName, checker.Equals, role)
	}
}

func (s *DockerTrustSuite) assertTargetNotInRoles(c *check.C, repoName, target string, roles ...string) {
	targets := s.notaryListTargetsInRole(c, repoName, "targets")

	roleName, ok := targets[target]
	if ok {
		for _, role := range roles {
			c.Assert(roleName, checker.Not(checker.Equals), role)
		}
	}
}
