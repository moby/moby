package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/api"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/utils"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestGetBoolParam(t *testing.T) {
	if ret, err := getBoolParam("true"); err != nil || !ret {
		t.Fatalf("true -> true, nil | got %t %s", ret, err)
	}
	if ret, err := getBoolParam("True"); err != nil || !ret {
		t.Fatalf("True -> true, nil | got %t %s", ret, err)
	}
	if ret, err := getBoolParam("1"); err != nil || !ret {
		t.Fatalf("1 -> true, nil | got %t %s", ret, err)
	}
	if ret, err := getBoolParam(""); err != nil || ret {
		t.Fatalf("\"\" -> false, nil | got %t %s", ret, err)
	}
	if ret, err := getBoolParam("false"); err != nil || ret {
		t.Fatalf("false -> false, nil | got %t %s", ret, err)
	}
	if ret, err := getBoolParam("0"); err != nil || ret {
		t.Fatalf("0 -> false, nil | got %t %s", ret, err)
	}
	if ret, err := getBoolParam("faux"); err == nil || ret {
		t.Fatalf("faux -> false, err | got %t %s", ret, err)

	}
}

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

func TestGetVersion(t *testing.T) {
	eng := tmpEngine(t)
	defer rmEngine(eng)
	var called bool
	eng.Register("version", func(job *engine.Job) engine.Status {
		called = true
		v := &engine.Env{}
		v.SetJson("Version", "42.1")
		v.Set("ApiVersion", "1.1.1.1.1")
		v.Set("GoVersion", "2.42")
		v.Set("Os", "Linux")
		v.Set("Arch", "x86_64")
		if _, err := v.WriteTo(job.Stdout); err != nil {
			return job.Error(err)
		}
		return engine.StatusOK
	})
	r := serveRequest("GET", "/version", nil, eng, t)
	if !called {
		t.Fatalf("handler was not called")
	}
	v := readEnv(r.Body, t)
	if v.Get("Version") != "42.1" {
		t.Fatalf("%#v\n", v)
	}
	if r.HeaderMap.Get("Content-Type") != "application/json" {
		t.Fatalf("%#v\n", r)
	}
}

func TestGetInfo(t *testing.T) {
	eng := tmpEngine(t)
	defer rmEngine(eng)
	var called bool
	eng.Register("info", func(job *engine.Job) engine.Status {
		called = true
		v := &engine.Env{}
		v.SetInt("Containers", 1)
		v.SetInt("Images", 42000)
		if _, err := v.WriteTo(job.Stdout); err != nil {
			return job.Error(err)
		}
		return engine.StatusOK
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
	if r.HeaderMap.Get("Content-Type") != "application/json" {
		t.Fatalf("%#v\n", r)
	}
}

func serveRequest(method, target string, body io.Reader, eng *engine.Engine, t *testing.T) *httptest.ResponseRecorder {
	r := httptest.NewRecorder()
	req, err := http.NewRequest(method, target, body)
	if err != nil {
		t.Fatal(err)
	}
	if err := ServeRequest(eng, api.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
	return r
}

func tmpEngine(t *testing.T) *engine.Engine {
	tmp, err := utils.TestDirectory("")
	if err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(tmp)
	if err != nil {
		t.Fatal(err)
	}
	return eng
}

func rmEngine(eng *engine.Engine) {
	os.RemoveAll(eng.Root())
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
