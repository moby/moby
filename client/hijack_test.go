package client

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"github.com/gotestyourself/gotestyourself/assert"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

func TestTLSCloseWriter(t *testing.T) {
	t.Parallel()

	var chErr chan error
	ts := &httptest.Server{Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		chErr = make(chan error, 1)
		defer close(chErr)
		if err := httputils.ParseForm(req); err != nil {
			chErr <- errors.Wrap(err, "error parsing form")
			http.Error(w, err.Error(), 500)
			return
		}
		r, rw, err := httputils.HijackConnection(w)
		if err != nil {
			chErr <- errors.Wrap(err, "error hijacking connection")
			http.Error(w, err.Error(), 500)
			return
		}
		defer r.Close()

		fmt.Fprint(rw, "HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.raw-stream\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\n")

		buf := make([]byte, 5)
		_, err = r.Read(buf)
		if err != nil {
			chErr <- errors.Wrap(err, "error reading from client")
			return
		}
		_, err = rw.Write(buf)
		if err != nil {
			chErr <- errors.Wrap(err, "error writing to client")
			return
		}
	})}}

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
	assert.Assert(t, err)

	ts.Listener = l
	defer l.Close()

	defer func() {
		if chErr != nil {
			assert.Assert(t, <-chErr)
		}
	}()

	ts.StartTLS()
	defer ts.Close()

	serverURL, err := url.Parse(ts.URL)
	assert.Assert(t, err)

	client, err := NewClient("tcp://"+serverURL.Host, "", ts.Client(), nil)
	assert.Assert(t, err)

	resp, err := client.postHijacked(context.Background(), "/asdf", url.Values{}, nil, map[string][]string{"Content-Type": {"text/plain"}})
	assert.Assert(t, err)
	defer resp.Close()

	if _, ok := resp.Conn.(types.CloseWriter); !ok {
		t.Fatal("tls conn did not implement the CloseWrite interface")
	}

	_, err = resp.Conn.Write([]byte("hello"))
	assert.Assert(t, err)

	b, err := ioutil.ReadAll(resp.Reader)
	assert.Assert(t, err)
	assert.Assert(t, string(b) == "hello")
	assert.Assert(t, resp.CloseWrite())

	// This should error since writes are closed
	_, err = resp.Conn.Write([]byte("no"))
	assert.Assert(t, err != nil)
}
