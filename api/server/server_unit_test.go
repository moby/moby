package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/version"
)

func TesthttpError(t *testing.T) {
	r := httptest.NewRecorder()

	httpError(r, fmt.Errorf("No such method"))
	if r.Code != http.StatusNotFound {
		t.Fatalf("Expected %d, got %d", http.StatusNotFound, r.Code)
	}

	httpError(r, fmt.Errorf("This accound hasn't been activated"))
	if r.Code != http.StatusForbidden {
		t.Fatalf("Expected %d, got %d", http.StatusForbidden, r.Code)
	}

	httpError(r, fmt.Errorf("Some error"))
	if r.Code != http.StatusInternalServerError {
		t.Fatalf("Expected %d, got %d", http.StatusInternalServerError, r.Code)
	}
}

func TestGetInfo(t *testing.T) {
	eng := engine.New()
	var called bool
	eng.Register("info", func(job *engine.Job) error {
		called = true
		v := &engine.Env{}
		v.SetInt("Containers", 1)
		v.SetInt("Images", 42000)
		if _, err := v.WriteTo(job.Stdout); err != nil {
			return err
		}
		return nil
	})
	r := serveRequest("GET", "/info", nil, eng, t)
	if !called {
		t.Fatalf("handler was not called")
	}
	v := readEnv(r.Body, t)
	if v.GetInt("Images") != 42000 {
		t.Fatalf("%#v\n", v)
	}
	if v.GetInt("Containers") != 1 {
		t.Fatalf("%#v\n", v)
	}
	assertContentType(r, "application/json", t)
}

func TestGetContainersByName(t *testing.T) {
	eng := engine.New()
	name := "container_name"
	var called bool
	eng.Register("container_inspect", func(job *engine.Job) error {
		called = true
		if job.Args[0] != name {
			t.Errorf("name != '%s': %#v", name, job.Args[0])
		}
		if api.APIVERSION.LessThan("1.12") && !job.GetenvBool("dirty") {
			t.Errorf("dirty env variable not set")
		} else if api.APIVERSION.GreaterThanOrEqualTo("1.12") && job.GetenvBool("dirty") {
			t.Errorf("dirty env variable set when it shouldn't")
		}
		v := &engine.Env{}
		v.SetBool("dirty", true)
		if _, err := v.WriteTo(job.Stdout); err != nil {
			return err
		}
		return nil
	})
	r := serveRequest("GET", "/containers/"+name+"/json", nil, eng, t)
	if !called {
		t.Fatal("handler was not called")
	}
	assertContentType(r, "application/json", t)
	var stdoutJson interface{}
	if err := json.Unmarshal(r.Body.Bytes(), &stdoutJson); err != nil {
		t.Fatalf("%#v", err)
	}
	if stdoutJson.(map[string]interface{})["dirty"].(float64) != 1 {
		t.Fatalf("%#v", stdoutJson)
	}
}

func TestGetImagesByName(t *testing.T) {
	eng := engine.New()
	name := "image_name"
	var called bool
	eng.Register("image_inspect", func(job *engine.Job) error {
		called = true
		if job.Args[0] != name {
			t.Fatalf("name != '%s': %#v", name, job.Args[0])
		}
		if api.APIVERSION.LessThan("1.12") && !job.GetenvBool("dirty") {
			t.Fatal("dirty env variable not set")
		} else if api.APIVERSION.GreaterThanOrEqualTo("1.12") && job.GetenvBool("dirty") {
			t.Fatal("dirty env variable set when it shouldn't")
		}
		v := &engine.Env{}
		v.SetBool("dirty", true)
		if _, err := v.WriteTo(job.Stdout); err != nil {
			return err
		}
		return nil
	})
	r := serveRequest("GET", "/images/"+name+"/json", nil, eng, t)
	if !called {
		t.Fatal("handler was not called")
	}
	if r.HeaderMap.Get("Content-Type") != "application/json" {
		t.Fatalf("%#v\n", r)
	}
	var stdoutJson interface{}
	if err := json.Unmarshal(r.Body.Bytes(), &stdoutJson); err != nil {
		t.Fatalf("%#v", err)
	}
	if stdoutJson.(map[string]interface{})["dirty"].(float64) != 1 {
		t.Fatalf("%#v", stdoutJson)
	}
}

func serveRequest(method, target string, body io.Reader, eng *engine.Engine, t *testing.T) *httptest.ResponseRecorder {
	return serveRequestUsingVersion(method, target, api.APIVERSION, body, eng, t)
}

func serveRequestUsingVersion(method, target string, version version.Version, body io.Reader, eng *engine.Engine, t *testing.T) *httptest.ResponseRecorder {
	r := httptest.NewRecorder()
	req, err := http.NewRequest(method, target, body)
	if err != nil {
		t.Fatal(err)
	}
	ServeRequest(eng, version, r, req)
	return r
}

func readEnv(src io.Reader, t *testing.T) *engine.Env {
	out := engine.NewOutput()
	v, err := out.AddEnv()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, src); err != nil {
		t.Fatal(err)
	}
	out.Close()
	return v
}

func toJson(data interface{}, t *testing.T) io.Reader {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(data); err != nil {
		t.Fatal(err)
	}
	return &buf
}

func assertContentType(recorder *httptest.ResponseRecorder, contentType string, t *testing.T) {
	if recorder.HeaderMap.Get("Content-Type") != contentType {
		t.Fatalf("%#v\n", recorder)
	}
}

// XXX: Duplicated from integration/utils_test.go, but maybe that's OK as that
// should die as soon as we converted all integration tests?
// assertHttpNotError expect the given response to not have an error.
// Otherwise the it causes the test to fail.
func assertHttpNotError(r *httptest.ResponseRecorder, t *testing.T) {
	// Non-error http status are [200, 400)
	if r.Code < http.StatusOK || r.Code >= http.StatusBadRequest {
		t.Fatal(fmt.Errorf("Unexpected http error: %v", r.Code))
	}
}

func createEnvFromGetImagesJSONStruct(data getImagesJSONStruct) types.Image {
	return types.Image{
		RepoTags:    data.RepoTags,
		ID:          data.Id,
		Created:     int(data.Created),
		Size:        int(data.Size),
		VirtualSize: int(data.VirtualSize),
	}
}

type getImagesJSONStruct struct {
	RepoTags    []string
	Id          string
	Created     int64
	Size        int64
	VirtualSize int64
}

var sampleImage getImagesJSONStruct = getImagesJSONStruct{
	RepoTags:    []string{"test-name:test-tag"},
	Id:          "ID",
	Created:     999,
	Size:        777,
	VirtualSize: 666,
}
