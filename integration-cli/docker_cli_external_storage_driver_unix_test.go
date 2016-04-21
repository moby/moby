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

	"github.com/docker/docker/daemon/storage"
	"github.com/docker/docker/daemon/storage/vfs"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/plugins"
	"github.com/go-check/check"
)

func init() {
	check.Suite(&DockerExternalStorageDriverSuite{
		ds: &DockerSuite{},
	})
}

type DockerExternalStorageDriverSuite struct {
	server  *httptest.Server
	jserver *httptest.Server
	ds      *DockerSuite
	d       *Daemon
	ec      map[string]*storageEventsCounter
}

type storageEventsCounter struct {
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

func (s *DockerExternalStorageDriverSuite) SetUpTest(c *check.C) {
	s.d = NewDaemon(c)
}

func (s *DockerExternalStorageDriverSuite) TearDownTest(c *check.C) {
	s.d.Stop()
	s.ds.TearDownTest(c)
}

func (s *DockerExternalStorageDriverSuite) SetUpSuite(c *check.C) {
	s.ec = make(map[string]*storageEventsCounter)
	s.setUpPluginViaSpecFile(c)
	s.setUpPluginViaJSONFile(c)
}

func (s *DockerExternalStorageDriverSuite) setUpPluginViaSpecFile(c *check.C) {
	mux := http.NewServeMux()
	s.server = httptest.NewServer(mux)

	s.setUpPlugin(c, "test-external-storage-driver", "spec", mux, []byte(s.server.URL))
}

func (s *DockerExternalStorageDriverSuite) setUpPluginViaJSONFile(c *check.C) {
	mux := http.NewServeMux()
	s.jserver = httptest.NewServer(mux)

	p := plugins.Plugin{Name: "json-external-storage-driver", Addr: s.jserver.URL}
	b, err := json.Marshal(p)
	c.Assert(err, check.IsNil)

	s.setUpPlugin(c, "json-external-storage-driver", "json", mux, b)
}

func (s *DockerExternalStorageDriverSuite) setUpPlugin(c *check.C, name string, ext string, mux *http.ServeMux, b []byte) {
	type storageDriverRequest struct {
		ID         string `json:",omitempty"`
		Parent     string `json:",omitempty"`
		MountLabel string `json:",omitempty"`
		ReadOnly   bool   `json:",omitempty"`
	}

	type storageDriverResponse struct {
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

	base, err := ioutil.TempDir("", name)
	c.Assert(err, check.IsNil)
	vfsProto, err := vfs.Init(base, []string{}, nil, nil)
	c.Assert(err, check.IsNil, check.Commentf("error initializing storage driver"))
	driver := storage.NewNaiveDiffDriver(vfsProto, nil, nil)

	s.ec[ext] = &storageEventsCounter{}
	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		s.ec[ext].activations++
		respond(w, `{"Implements": ["StorageDriver"]}`)
	})

	mux.HandleFunc("/StorageDriver.Init", func(w http.ResponseWriter, r *http.Request) {
		s.ec[ext].init++
		respond(w, "{}")
	})

	mux.HandleFunc("/StorageDriver.CreateReadWrite", func(w http.ResponseWriter, r *http.Request) {
		s.ec[ext].creations++

		var req storageDriverRequest
		if err := decReq(r.Body, &req, w); err != nil {
			return
		}
		if err := driver.CreateReadWrite(req.ID, req.Parent, "", nil); err != nil {
			respond(w, err)
			return
		}
		respond(w, "{}")
	})

	mux.HandleFunc("/StorageDriver.Create", func(w http.ResponseWriter, r *http.Request) {
		s.ec[ext].creations++

		var req storageDriverRequest
		if err := decReq(r.Body, &req, w); err != nil {
			return
		}
		if err := driver.Create(req.ID, req.Parent, "", nil); err != nil {
			respond(w, err)
			return
		}
		respond(w, "{}")
	})

	mux.HandleFunc("/StorageDriver.Remove", func(w http.ResponseWriter, r *http.Request) {
		s.ec[ext].removals++

		var req storageDriverRequest
		if err := decReq(r.Body, &req, w); err != nil {
			return
		}

		if err := driver.Remove(req.ID); err != nil {
			respond(w, err)
			return
		}
		respond(w, "{}")
	})

	mux.HandleFunc("/StorageDriver.Get", func(w http.ResponseWriter, r *http.Request) {
		s.ec[ext].gets++

		var req storageDriverRequest
		if err := decReq(r.Body, &req, w); err != nil {
			return
		}

		dir, err := driver.Get(req.ID, req.MountLabel)
		if err != nil {
			respond(w, err)
			return
		}
		respond(w, &storageDriverResponse{Dir: dir})
	})

	mux.HandleFunc("/StorageDriver.Put", func(w http.ResponseWriter, r *http.Request) {
		s.ec[ext].puts++

		var req storageDriverRequest
		if err := decReq(r.Body, &req, w); err != nil {
			return
		}

		if err := driver.Put(req.ID); err != nil {
			respond(w, err)
			return
		}
		respond(w, "{}")
	})

	mux.HandleFunc("/StorageDriver.Exists", func(w http.ResponseWriter, r *http.Request) {
		s.ec[ext].exists++

		var req storageDriverRequest
		if err := decReq(r.Body, &req, w); err != nil {
			return
		}
		respond(w, &storageDriverResponse{Exists: driver.Exists(req.ID)})
	})

	mux.HandleFunc("/StorageDriver.Status", func(w http.ResponseWriter, r *http.Request) {
		s.ec[ext].stats++
		respond(w, &storageDriverResponse{Status: driver.Status()})
	})

	mux.HandleFunc("/StorageDriver.Cleanup", func(w http.ResponseWriter, r *http.Request) {
		s.ec[ext].cleanups++
		err := driver.Cleanup()
		if err != nil {
			respond(w, err)
			return
		}
		respond(w, `{}`)
	})

	mux.HandleFunc("/StorageDriver.GetMetadata", func(w http.ResponseWriter, r *http.Request) {
		s.ec[ext].metadata++

		var req storageDriverRequest
		if err := decReq(r.Body, &req, w); err != nil {
			return
		}

		data, err := driver.GetMetadata(req.ID)
		if err != nil {
			respond(w, err)
			return
		}
		respond(w, &storageDriverResponse{Metadata: data})
	})

	mux.HandleFunc("/StorageDriver.Diff", func(w http.ResponseWriter, r *http.Request) {
		s.ec[ext].diff++

		var req storageDriverRequest
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

	mux.HandleFunc("/StorageDriver.Changes", func(w http.ResponseWriter, r *http.Request) {
		s.ec[ext].changes++
		var req storageDriverRequest
		if err := decReq(r.Body, &req, w); err != nil {
			return
		}

		changes, err := driver.Changes(req.ID, req.Parent)
		if err != nil {
			respond(w, err)
			return
		}
		respond(w, &storageDriverResponse{Changes: changes})
	})

	mux.HandleFunc("/StorageDriver.ApplyDiff", func(w http.ResponseWriter, r *http.Request) {
		s.ec[ext].applydiff++
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
		respond(w, &storageDriverResponse{Size: size})
	})

	mux.HandleFunc("/StorageDriver.DiffSize", func(w http.ResponseWriter, r *http.Request) {
		s.ec[ext].diffsize++

		var req storageDriverRequest
		if err := decReq(r.Body, &req, w); err != nil {
			return
		}

		size, err := driver.DiffSize(req.ID, req.Parent)
		if err != nil {
			respond(w, err)
			return
		}
		respond(w, &storageDriverResponse{Size: size})
	})

	err = os.MkdirAll("/etc/docker/plugins", 0755)
	c.Assert(err, check.IsNil, check.Commentf("error creating /etc/docker/plugins"))

	specFile := "/etc/docker/plugins/" + name + "." + ext
	err = ioutil.WriteFile(specFile, b, 0644)
	c.Assert(err, check.IsNil, check.Commentf("error writing to %s", specFile))
}

func (s *DockerExternalStorageDriverSuite) TearDownSuite(c *check.C) {
	s.server.Close()
	s.jserver.Close()

	err := os.RemoveAll("/etc/docker/plugins")
	c.Assert(err, check.IsNil, check.Commentf("error removing /etc/docker/plugins"))
}

func (s *DockerExternalStorageDriverSuite) TestExternalStorageDriver(c *check.C) {
	s.testExternalStorageDriver("test-external-storage-driver", "spec", c)
	s.testExternalStorageDriver("json-external-storage-driver", "json", c)
}

func (s *DockerExternalStorageDriverSuite) testExternalStorageDriver(name string, ext string, c *check.C) {
	if err := s.d.StartWithBusybox("-s", name); err != nil {
		b, _ := ioutil.ReadFile(s.d.LogFileName())
		c.Assert(err, check.IsNil, check.Commentf("\n%s", string(b)))
	}

	out, err := s.d.Cmd("run", "-d", "--name=storagetest", "busybox", "sh", "-c", "echo hello > /hello")
	c.Assert(err, check.IsNil, check.Commentf(out))

	err = s.d.Restart("-s", name)

	out, err = s.d.Cmd("inspect", "--format='{{.GraphDriver.Name}}'", "storagetest")
	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(strings.TrimSpace(out), check.Equals, name)

	out, err = s.d.Cmd("diff", "storagetest")
	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(strings.Contains(out, "A /hello"), check.Equals, true)

	out, err = s.d.Cmd("rm", "-f", "storagetest")
	c.Assert(err, check.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("info")
	c.Assert(err, check.IsNil, check.Commentf(out))

	err = s.d.Stop()
	c.Assert(err, check.IsNil)

	// Don't check s.ec.exists, because the daemon no longer calls the
	// Exists function.
	c.Assert(s.ec[ext].activations, check.Equals, 2)
	c.Assert(s.ec[ext].init, check.Equals, 2)
	c.Assert(s.ec[ext].creations >= 1, check.Equals, true)
	c.Assert(s.ec[ext].removals >= 1, check.Equals, true)
	c.Assert(s.ec[ext].gets >= 1, check.Equals, true)
	c.Assert(s.ec[ext].puts >= 1, check.Equals, true)
	c.Assert(s.ec[ext].stats, check.Equals, 3)
	c.Assert(s.ec[ext].cleanups, check.Equals, 2)
	c.Assert(s.ec[ext].applydiff >= 1, check.Equals, true)
	c.Assert(s.ec[ext].changes, check.Equals, 1)
	c.Assert(s.ec[ext].diffsize, check.Equals, 0)
	c.Assert(s.ec[ext].diff, check.Equals, 0)
	c.Assert(s.ec[ext].metadata, check.Equals, 1)
}

func (s *DockerExternalStorageDriverSuite) TestExternalStorageDriverPull(c *check.C) {
	testRequires(c, Network)
	c.Assert(s.d.Start(), check.IsNil)

	out, err := s.d.Cmd("pull", "busybox:latest")
	c.Assert(err, check.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("run", "-d", "busybox", "top")
	c.Assert(err, check.IsNil, check.Commentf(out))
}
