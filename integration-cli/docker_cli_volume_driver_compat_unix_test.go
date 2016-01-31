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
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/integration/checker"

	"github.com/go-check/check"
)

func init() {
	check.Suite(&DockerExternalVolumeSuiteCompatV1_1{
		ds: &DockerSuite{},
	})
}

type DockerExternalVolumeSuiteCompatV1_1 struct {
	server *httptest.Server
	ds     *DockerSuite
	d      *Daemon
	ec     *eventCounter
}

func (s *DockerExternalVolumeSuiteCompatV1_1) SetUpTest(c *check.C) {
	s.d = NewDaemon(c)
	s.ec = &eventCounter{}
}

func (s *DockerExternalVolumeSuiteCompatV1_1) TearDownTest(c *check.C) {
	s.d.Stop()
	s.ds.TearDownTest(c)
}

func (s *DockerExternalVolumeSuiteCompatV1_1) SetUpSuite(c *check.C) {
	mux := http.NewServeMux()
	s.server = httptest.NewServer(mux)

	type pluginRequest struct {
		Name string
	}

	type pluginResp struct {
		Mountpoint string `json:",omitempty"`
		Err        string `json:",omitempty"`
	}

	type vol struct {
		Name       string
		Mountpoint string
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
		volList = append(volList, vol{Name: pr.Name})
		send(w, nil)
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

func (s *DockerExternalVolumeSuiteCompatV1_1) TearDownSuite(c *check.C) {
	s.server.Close()

	err := os.RemoveAll("/etc/docker/plugins")
	c.Assert(err, checker.IsNil)
}

func (s *DockerExternalVolumeSuiteCompatV1_1) TestExternalVolumeDriverCompatV1_1(c *check.C) {
	err := s.d.StartWithBusybox()
	c.Assert(err, checker.IsNil)

	out, err := s.d.Cmd("run", "--name=test", "-v", "foo:/bar", "--volume-driver", "test-external-volume-driver", "busybox", "sh", "-c", "echo hello > /bar/hello")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	out, err = s.d.Cmd("rm", "test")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("run", "--name=test2", "-v", "foo:/bar", "busybox", "cat", "/bar/hello")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(strings.TrimSpace(out), checker.Equals, "hello")

	err = s.d.Restart()
	c.Assert(err, checker.IsNil)

	out, err = s.d.Cmd("start", "-a", "test2")
	c.Assert(strings.TrimSpace(out), checker.Equals, "hello")

	out, err = s.d.Cmd("rm", "test2")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("volume", "inspect", "foo")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("volume", "rm", "foo")
	c.Assert(err, checker.IsNil, check.Commentf(out))
}
