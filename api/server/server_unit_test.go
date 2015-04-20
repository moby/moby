package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/api"
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

func assertContentType(recorder *httptest.ResponseRecorder, contentType string, t *testing.T) {
	if recorder.HeaderMap.Get("Content-Type") != contentType {
		t.Fatalf("%#v\n", recorder)
	}
}
