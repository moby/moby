package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const v2binary = "registry-v2"

type testRegistryV2 struct {
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
	if _, err := fmt.Fprintf(config, template, tmp, privateRegistryURL); err != nil {
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
	}, nil
}

func (t *testRegistryV2) Ping() error {
	// We always ping through HTTP for our test registry.
	resp, err := http.Get(fmt.Sprintf("http://%s/v2/", privateRegistryURL))
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

func pingV1(ip string) error {
	// We always ping through HTTP for our test registry.
	resp, err := http.Get(fmt.Sprintf("http://%s/v1/search", ip))
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("registry ping replied with an unexpected status code %d", resp.StatusCode)
	}
	return nil
}

func startRegistryV1() error {
	//wait for registry image to be available
	for i := 0; i < 10; i++ {
		imagesCmd := exec.Command(dockerBinary, "images")
		out, _, err := runCommandWithOutput(imagesCmd)
		if err != nil {
			return err
		}
		if strings.Contains(out, "registry") {
			break
		}
		time.Sleep(60000 * time.Millisecond)
		if i == 10 {
			fmt.Errorf("No registry image is found to start the regictry V1 services")
		}
	}

	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--name", "regserver", "-d", "-p", "5000:5000", "registry", "docker-registry")); err != nil {
		fmt.Errorf("Failed to start registry: error %v, output %q", err, out)
	}
	ip := privateV1RegistryURL
	//wait until registry server is available
	for i := 0; i < 10; i++ {
		if err := pingV1(ip); err == nil {
			return nil
		} else if i == 10 && err != nil {
			return err
		}
		time.Sleep(2000 * time.Millisecond)
	}
	return nil
}
