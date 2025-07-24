package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestTLSCloseWriter(t *testing.T) {
	t.Parallel()

	var chErr chan error
	ts := &httptest.Server{Config: &http.Server{
		ReadHeaderTimeout: 5 * time.Minute, // "G112: Potential Slowloris Attack (gosec)"; not a real concern for our use, so setting a long timeout.
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			chErr = make(chan error, 1)
			defer close(chErr)

			if err := req.ParseForm(); err != nil && !strings.HasPrefix(err.Error(), "mime:") {
				chErr <- fmt.Errorf("error parsing form: %w", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				chErr <- fmt.Errorf("error hijacking connection: %w", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer conn.Close()

			// Flush the options to make sure the client sets the raw mode
			_, _ = conn.Write([]byte{})

			fmt.Fprint(conn, "HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.raw-stream\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\n")

			buf := make([]byte, 5)
			_, err = conn.Read(buf)
			if err != nil {
				chErr <- fmt.Errorf("error reading from client: %w", err)
				return
			}
			_, err = conn.Write(buf)
			if err != nil {
				chErr <- fmt.Errorf("error writing to client: %w", err)
				return
			}
		}),
	}}

	var (
		l   net.Listener
		err error
	)
	for i := 1024; i < 10000; i++ {
		l, err = net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", i))
		if err == nil {
			break
		}
	}
	assert.NilError(t, err)

	ts.Listener = l
	defer l.Close()

	defer func() {
		if chErr != nil {
			assert.NilError(t, <-chErr)
		}
	}()

	ts.StartTLS()
	defer ts.Close()

	serverURL, err := url.Parse(ts.URL)
	assert.NilError(t, err)

	client, err := NewClientWithOpts(WithHost("tcp://"+serverURL.Host), WithHTTPClient(ts.Client()))
	assert.NilError(t, err)

	resp, err := client.postHijacked(context.Background(), "/asdf", url.Values{}, nil, map[string][]string{"Content-Type": {"text/plain"}})
	assert.NilError(t, err)
	defer resp.Close()

	_, ok := resp.Conn.(CloseWriter)
	assert.Check(t, ok, "tls conn did not implement the CloseWrite interface")

	_, err = resp.Conn.Write([]byte("hello"))
	assert.NilError(t, err)

	b, err := io.ReadAll(resp.Reader)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(string(b), "hello"))
	assert.NilError(t, resp.CloseWrite())

	// This should error since writes are closed
	_, err = resp.Conn.Write([]byte("no"))
	assert.Check(t, err != nil)
}
