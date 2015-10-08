// +build experimental
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
	"strings"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/daemon/graphdriver/vfs"
	"github.com/docker/docker/pkg/archive"
	"github.com/go-check/check"
)

func init() {
	check.Suite(&DockerExternalGraphdriverSuite{
		ds: &DockerSuite{},
	})
}

type DockerExternalGraphdriverSuite struct {
	server *httptest.Server
	ds     *DockerSuite
	d      *Daemon
	ec     *graphEventsCounter
}

type graphEventsCounter struct {
	activations int
	creations   int
	removals    int
	gets        int
	puts        int
	stats       int
	cleanups    int
	exists      int
	init        int
	metadata    int
	diff        int
	applydiff   int
	changes     int
	diffsize    int
}

func (s *DockerExternalGraphdriverSuite) SetUpTest(c *check.C) {
	s.d = NewDaemon(c)
	s.ec = &graphEventsCounter{}
}

func (s *DockerExternalGraphdriverSuite) TearDownTest(c *check.C) {
	s.d.Stop()
	s.ds.TearDownTest(c)
}

func (s *DockerExternalGraphdriverSuite) SetUpSuite(c *check.C) {
	mux := http.NewServeMux()
	s.server = httptest.NewServer(mux)

	type graphDriverRequest struct {
		ID         string `json:",omitempty"`
		Parent     string `json:",omitempty"`
		MountLabel string `json:",omitempty"`
	}

	type graphDriverResponse struct {
		Err      error             `json:",omitempty"`
		Dir      string            `json:",omitempty"`
		Exists   bool              `json:",omitempty"`
		Status   [][2]string       `json:",omitempty"`
		Metadata map[string]string `json:",omitempty"`
		Changes  []archive.Change  `json:",omitempty"`
		Size     int64             `json:",omitempty"`
	}

	respond := func(w http.ResponseWriter, data interface{}) {
		w.Header().Set("Content-Type", "appplication/vnd.docker.plugins.v1+json")
		switch t := data.(type) {
		case error:
			fmt.Fprintln(w, fmt.Sprintf(`{"Err": %q}`, t.Error()))
		case string:
			fmt.Fprintln(w, t)
		default:
			json.NewEncoder(w).Encode(&data)
		}
	}

	decReq := func(b io.ReadCloser, out interface{}, w http.ResponseWriter) error {
		defer b.Close()
		if err := json.NewDecoder(b).Decode(&out); err != nil {
			http.Error(w, fmt.Sprintf("error decoding json: %s", err.Error()), 500)
		}
		return nil
	}

	base, err := ioutil.TempDir("", "external-graph-test")
	c.Assert(err, check.IsNil)
	vfsProto, err := vfs.Init(base, []string{}, nil, nil)
	if err != nil {
		c.Fatalf("error initializing graph driver: %v", err)
	}
	driver := graphdriver.NewNaiveDiffDriver(vfsProto, nil, nil)

	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		s.ec.activations++
		respond(w, `{"Implements": ["GraphDriver"]}`)
	})

	mux.HandleFunc("/GraphDriver.Init", func(w http.ResponseWriter, r *http.Request) {
		s.ec.init++
		respond(w, "{}")
	})

	mux.HandleFunc("/GraphDriver.Create", func(w http.ResponseWriter, r *http.Request) {
		s.ec.creations++

		var req graphDriverRequest
		if err := decReq(r.Body, &req, w); err != nil {
			return
		}
		if err := driver.Create(req.ID, req.Parent); err != nil {
			respond(w, err)
			return
		}
		respond(w, "{}")
	})

	mux.HandleFunc("/GraphDriver.Remove", func(w http.ResponseWriter, r *http.Request) {
		s.ec.removals++

		var req graphDriverRequest
		if err := decReq(r.Body, &req, w); err != nil {
			return
		}

		if err := driver.Remove(req.ID); err != nil {
			respond(w, err)
			return
		}
		respond(w, "{}")
	})

	mux.HandleFunc("/GraphDriver.Get", func(w http.ResponseWriter, r *http.Request) {
		s.ec.gets++

		var req graphDriverRequest
		if err := decReq(r.Body, &req, w); err != nil {
			return
		}

		dir, err := driver.Get(req.ID, req.MountLabel)
		if err != nil {
			respond(w, err)
			return
		}
		respond(w, &graphDriverResponse{Dir: dir})
	})

	mux.HandleFunc("/GraphDriver.Put", func(w http.ResponseWriter, r *http.Request) {
		s.ec.puts++

		var req graphDriverRequest
		if err := decReq(r.Body, &req, w); err != nil {
			return
		}

		if err := driver.Put(req.ID); err != nil {
			respond(w, err)
			return
		}
		respond(w, "{}")
	})

	mux.HandleFunc("/GraphDriver.Exists", func(w http.ResponseWriter, r *http.Request) {
		s.ec.exists++

		var req graphDriverRequest
		if err := decReq(r.Body, &req, w); err != nil {
			return
		}
		respond(w, &graphDriverResponse{Exists: driver.Exists(req.ID)})
	})

	mux.HandleFunc("/GraphDriver.Status", func(w http.ResponseWriter, r *http.Request) {
		s.ec.stats++
		respond(w, &graphDriverResponse{Status: driver.Status()})
	})

	mux.HandleFunc("/GraphDriver.Cleanup", func(w http.ResponseWriter, r *http.Request) {
		s.ec.cleanups++
		err := driver.Cleanup()
		if err != nil {
			respond(w, err)
			return
		}
		respond(w, `{}`)
	})

	mux.HandleFunc("/GraphDriver.GetMetadata", func(w http.ResponseWriter, r *http.Request) {
		s.ec.metadata++

		var req graphDriverRequest
		if err := decReq(r.Body, &req, w); err != nil {
			return
		}

		data, err := driver.GetMetadata(req.ID)
		if err != nil {
			respond(w, err)
			return
		}
		respond(w, &graphDriverResponse{Metadata: data})
	})

	mux.HandleFunc("/GraphDriver.Diff", func(w http.ResponseWriter, r *http.Request) {
		s.ec.diff++

		var req graphDriverRequest
		if err := decReq(r.Body, &req, w); err != nil {
			return
		}

		diff, err := driver.Diff(req.ID, req.Parent)
		if err != nil {
			respond(w, err)
			return
		}
		io.Copy(w, diff)
	})

	mux.HandleFunc("/GraphDriver.Changes", func(w http.ResponseWriter, r *http.Request) {
		s.ec.changes++
		var req graphDriverRequest
		if err := decReq(r.Body, &req, w); err != nil {
			return
		}

		changes, err := driver.Changes(req.ID, req.Parent)
		if err != nil {
			respond(w, err)
			return
		}
		respond(w, &graphDriverResponse{Changes: changes})
	})

	mux.HandleFunc("/GraphDriver.ApplyDiff", func(w http.ResponseWriter, r *http.Request) {
		s.ec.applydiff++
		var diff archive.Reader = r.Body
		defer r.Body.Close()

		id := r.URL.Query().Get("id")
		parent := r.URL.Query().Get("parent")

		if id == "" {
			http.Error(w, fmt.Sprintf("missing id"), 409)
		}

		size, err := driver.ApplyDiff(id, parent, diff)
		if err != nil {
			respond(w, err)
			return
		}
		respond(w, &graphDriverResponse{Size: size})
	})

	mux.HandleFunc("/GraphDriver.DiffSize", func(w http.ResponseWriter, r *http.Request) {
		s.ec.diffsize++

		var req graphDriverRequest
		if err := decReq(r.Body, &req, w); err != nil {
			return
		}

		size, err := driver.DiffSize(req.ID, req.Parent)
		if err != nil {
			respond(w, err)
			return
		}
		respond(w, &graphDriverResponse{Size: size})
	})

	if err := os.MkdirAll("/etc/docker/plugins", 0755); err != nil {
		c.Fatal(err)
	}

	if err := ioutil.WriteFile("/etc/docker/plugins/test-external-graph-driver.spec", []byte(s.server.URL), 0644); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerExternalGraphdriverSuite) TearDownSuite(c *check.C) {
	s.server.Close()

	if err := os.RemoveAll("/etc/docker/plugins"); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerExternalGraphdriverSuite) TestExternalGraphDriver(c *check.C) {
	if err := s.d.StartWithBusybox("-s", "test-external-graph-driver"); err != nil {
		b, _ := ioutil.ReadFile(s.d.LogfileName())
		c.Assert(err, check.IsNil, check.Commentf("\n%s", string(b)))
	}

	out, err := s.d.Cmd("run", "-d", "--name=graphtest", "busybox", "sh", "-c", "echo hello > /hello")
	c.Assert(err, check.IsNil, check.Commentf(out))

	err = s.d.Restart("-s", "test-external-graph-driver")

	out, err = s.d.Cmd("inspect", "--format='{{.GraphDriver.Name}}'", "graphtest")
	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(strings.TrimSpace(out), check.Equals, "test-external-graph-driver")

	out, err = s.d.Cmd("diff", "graphtest")
	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(strings.Contains(out, "A /hello"), check.Equals, true)

	out, err = s.d.Cmd("rm", "-f", "graphtest")
	c.Assert(err, check.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("info")
	c.Assert(err, check.IsNil, check.Commentf(out))

	err = s.d.Stop()
	c.Assert(err, check.IsNil)

	c.Assert(s.ec.activations, check.Equals, 2)
	c.Assert(s.ec.init, check.Equals, 2)
	c.Assert(s.ec.creations >= 1, check.Equals, true)
	c.Assert(s.ec.removals >= 1, check.Equals, true)
	c.Assert(s.ec.gets >= 1, check.Equals, true)
	c.Assert(s.ec.puts >= 1, check.Equals, true)
	c.Assert(s.ec.stats, check.Equals, 3)
	c.Assert(s.ec.cleanups, check.Equals, 2)
	c.Assert(s.ec.exists >= 1, check.Equals, true)
	c.Assert(s.ec.applydiff >= 1, check.Equals, true)
	c.Assert(s.ec.changes, check.Equals, 1)
	c.Assert(s.ec.diffsize, check.Equals, 0)
	c.Assert(s.ec.diff, check.Equals, 0)
	c.Assert(s.ec.metadata, check.Equals, 1)
}

func (s *DockerExternalGraphdriverSuite) TestExternalGraphDriverPull(c *check.C) {
	testRequires(c, Network)
	c.Assert(s.d.Start(), check.IsNil)

	out, err := s.d.Cmd("pull", "busybox:latest")
	c.Assert(err, check.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("run", "-d", "busybox", "top")
	c.Assert(err, check.IsNil, check.Commentf(out))
}
