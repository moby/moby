package plugins // import "github.com/docker/docker/pkg/plugins"

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/pkg/plugins/transport"
	"github.com/docker/go-connections/tlsconfig"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func setupRemotePluginServer(t *testing.T) (mux *http.ServeMux, addr string) {
	t.Helper()
	mux = http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Logf("started remote plugin server listening on: %s", server.URL)
	t.Cleanup(func() {
		server.Close()
	})
	return mux, server.URL
}

func TestFailedConnection(t *testing.T) {
	c, _ := NewClient("tcp://127.0.0.1:1", &tlsconfig.Options{InsecureSkipVerify: true})
	_, err := c.callWithRetry("Service.Method", nil, false)
	if err == nil {
		t.Fatal("Unexpected successful connection")
	}
}

func TestFailOnce(t *testing.T) {
	mux, addr := setupRemotePluginServer(t)

	failed := false
	mux.HandleFunc("/Test.FailOnce", func(w http.ResponseWriter, r *http.Request) {
		if !failed {
			failed = true
			panic("Plugin not ready")
		}
	})

	c, _ := NewClient(addr, &tlsconfig.Options{InsecureSkipVerify: true})
	b := strings.NewReader("body")
	_, err := c.callWithRetry("Test.FailOnce", b, true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEchoInputOutput(t *testing.T) {
	mux, addr := setupRemotePluginServer(t)

	m := Manifest{[]string{"VolumeDriver", "NetworkDriver"}}

	mux.HandleFunc("/Test.Echo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("Expected POST, got %s\n", r.Method)
		}

		header := w.Header()
		header.Set("Content-Type", transport.VersionMimetype)

		io.Copy(w, r.Body)
	})

	c, _ := NewClient(addr, &tlsconfig.Options{InsecureSkipVerify: true})
	var output Manifest
	err := c.Call("Test.Echo", m, &output)
	if err != nil {
		t.Fatal(err)
	}

	assert.Check(t, is.DeepEqual(m, output))
	err = c.Call("Test.Echo", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBackoff(t *testing.T) {
	cases := []struct {
		retries    int
		expTimeOff time.Duration
	}{
		{0, time.Duration(1)},
		{1, time.Duration(2)},
		{2, time.Duration(4)},
		{4, time.Duration(16)},
		{6, time.Duration(30)},
		{10, time.Duration(30)},
	}

	for _, c := range cases {
		s := c.expTimeOff * time.Second
		if d := backoff(c.retries); d != s {
			t.Fatalf("Retry %v, expected %v, was %v\n", c.retries, s, d)
		}
	}
}

func TestAbortRetry(t *testing.T) {
	cases := []struct {
		timeOff  time.Duration
		expAbort bool
	}{
		{time.Duration(1), false},
		{time.Duration(2), false},
		{time.Duration(10), false},
		{time.Duration(30), true},
		{time.Duration(40), true},
	}

	for _, c := range cases {
		s := c.timeOff * time.Second
		if a := abort(time.Now(), s); a != c.expAbort {
			t.Fatalf("Duration %v, expected %v, was %v\n", c.timeOff, s, a)
		}
	}
}

func TestClientScheme(t *testing.T) {
	cases := map[string]string{
		"tcp://127.0.0.1:8080":          "http",
		"unix:///usr/local/plugins/foo": "http",
		"http://127.0.0.1:8080":         "http",
		"https://127.0.0.1:8080":        "https",
	}

	for addr, scheme := range cases {
		u, err := url.Parse(addr)
		if err != nil {
			t.Fatal(err)
		}
		s := httpScheme(u)

		if s != scheme {
			t.Fatalf("URL scheme mismatch, expected %s, got %s", scheme, s)
		}
	}
}

func TestNewClientWithTimeout(t *testing.T) {
	mux, addr := setupRemotePluginServer(t)

	m := Manifest{[]string{"VolumeDriver", "NetworkDriver"}}

	mux.HandleFunc("/Test.Echo", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Duration(600) * time.Millisecond)
		io.Copy(w, r.Body)
	})

	// setting timeout of 500ms
	timeout := time.Duration(500) * time.Millisecond
	c, _ := NewClientWithTimeout(addr, &tlsconfig.Options{InsecureSkipVerify: true}, timeout)
	var output Manifest
	err := c.Call("Test.Echo", m, &output)
	if err == nil {
		t.Fatal("Expected timeout error")
	}
}

func TestClientStream(t *testing.T) {
	mux, addr := setupRemotePluginServer(t)

	m := Manifest{[]string{"VolumeDriver", "NetworkDriver"}}
	var output Manifest

	mux.HandleFunc("/Test.Echo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("Expected POST, got %s", r.Method)
		}

		header := w.Header()
		header.Set("Content-Type", transport.VersionMimetype)

		io.Copy(w, r.Body)
	})

	c, _ := NewClient(addr, &tlsconfig.Options{InsecureSkipVerify: true})
	body, err := c.Stream("Test.Echo", m)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	if err := json.NewDecoder(body).Decode(&output); err != nil {
		t.Fatalf("Test.Echo: error reading plugin resp: %v", err)
	}
	assert.Check(t, is.DeepEqual(m, output))
}

func TestClientSendFile(t *testing.T) {
	mux, addr := setupRemotePluginServer(t)

	m := Manifest{[]string{"VolumeDriver", "NetworkDriver"}}
	var output Manifest
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(m); err != nil {
		t.Fatal(err)
	}
	mux.HandleFunc("/Test.Echo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("Expected POST, got %s\n", r.Method)
		}

		header := w.Header()
		header.Set("Content-Type", transport.VersionMimetype)

		io.Copy(w, r.Body)
	})

	c, _ := NewClient(addr, &tlsconfig.Options{InsecureSkipVerify: true})
	if err := c.SendFile("Test.Echo", &buf, &output); err != nil {
		t.Fatal(err)
	}
	assert.Check(t, is.DeepEqual(m, output))
}

func TestClientWithRequestTimeout(t *testing.T) {
	type timeoutError interface {
		Timeout() bool
	}

	unblock := make(chan struct{})
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-unblock:
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusOK)
	}

	srv := httptest.NewServer(http.HandlerFunc(testHandler))
	defer func() {
		close(unblock)
		srv.Close()
	}()

	client := &Client{http: srv.Client(), requestFactory: &testRequestWrapper{srv}}
	errCh := make(chan error, 1)
	go func() {
		_, err := client.callWithRetry("/Plugin.Hello", nil, false, WithRequestTimeout(time.Millisecond))
		errCh <- err
	}()

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case err := <-errCh:
		var tErr timeoutError
		if assert.Check(t, errors.As(err, &tErr), "want timeout error, got %T", err) {
			assert.Check(t, tErr.Timeout())
		}
	case <-timer.C:
		t.Fatal("client request did not time out in time")
	}
}

type testRequestWrapper struct {
	*httptest.Server
}

func (w *testRequestWrapper) NewRequest(path string, data io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodPost, path, data)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(w.Server.URL)
	if err != nil {
		return nil, err
	}
	req.URL = u
	return req, nil
}
