package transport // import "github.com/docker/docker/pkg/plugins/transport"

import (
	"io"
	"net/http"
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestHTTPTransport(t *testing.T) {
	var r io.Reader
	roundTripper := &http.Transport{}
	newTransport := NewHTTPTransport(roundTripper, "http", "0.0.0.0")
	request, err := newTransport.NewRequest("", r)
	if err != nil {
		t.Fatal(err)
	}
	assert.Check(t, is.Equal("POST", request.Method))
}
