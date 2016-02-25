// +build !windows

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/engine-api/types"
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
	lists       int
	gets        int
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
		Name string
		Opts map[string]string
	}

	type pluginResp struct {
		Mountpoint string `json:",omitempty"`
		Err        string `json:",omitempty"`
	}

	type vol struct {
		Name       string
		Mountpoint string
		Ninja      bool // hack used to trigger an null volume return on `Get`
	}
	var volList []vol

	read := func(b io.ReadCloser) (pluginRequest, error) {
		defer b.Close()
		var pr pluginRequest
		if err := json.NewDecoder(b).Decode(&pr); err != nil {
			return pr, err
		}
		return pr, nil
	}

	send := func(w http.ResponseWriter, data interface{}) {
		switch t := data.(type) {
		case error:
			http.Error(w, t.Error(), 500)
		case string:
			w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
			fmt.Fprintln(w, t)
		default:
			w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
			json.NewEncoder(w).Encode(&data)
		}
	}

	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		s.ec.activations++
		send(w, `{"Implements": ["VolumeDriver"]}`)
	})

	mux.HandleFunc("/VolumeDriver.Create", func(w http.ResponseWriter, r *http.Request) {
		s.ec.creations++
		pr, err := read(r.Body)
		if err != nil {
			send(w, err)
			return
		}
		_, isNinja := pr.Opts["ninja"]
		volList = append(volList, vol{Name: pr.Name, Ninja: isNinja})
		send(w, nil)
	})

	mux.HandleFunc("/VolumeDriver.List", func(w http.ResponseWriter, r *http.Request) {
		s.ec.lists++
		send(w, map[string][]vol{"Volumes": volList})
	})

	mux.HandleFunc("/VolumeDriver.Get", func(w http.ResponseWriter, r *http.Request) {
		s.ec.gets++
		pr, err := read(r.Body)
		if err != nil {
			send(w, err)
			return
		}

		for _, v := range volList {
			if v.Name == pr.Name {
				if v.Ninja {
					send(w, map[string]vol{})
					return
				}
				v.Mountpoint = hostVolumePath(pr.Name)
				send(w, map[string]vol{"Volume": v})
				return
			}
		}
		send(w, `{"Err": "no such volume"}`)
	})

	mux.HandleFunc("/VolumeDriver.Remove", func(w http.ResponseWriter, r *http.Request) {
		s.ec.removals++
		pr, err := read(r.Body)
		if err != nil {
			send(w, err)
			return
		}

		if err := os.RemoveAll(hostVolumePath(pr.Name)); err != nil {
			send(w, &pluginResp{Err: err.Error()})
			return
		}

		for i, v := range volList {
			if v.Name == pr.Name {
				if err := os.RemoveAll(hostVolumePath(v.Name)); err != nil {
					send(w, fmt.Sprintf(`{"Err": "%v"}`, err))
					return
				}
				volList = append(volList[:i], volList[i+1:]...)
				break
			}
		}
		send(w, nil)
	})

	mux.HandleFunc("/VolumeDriver.Path", func(w http.ResponseWriter, r *http.Request) {
		s.ec.paths++

		pr, err := read(r.Body)
		if err != nil {
			send(w, err)
			return
		}
		p := hostVolumePath(pr.Name)
		send(w, &pluginResp{Mountpoint: p})
	})

	mux.HandleFunc("/VolumeDriver.Mount", func(w http.ResponseWriter, r *http.Request) {
		s.ec.mounts++

		pr, err := read(r.Body)
		if err != nil {
			send(w, err)
			return
		}

		p := hostVolumePath(pr.Name)
		if err := os.MkdirAll(p, 0755); err != nil {
			send(w, &pluginResp{Err: err.Error()})
			return
		}

		if err := ioutil.WriteFile(filepath.Join(p, "test"), []byte(s.server.URL), 0644); err != nil {
			send(w, err)
			return
		}

		send(w, &pluginResp{Mountpoint: p})
	})

	mux.HandleFunc("/VolumeDriver.Unmount", func(w http.ResponseWriter, r *http.Request) {
		s.ec.unmounts++

		_, err := read(r.Body)
		if err != nil {
			send(w, err)
			return
		}

		send(w, nil)
	})

	err := os.MkdirAll("/etc/docker/plugins", 0755)
	c.Assert(err, checker.IsNil)

	err = ioutil.WriteFile("/etc/docker/plugins/test-external-volume-driver.spec", []byte(s.server.URL), 0644)
	c.Assert(err, checker.IsNil)
}

func (s *DockerExternalVolumeSuite) TearDownSuite(c *check.C) {
	s.server.Close()

	err := os.RemoveAll("/etc/docker/plugins")
	c.Assert(err, checker.IsNil)
}

func (s *DockerExternalVolumeSuite) TestExternalVolumeDriverNamed(c *check.C) {
	err := s.d.StartWithBusybox()
	c.Assert(err, checker.IsNil)

	out, err := s.d.Cmd("run", "--rm", "--name", "test-data", "-v", "external-volume-test:/tmp/external-volume-test", "--volume-driver", "test-external-volume-driver", "busybox:latest", "cat", "/tmp/external-volume-test/test")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(out, checker.Contains, s.server.URL)

	_, err = s.d.Cmd("volume", "rm", "external-volume-test")
	c.Assert(err, checker.IsNil)

	p := hostVolumePath("external-volume-test")
	_, err = os.Lstat(p)
	c.Assert(err, checker.NotNil)
	c.Assert(os.IsNotExist(err), checker.True, check.Commentf("Expected volume path in host to not exist: %s, %v\n", p, err))

	c.Assert(s.ec.activations, checker.Equals, 1)
	c.Assert(s.ec.creations, checker.Equals, 1)
	c.Assert(s.ec.removals, checker.Equals, 1)
	c.Assert(s.ec.mounts, checker.Equals, 1)
	c.Assert(s.ec.unmounts, checker.Equals, 1)
}

func (s *DockerExternalVolumeSuite) TestExternalVolumeDriverUnnamed(c *check.C) {
	err := s.d.StartWithBusybox()
	c.Assert(err, checker.IsNil)

	out, err := s.d.Cmd("run", "--rm", "--name", "test-data", "-v", "/tmp/external-volume-test", "--volume-driver", "test-external-volume-driver", "busybox:latest", "cat", "/tmp/external-volume-test/test")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(out, checker.Contains, s.server.URL)

	c.Assert(s.ec.activations, checker.Equals, 1)
	c.Assert(s.ec.creations, checker.Equals, 1)
	c.Assert(s.ec.removals, checker.Equals, 1)
	c.Assert(s.ec.mounts, checker.Equals, 1)
	c.Assert(s.ec.unmounts, checker.Equals, 1)
}

func (s DockerExternalVolumeSuite) TestExternalVolumeDriverVolumesFrom(c *check.C) {
	err := s.d.StartWithBusybox()
	c.Assert(err, checker.IsNil)

	out, err := s.d.Cmd("run", "-d", "--name", "vol-test1", "-v", "/foo", "--volume-driver", "test-external-volume-driver", "busybox:latest")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("run", "--rm", "--volumes-from", "vol-test1", "--name", "vol-test2", "busybox", "ls", "/tmp")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("rm", "-fv", "vol-test1")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	c.Assert(s.ec.activations, checker.Equals, 1)
	c.Assert(s.ec.creations, checker.Equals, 1)
	c.Assert(s.ec.removals, checker.Equals, 1)
	c.Assert(s.ec.mounts, checker.Equals, 2)
	c.Assert(s.ec.unmounts, checker.Equals, 2)
}

func (s DockerExternalVolumeSuite) TestExternalVolumeDriverDeleteContainer(c *check.C) {
	err := s.d.StartWithBusybox()
	c.Assert(err, checker.IsNil)

	out, err := s.d.Cmd("run", "-d", "--name", "vol-test1", "-v", "/foo", "--volume-driver", "test-external-volume-driver", "busybox:latest")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("rm", "-fv", "vol-test1")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	c.Assert(s.ec.activations, checker.Equals, 1)
	c.Assert(s.ec.creations, checker.Equals, 1)
	c.Assert(s.ec.removals, checker.Equals, 1)
	c.Assert(s.ec.mounts, checker.Equals, 1)
	c.Assert(s.ec.unmounts, checker.Equals, 1)
}

func hostVolumePath(name string) string {
	return fmt.Sprintf("/var/lib/docker/volumes/%s", name)
}

// Make sure a request to use a down driver doesn't block other requests
func (s *DockerExternalVolumeSuite) TestExternalVolumeDriverLookupNotBlocked(c *check.C) {
	specPath := "/etc/docker/plugins/down-driver.spec"
	err := ioutil.WriteFile(specPath, []byte("tcp://127.0.0.7:9999"), 0644)
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(specPath)

	chCmd1 := make(chan struct{})
	chCmd2 := make(chan error)
	cmd1 := exec.Command(dockerBinary, "volume", "create", "-d", "down-driver")
	cmd2 := exec.Command(dockerBinary, "volume", "create")

	c.Assert(cmd1.Start(), checker.IsNil)
	defer cmd1.Process.Kill()
	time.Sleep(100 * time.Millisecond) // ensure API has been called
	c.Assert(cmd2.Start(), checker.IsNil)

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
		c.Assert(err, checker.IsNil)
	case <-time.After(5 * time.Second):
		cmd2.Process.Kill()
		c.Fatal("volume creates are blocked by previous create requests when previous driver is down")
	}
}

func (s *DockerExternalVolumeSuite) TestExternalVolumeDriverRetryNotImmediatelyExists(c *check.C) {
	err := s.d.StartWithBusybox()
	c.Assert(err, checker.IsNil)

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
		// no need to check for an error here since it will get picked up by the timeout later
		ioutil.WriteFile(specPath, []byte(s.server.URL), 0644)
	}()

	select {
	case err := <-errchan:
		c.Assert(err, checker.IsNil)
	case <-time.After(8 * time.Second):
		c.Fatal("volume creates fail when plugin not immediately available")
	}

	_, err = s.d.Cmd("volume", "rm", "external-volume-test")
	c.Assert(err, checker.IsNil)

	c.Assert(s.ec.activations, checker.Equals, 1)
	c.Assert(s.ec.creations, checker.Equals, 1)
	c.Assert(s.ec.removals, checker.Equals, 1)
	c.Assert(s.ec.mounts, checker.Equals, 1)
	c.Assert(s.ec.unmounts, checker.Equals, 1)
}

func (s *DockerExternalVolumeSuite) TestExternalVolumeDriverBindExternalVolume(c *check.C) {
	dockerCmd(c, "volume", "create", "-d", "test-external-volume-driver", "--name", "foo")
	dockerCmd(c, "run", "-d", "--name", "testing", "-v", "foo:/bar", "busybox", "top")

	var mounts []struct {
		Name   string
		Driver string
	}
	out := inspectFieldJSON(c, "testing", "Mounts")
	c.Assert(json.NewDecoder(strings.NewReader(out)).Decode(&mounts), checker.IsNil)
	c.Assert(len(mounts), checker.Equals, 1, check.Commentf(out))
	c.Assert(mounts[0].Name, checker.Equals, "foo")
	c.Assert(mounts[0].Driver, checker.Equals, "test-external-volume-driver")
}

func (s *DockerExternalVolumeSuite) TesttExternalVolumeDriverList(c *check.C) {
	dockerCmd(c, "volume", "create", "-d", "test-external-volume-driver", "--name", "abc")
	out, _ := dockerCmd(c, "volume", "ls")
	ls := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(ls), check.Equals, 2, check.Commentf("\n%s", out))

	vol := strings.Fields(ls[len(ls)-1])
	c.Assert(len(vol), check.Equals, 2, check.Commentf("%v", vol))
	c.Assert(vol[0], check.Equals, "test-external-volume-driver")
	c.Assert(vol[1], check.Equals, "abc")

	c.Assert(s.ec.lists, check.Equals, 1)
}

func (s *DockerExternalVolumeSuite) TestExternalVolumeDriverGet(c *check.C) {
	out, _, err := dockerCmdWithError("volume", "inspect", "dummy")
	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(s.ec.gets, check.Equals, 1)
	c.Assert(out, checker.Contains, "No such volume")
}

func (s *DockerExternalVolumeSuite) TestExternalVolumeDriverWithDaemnRestart(c *check.C) {
	dockerCmd(c, "volume", "create", "-d", "test-external-volume-driver", "--name", "abc")
	err := s.d.Restart()
	c.Assert(err, checker.IsNil)

	dockerCmd(c, "run", "--name=test", "-v", "abc:/foo", "busybox", "true")
	var mounts []types.MountPoint
	inspectFieldAndMarshall(c, "test", "Mounts", &mounts)
	c.Assert(mounts, checker.HasLen, 1)
	c.Assert(mounts[0].Driver, checker.Equals, "test-external-volume-driver")
}

// Ensures that the daemon handles when the plugin responds to a `Get` request with a null volume and a null error.
// Prior the daemon would panic in this scenario.
func (s *DockerExternalVolumeSuite) TestExternalVolumeDriverGetEmptyResponse(c *check.C) {
	dockerCmd(c, "volume", "create", "-d", "test-external-volume-driver", "--name", "abc", "--opt", "ninja=1")
	out, _, err := dockerCmdWithError("volume", "inspect", "abc")
	c.Assert(err, checker.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "No such volume")
}
