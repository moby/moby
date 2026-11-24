package daemonapiv0_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	daemonapiv0 "github.com/moby/moby/extpoints/daemonapi/v0"
)

type fakeAPI struct {
	host  string
	calls atomic.Int32
}

func (f *fakeAPI) Get(context.Context, *daemonapiv0.GetRequest) (*daemonapiv0.GetResponse, error) {
	f.calls.Add(1)
	return &daemonapiv0.GetResponse{Host: f.host}, nil
}

func TestDialContextGetsEndpointOnFirstConnect(t *testing.T) {
	t.Parallel()

	// t.TempDir paths can exceed the unix socket path limit on macOS.
	dir, err := os.MkdirTemp("", "daemonapi")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	sock := filepath.Join(dir, "api.sock")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Force a new connection per request so the second request dials again.
		w.Header().Set("Connection", "close")
	}))
	_ = srv.Listener.Close()
	srv.Listener = l
	srv.Start()
	t.Cleanup(srv.Close)

	api := &fakeAPI{host: "unix://" + sock}
	tr := &http.Transport{DialContext: daemonapiv0.DialContext(api)}
	t.Cleanup(tr.CloseIdleConnections)
	c := &http.Client{Transport: tr}
	if n := api.calls.Load(); n != 0 {
		t.Errorf("Get called %d times before connecting, want 0", n)
	}

	for range 2 {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://daemonapi/_ping", http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	}
	if n := api.calls.Load(); n != 1 {
		t.Errorf("Get called %d times, want 1", n)
	}
}
