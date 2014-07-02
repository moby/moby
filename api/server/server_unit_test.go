package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dotcloud/docker/api"
	"github.com/dotcloud/docker/engine"
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
	eng := engine.New()
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
	eng := engine.New()
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

func TestGetContainersByName(t *testing.T) {
	eng := engine.New()
	name := "container_name"
	var called bool
	eng.Register("container_inspect", func(job *engine.Job) engine.Status {
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
			return job.Error(err)
		}
		return engine.StatusOK
	})
	r := serveRequest("GET", "/containers/"+name+"/json", nil, eng, t)
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

func TestGetEvents(t *testing.T) {
	eng := engine.New()
	var called bool
	eng.Register("events", func(job *engine.Job) engine.Status {
		called = true
		since := job.Getenv("since")
		if since != "1" {
			t.Fatalf("'since' should be 1, found %#v instead", since)
		}
		until := job.Getenv("until")
		if until != "0" {
			t.Fatalf("'until' should be 0, found %#v instead", until)
		}
		v := &engine.Env{}
		v.Set("since", since)
		v.Set("until", until)
		if _, err := v.WriteTo(job.Stdout); err != nil {
			return job.Error(err)
		}
		return engine.StatusOK
	})
	r := serveRequest("GET", "/events?since=1&until=0", nil, eng, t)
	if !called {
		t.Fatal("handler was not called")
	}
	if r.HeaderMap.Get("Content-Type") != "application/json" {
		t.Fatalf("%#v\n", r)
	}
	var stdout_json struct {
		Since int
		Until int
	}
	if err := json.Unmarshal(r.Body.Bytes(), &stdout_json); err != nil {
		t.Fatalf("%#v", err)
	}
	if stdout_json.Since != 1 {
		t.Fatalf("since != 1: %#v", stdout_json.Since)
	}
	if stdout_json.Until != 0 {
		t.Fatalf("until != 0: %#v", stdout_json.Until)
	}
}

func TestLogs(t *testing.T) {
	eng := engine.New()
	var inspect bool
	var logs bool
	eng.Register("container_inspect", func(job *engine.Job) engine.Status {
		inspect = true
		if len(job.Args) == 0 {
			t.Fatal("Job arguments is empty")
		}
		if job.Args[0] != "test" {
			t.Fatalf("Container name %s, must be test", job.Args[0])
		}
		return engine.StatusOK
	})
	expected := "logs"
	eng.Register("logs", func(job *engine.Job) engine.Status {
		logs = true
		if len(job.Args) == 0 {
			t.Fatal("Job arguments is empty")
		}
		if job.Args[0] != "test" {
			t.Fatalf("Container name %s, must be test", job.Args[0])
		}
		follow := job.Getenv("follow")
		if follow != "1" {
			t.Fatalf("follow: %s, must be 1", follow)
		}
		stdout := job.Getenv("stdout")
		if stdout != "1" {
			t.Fatalf("stdout %s, must be 1", stdout)
		}
		stderr := job.Getenv("stderr")
		if stderr != "" {
			t.Fatalf("stderr %s, must be empty", stderr)
		}
		timestamps := job.Getenv("timestamps")
		if timestamps != "1" {
			t.Fatalf("timestamps %s, must be 1", timestamps)
		}
		job.Stdout.Write([]byte(expected))
		return engine.StatusOK
	})
	r := serveRequest("GET", "/containers/test/logs?follow=1&stdout=1&timestamps=1", nil, eng, t)
	if r.Code != http.StatusOK {
		t.Fatalf("Got status %d, expected %d", r.Code, http.StatusOK)
	}
	if !inspect {
		t.Fatal("container_inspect job was not called")
	}
	if !logs {
		t.Fatal("logs job was not called")
	}
	res := r.Body.String()
	if res != expected {
		t.Fatalf("Output %s, expected %s", res, expected)
	}
}

func TestLogsNoStreams(t *testing.T) {
	eng := engine.New()
	var inspect bool
	var logs bool
	eng.Register("container_inspect", func(job *engine.Job) engine.Status {
		inspect = true
		if len(job.Args) == 0 {
			t.Fatal("Job arguments is empty")
		}
		if job.Args[0] != "test" {
			t.Fatalf("Container name %s, must be test", job.Args[0])
		}
		return engine.StatusOK
	})
	eng.Register("logs", func(job *engine.Job) engine.Status {
		logs = true
		return engine.StatusOK
	})
	r := serveRequest("GET", "/containers/test/logs", nil, eng, t)
	if r.Code != http.StatusBadRequest {
		t.Fatalf("Got status %d, expected %d", r.Code, http.StatusBadRequest)
	}
	if inspect {
		t.Fatal("container_inspect job was called, but it shouldn't")
	}
	if logs {
		t.Fatal("logs job was called, but it shouldn't")
	}
	res := strings.TrimSpace(r.Body.String())
	expected := "Bad parameters: you must choose at least one stream"
	if !strings.Contains(res, expected) {
		t.Fatalf("Output %s, expected %s in it", res, expected)
	}
}

func TestGetImagesHistory(t *testing.T) {
	eng := engine.New()
	imageName := "docker-test-image"
	var called bool
	eng.Register("history", func(job *engine.Job) engine.Status {
		called = true
		if len(job.Args) == 0 {
			t.Fatal("Job arguments is empty")
		}
		if job.Args[0] != imageName {
			t.Fatalf("name != '%s': %#v", imageName, job.Args[0])
		}
		v := &engine.Env{}
		if _, err := v.WriteTo(job.Stdout); err != nil {
			return job.Error(err)
		}
		return engine.StatusOK
	})
	r := serveRequest("GET", "/images/"+imageName+"/history", nil, eng, t)
	if !called {
		t.Fatalf("handler was not called")
	}
	if r.Code != http.StatusOK {
		t.Fatalf("Got status %d, expected %d", r.Code, http.StatusOK)
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
