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
	"time"

	"github.com/go-check/check"
)

func init() {
	check.Suite(&DockerExternalVolumeSuite{
		ds: &DockerSuite{},
	})
}

type eventCounter struct {
	activations int
	creations   int
	removals    int
	mounts      int
	unmounts    int
	paths       int
}

type DockerExternalVolumeSuite struct {
	server *httptest.Server
	ds     *DockerSuite
	d      *Daemon
	ec     *eventCounter
}

func (s *DockerExternalVolumeSuite) SetUpTest(c *check.C) {
	s.d = NewDaemon(c)
	s.ec = &eventCounter{}
}

func (s *DockerExternalVolumeSuite) TearDownTest(c *check.C) {
	s.d.Stop()
	s.ds.TearDownTest(c)
}

func (s *DockerExternalVolumeSuite) SetUpSuite(c *check.C) {
	mux := http.NewServeMux()
	s.server = httptest.NewServer(mux)

	type pluginRequest struct {
		name string
	}

	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		s.ec.activations++

		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		fmt.Fprintln(w, `{"Implements": ["VolumeDriver"]}`)
	})

	mux.HandleFunc("/VolumeDriver.Create", func(w http.ResponseWriter, r *http.Request) {
		s.ec.creations++

		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		fmt.Fprintln(w, `{}`)
	})

	mux.HandleFunc("/VolumeDriver.Remove", func(w http.ResponseWriter, r *http.Request) {
		s.ec.removals++

		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		fmt.Fprintln(w, `{}`)
	})

	mux.HandleFunc("/VolumeDriver.Path", func(w http.ResponseWriter, r *http.Request) {
		s.ec.paths++

		var pr pluginRequest
		if err := json.NewDecoder(r.Body).Decode(&pr); err != nil {
			http.Error(w, err.Error(), 500)
		}

		p := hostVolumePath(pr.name)

		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		fmt.Fprintln(w, fmt.Sprintf("{\"Mountpoint\": \"%s\"}", p))
	})

	mux.HandleFunc("/VolumeDriver.Mount", func(w http.ResponseWriter, r *http.Request) {
		s.ec.mounts++

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

		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		fmt.Fprintln(w, fmt.Sprintf("{\"Mountpoint\": \"%s\"}", p))
	})

	mux.HandleFunc("/VolumeDriver.Unmount", func(w http.ResponseWriter, r *http.Request) {
		s.ec.unmounts++

		var pr pluginRequest
		if err := json.NewDecoder(r.Body).Decode(&pr); err != nil {
			http.Error(w, err.Error(), 500)
		}

		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		fmt.Fprintln(w, `{}`)
	})

	if err := os.MkdirAll("/etc/docker/plugins", 0755); err != nil {
		c.Fatal(err)
	}

	if err := ioutil.WriteFile("/etc/docker/plugins/test-external-volume-driver.spec", []byte(s.server.URL), 0644); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerExternalVolumeSuite) TearDownSuite(c *check.C) {
	s.server.Close()

	if err := os.RemoveAll("/etc/docker/plugins"); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerExternalVolumeSuite) TestStartExternalNamedVolumeDriver(c *check.C) {
	if err := s.d.StartWithBusybox(); err != nil {
		c.Fatal(err)
	}

	out, err := s.d.Cmd("run", "--rm", "--name", "test-data", "-v", "external-volume-test:/tmp/external-volume-test", "--volume-driver", "test-external-volume-driver", "busybox:latest", "cat", "/tmp/external-volume-test/test")
	if err != nil {
		c.Fatal(out, err)
	}

	if !strings.Contains(out, s.server.URL) {
		c.Fatalf("External volume mount failed. Output: %s\n", out)
	}

	p := hostVolumePath("external-volume-test")
	_, err = os.Lstat(p)
	if err == nil {
		c.Fatalf("Expected error checking volume path in host: %s\n", p)
	}

	if !os.IsNotExist(err) {
		c.Fatalf("Expected volume path in host to not exist: %s, %v\n", p, err)
	}

	c.Assert(s.ec.activations, check.Equals, 1)
	c.Assert(s.ec.creations, check.Equals, 1)
	c.Assert(s.ec.removals, check.Equals, 1)
	c.Assert(s.ec.mounts, check.Equals, 1)
	c.Assert(s.ec.unmounts, check.Equals, 1)
}

func (s *DockerExternalVolumeSuite) TestStartExternalVolumeUnnamedDriver(c *check.C) {
	if err := s.d.StartWithBusybox(); err != nil {
		c.Fatal(err)
	}

	out, err := s.d.Cmd("run", "--rm", "--name", "test-data", "-v", "/tmp/external-volume-test", "--volume-driver", "test-external-volume-driver", "busybox:latest", "cat", "/tmp/external-volume-test/test")
	if err != nil {
		c.Fatal(err)
	}

	if !strings.Contains(out, s.server.URL) {
		c.Fatalf("External volume mount failed. Output: %s\n", out)
	}

	c.Assert(s.ec.activations, check.Equals, 1)
	c.Assert(s.ec.creations, check.Equals, 1)
	c.Assert(s.ec.removals, check.Equals, 1)
	c.Assert(s.ec.mounts, check.Equals, 1)
	c.Assert(s.ec.unmounts, check.Equals, 1)
}

func (s DockerExternalVolumeSuite) TestStartExternalVolumeDriverVolumesFrom(c *check.C) {
	if err := s.d.StartWithBusybox(); err != nil {
		c.Fatal(err)
	}

	out, err := s.d.Cmd("run", "-d", "--name", "vol-test1", "-v", "/foo", "--volume-driver", "test-external-volume-driver", "busybox:latest")
	c.Assert(err, check.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("run", "--rm", "--volumes-from", "vol-test1", "--name", "vol-test2", "busybox", "ls", "/tmp")
	c.Assert(err, check.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("rm", "-fv", "vol-test1")
	c.Assert(err, check.IsNil, check.Commentf(out))

	c.Assert(s.ec.activations, check.Equals, 1)
	c.Assert(s.ec.creations, check.Equals, 1)
	c.Assert(s.ec.removals, check.Equals, 1)
	c.Assert(s.ec.mounts, check.Equals, 2)
	c.Assert(s.ec.unmounts, check.Equals, 2)
}

func (s DockerExternalVolumeSuite) TestStartExternalVolumeDriverDeleteContainer(c *check.C) {
	if err := s.d.StartWithBusybox(); err != nil {
		c.Fatal(err)
	}

	if out, err := s.d.Cmd("run", "-d", "--name", "vol-test1", "-v", "/foo", "--volume-driver", "test-external-volume-driver", "busybox:latest"); err != nil {
		c.Fatal(out, err)
	}

	if out, err := s.d.Cmd("rm", "-fv", "vol-test1"); err != nil {
		c.Fatal(out, err)
	}

	c.Assert(s.ec.activations, check.Equals, 1)
	c.Assert(s.ec.creations, check.Equals, 1)
	c.Assert(s.ec.removals, check.Equals, 1)
	c.Assert(s.ec.mounts, check.Equals, 1)
	c.Assert(s.ec.unmounts, check.Equals, 1)
}

func hostVolumePath(name string) string {
	return fmt.Sprintf("/var/lib/docker/volumes/%s", name)
}

func (s *DockerExternalVolumeSuite) TestStartExternalNamedVolumeDriverCheckBindLocalVolume(c *check.C) {
	if err := s.d.StartWithBusybox(); err != nil {
		c.Fatal(err)
	}

	expected := s.server.URL

	dockerfile := fmt.Sprintf(`FROM busybox:latest
	RUN mkdir /nobindthenlocalvol
	RUN echo %s > /nobindthenlocalvol/test
	VOLUME ["/nobindthenlocalvol"]`, expected)

	img := "test-checkbindlocalvolume"

	_, err := buildImageWithOutInDamon(s.d.sock(), img, dockerfile, true)
	c.Assert(err, check.IsNil)

	out, err := s.d.Cmd("run", "--rm", "--name", "test-data-nobind", "-v", "external-volume-test:/tmp/external-volume-test", "--volume-driver", "test-external-volume-driver", img, "cat", "/nobindthenlocalvol/test")
	c.Assert(err, check.IsNil)

	if !strings.Contains(out, expected) {
		c.Fatalf("External volume mount failed. Output: %s\n", out)
	}

	c.Assert(s.ec.activations, check.Equals, 1)
	c.Assert(s.ec.creations, check.Equals, 1)
	c.Assert(s.ec.removals, check.Equals, 1)
	c.Assert(s.ec.mounts, check.Equals, 1)
	c.Assert(s.ec.unmounts, check.Equals, 1)
}

// Make sure a request to use a down driver doesn't block other requests
func (s *DockerExternalVolumeSuite) TestStartExternalVolumeDriverLookupNotBlocked(c *check.C) {
	specPath := "/etc/docker/plugins/down-driver.spec"
	err := ioutil.WriteFile("/etc/docker/plugins/down-driver.spec", []byte("tcp://127.0.0.7:9999"), 0644)
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(specPath)

	chCmd1 := make(chan struct{})
	chCmd2 := make(chan error)
	cmd1 := exec.Command(dockerBinary, "volume", "create", "-d", "down-driver")
	cmd2 := exec.Command(dockerBinary, "volume", "create")

	c.Assert(cmd1.Start(), check.IsNil)
	defer cmd1.Process.Kill()
	time.Sleep(100 * time.Millisecond) // ensure API has been called
	c.Assert(cmd2.Start(), check.IsNil)

	go func() {
		cmd1.Wait()
		close(chCmd1)
	}()
	go func() {
		chCmd2 <- cmd2.Wait()
	}()

	select {
	case <-chCmd1:
		cmd2.Process.Kill()
		c.Fatalf("volume create with down driver finished unexpectedly")
	case err := <-chCmd2:
		c.Assert(err, check.IsNil, check.Commentf("error creating volume"))
	case <-time.After(5 * time.Second):
		c.Fatal("volume creates are blocked by previous create requests when previous driver is down")
		cmd2.Process.Kill()
	}
}

func (s *DockerExternalVolumeSuite) TestStartExternalVolumeDriverRetryNotImmediatelyExists(c *check.C) {
	if err := s.d.StartWithBusybox(); err != nil {
		c.Fatal(err)
	}

	specPath := "/etc/docker/plugins/test-external-volume-driver-retry.spec"
	os.RemoveAll(specPath)
	defer os.RemoveAll(specPath)

	errchan := make(chan error)
	go func() {
		if out, err := s.d.Cmd("run", "--rm", "--name", "test-data-retry", "-v", "external-volume-test:/tmp/external-volume-test", "--volume-driver", "test-external-volume-driver-retry", "busybox:latest"); err != nil {
			errchan <- fmt.Errorf("%v:\n%s", err, out)
		}
		close(errchan)
	}()
	go func() {
		// wait for a retry to occur, then create spec to allow plugin to register
		time.Sleep(2000 * time.Millisecond)
		if err := ioutil.WriteFile(specPath, []byte(s.server.URL), 0644); err != nil {
			c.Fatal(err)
		}
	}()

	select {
	case err := <-errchan:
		if err != nil {
			c.Fatal(err)
		}
	case <-time.After(8 * time.Second):
		c.Fatal("volume creates fail when plugin not immediately available")
	}

	c.Assert(s.ec.activations, check.Equals, 1)
	c.Assert(s.ec.creations, check.Equals, 1)
	c.Assert(s.ec.removals, check.Equals, 1)
	c.Assert(s.ec.mounts, check.Equals, 1)
	c.Assert(s.ec.unmounts, check.Equals, 1)
}

func (s *DockerExternalVolumeSuite) TestStartExternalVolumeDriverBindExternalVolume(c *check.C) {
	dockerCmd(c, "volume", "create", "-d", "test-external-volume-driver", "--name", "foo")
	dockerCmd(c, "run", "-d", "--name", "testing", "-v", "foo:/bar", "busybox", "top")

	var mounts []struct {
		Name   string
		Driver string
	}
	out, err := inspectFieldJSON("testing", "Mounts")
	c.Assert(err, check.IsNil)
	c.Assert(json.NewDecoder(strings.NewReader(out)).Decode(&mounts), check.IsNil)
	c.Assert(len(mounts), check.Equals, 1, check.Commentf(out))
	c.Assert(mounts[0].Name, check.Equals, "foo")
	c.Assert(mounts[0].Driver, check.Equals, "test-external-volume-driver")
}
