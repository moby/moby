// +build experimental
// +build !windows

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-check/check"
)

func init() {
	check.Suite(&ExternalVolumeSuite{
		ds: &DockerSuite{},
	})
}

type ExternalVolumeSuite struct {
	server *httptest.Server
	ds     *DockerSuite
}

func (s *ExternalVolumeSuite) SetUpTest(c *check.C) {
	s.ds.SetUpTest(c)
}

func (s *ExternalVolumeSuite) TearDownTest(c *check.C) {
	s.ds.TearDownTest(c)
}

func (s *ExternalVolumeSuite) SetUpSuite(c *check.C) {
	mux := http.NewServeMux()
	s.server = httptest.NewServer(mux)

	type pluginRequest struct {
		name string
	}

	hostVolumePath := func(name string) string {
		return fmt.Sprintf("/var/lib/docker/volumes/%s", name)
	}

	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "appplication/vnd.docker.plugins.v1+json")
		fmt.Fprintln(w, `{"Implements": ["VolumeDriver"]}`)
	})

	mux.HandleFunc("/VolumeDriver.Create", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "appplication/vnd.docker.plugins.v1+json")
		fmt.Fprintln(w, `{}`)
	})

	mux.HandleFunc("/VolumeDriver.Remove", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "appplication/vnd.docker.plugins.v1+json")
		fmt.Fprintln(w, `{}`)
	})

	mux.HandleFunc("/VolumeDriver.Path", func(w http.ResponseWriter, r *http.Request) {
		var pr pluginRequest
		if err := json.NewDecoder(r.Body).Decode(&pr); err != nil {
			http.Error(w, err.Error(), 500)
		}

		p := hostVolumePath(pr.name)

		w.Header().Set("Content-Type", "appplication/vnd.docker.plugins.v1+json")
		fmt.Fprintln(w, fmt.Sprintf("{\"Mountpoint\": \"%s\"}", p))
	})

	mux.HandleFunc("/VolumeDriver.Mount", func(w http.ResponseWriter, r *http.Request) {
		var pr pluginRequest
		if err := json.NewDecoder(r.Body).Decode(&pr); err != nil {
			http.Error(w, err.Error(), 500)
		}

		p := hostVolumePath(pr.name)
		if err := os.MkdirAll(p, 0755); err != nil {
			http.Error(w, err.Error(), 500)
		}

		if err := ioutil.WriteFile(filepath.Join(p, "test"), []byte(s.server.URL), 0644); err != nil {
			http.Error(w, err.Error(), 500)
		}

		w.Header().Set("Content-Type", "appplication/vnd.docker.plugins.v1+json")
		fmt.Fprintln(w, fmt.Sprintf("{\"Mountpoint\": \"%s\"}", p))
	})

	mux.HandleFunc("/VolumeDriver.Umount", func(w http.ResponseWriter, r *http.Request) {
		var pr pluginRequest
		if err := json.NewDecoder(r.Body).Decode(&pr); err != nil {
			http.Error(w, err.Error(), 500)
		}

		p := hostVolumePath(pr.name)
		if err := os.RemoveAll(p); err != nil {
			http.Error(w, err.Error(), 500)
		}

		w.Header().Set("Content-Type", "appplication/vnd.docker.plugins.v1+json")
		fmt.Fprintln(w, `{}`)
	})

	if err := os.MkdirAll("/usr/share/docker/plugins", 0755); err != nil {
		c.Fatal(err)
	}

	if err := ioutil.WriteFile("/usr/share/docker/plugins/test-external-volume-driver.spec", []byte(s.server.URL), 0644); err != nil {
		c.Fatal(err)
	}
}

func (s *ExternalVolumeSuite) TearDownSuite(c *check.C) {
	s.server.Close()

	if err := os.RemoveAll("/usr/share/docker/plugins"); err != nil {
		c.Fatal(err)
	}
}

func (s *ExternalVolumeSuite) TestStartExternalVolumeDriver(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "--name", "test-data", "-v", "external-volume-test:/tmp/external-volume-test", "--volume-driver", "test-external-volume-driver", "busybox:latest", "cat", "/tmp/external-volume-test/test")
	out, stderr, exitCode, err := runCommandWithStdoutStderr(runCmd)
	if err != nil && exitCode != 0 {
		c.Fatal(out, stderr, err)
	}

	if !strings.Contains(out, s.server.URL) {
		c.Fatalf("External volume mount failed. Output: %s\n", out)
	}
}
