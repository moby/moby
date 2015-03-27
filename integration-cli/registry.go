package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const v2binary = "registry-v2"

type testRegistryV2 struct {
	cmd *exec.Cmd
	url string
	dir string
}

func newTestRegistryV2At(t *testing.T, url string) (*testRegistryV2, error) {
	template := `version: 0.1
loglevel: debug
storage:
    filesystem:
        rootdirectory: %s
http:
    addr: %s`
	tmp, err := ioutil.TempDir("", "registry-test-")
	if err != nil {
		return nil, err
	}
	confPath := filepath.Join(tmp, "config.yaml")
	config, err := os.Create(confPath)
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(config, template, tmp, url); err != nil {
		os.RemoveAll(tmp)
		return nil, err
	}

	cmd := exec.Command(v2binary, confPath)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(tmp)
		if os.IsNotExist(err) {
			t.Skip()
		}
		return nil, err
	}
	return &testRegistryV2{
		cmd: cmd,
		url: url,
		dir: tmp,
	}, nil
}

func newTestRegistryV2(t *testing.T) (*testRegistryV2, error) {
	return newTestRegistryV2At(t, privateRegistryURLs[0])
}

func (t *testRegistryV2) Ping() error {
	// We always ping through HTTP for our test registry.
	resp, err := http.Get(fmt.Sprintf("http://%s/v2/", t.url))
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("registry ping replied with an unexpected status code %d", resp.StatusCode)
	}
	return nil
}

func (r *testRegistryV2) Close() {
	r.cmd.Process.Kill()
	os.RemoveAll(r.dir)
}
