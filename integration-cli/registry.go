package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const v2binary = "registry-v2"

type testRegistryV2 struct {
	URL string
	cmd *exec.Cmd
	dir string
}

func newTestRegistryV2(t *testing.T) (*testRegistryV2, error) {
	template := `version: 0.1
loglevel: debug
storage:
    filesystem:
        rootdirectory: %s
http:
    addr: :%s`
	tmp, err := ioutil.TempDir("", "registry-test-")
	if err != nil {
		return nil, err
	}
	confPath := filepath.Join(tmp, "config.yaml")
	config, err := os.Create(confPath)
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(config, template, tmp, "5000"); err != nil {
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
		dir: tmp,
		URL: "localhost:5000",
	}, nil
}

func (r *testRegistryV2) Close() {
	r.cmd.Process.Kill()
	os.RemoveAll(r.dir)
}
