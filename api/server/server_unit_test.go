package server

import (
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
	tmp, err := utils.TestDirectory("")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	eng, err := engine.New(tmp)
	if err != nil {
		t.Fatal(err)
	}
	var called bool
	eng.Register("version", func(job *engine.Job) engine.Status {
		called = true
		v := &engine.Env{}
		v.Set("Version", "42.1")
		v.Set("ApiVersion", "1.1.1.1.1")
		v.Set("GoVersion", "2.42")
		v.Set("Os", "Linux")
		v.Set("Arch", "x86_64")
		if _, err := v.WriteTo(job.Stdout); err != nil {
			return job.Error(err)
		}
		return engine.StatusOK
	})

	r := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/version", nil)
	if err != nil {
		t.Fatal(err)
	}
	// FIXME getting the version should require an actual running Server
	if err := ServeRequest(eng, api.APIVERSION, r, req); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatalf("handler was not called")
	}
	out := engine.NewOutput()
	v, err := out.AddEnv()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, r.Body); err != nil {
		t.Fatal(err)
	}
	out.Close()
	expected := "42.1"
	if result := v.Get("Version"); result != expected {
		t.Errorf("Expected version %s, %s found", expected, result)
	}
	expected = "application/json"
	if result := r.HeaderMap.Get("Content-Type"); result != expected {
		t.Errorf("Expected Content-Type %s, %s found", expected, result)
	}
}
