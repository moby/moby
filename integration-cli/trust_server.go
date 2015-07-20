package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/docker/docker/pkg/tlsconfig"
	"github.com/go-check/check"
)

var notaryBinary = "notary-server"

type testNotary struct {
	cmd *exec.Cmd
	dir string
}

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
		"port": ""
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
	if _, err := fmt.Fprintf(config, template, "localhost:4443"); err != nil {
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
	return &testNotary{
		cmd: cmd,
		dir: tmp,
	}, nil
}

func (t *testNotary) address() string {
	return "localhost:4443"
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
	resp, err := client.Get(fmt.Sprintf("https://%s/v2/", t.address()))
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
	env := []string{
		"DOCKER_TRUST=1",
		fmt.Sprintf("DOCKER_TRUST_SERVER=%s", s.not.address()),
		"DOCKER_TRUST_ROOT_PASSPHRASE=12345678",
		"DOCKER_TRUST_TARGET_PASSPHRASE=12345678",
		"DOCKER_TRUST_SNAPSHOT_PASSPHRASE=12345678",
	}
	cmd.Env = append(os.Environ(), env...)
}
