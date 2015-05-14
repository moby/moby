package plugins

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

var (
	mux    *http.ServeMux
	server *httptest.Server
)

func setupRemotePluginServer() string {
	mux = http.NewServeMux()
	server = httptest.NewServer(mux)
	return server.URL
}

func teardownRemotePluginServer() {
	if server != nil {
		server.Close()
	}
}

func TestFailedConnection(t *testing.T) {
	c := NewClient("tcp://127.0.0.1:1")
	err := c.Call("Service.Method", nil, nil)
	if err == nil {
		t.Fatal("Unexpected successful connection")
	}
}

func TestEchoInputOutput(t *testing.T) {
	addr := setupRemotePluginServer()
	defer teardownRemotePluginServer()

	m := Manifest{[]string{"VolumeDriver", "NetworkDriver"}}

	mux.HandleFunc("/Test.Echo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("Expected POST, got %s\n", r.Method)
		}

		header := w.Header()
		header.Set("Content-Type", versionMimetype)

		io.Copy(w, r.Body)
	})

	c := NewClient(addr)
	var output Manifest
	err := c.Call("Test.Echo", m, &output)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(output, m) {
		t.Fatalf("Expected %v, was %v\n", m, output)
	}
}
