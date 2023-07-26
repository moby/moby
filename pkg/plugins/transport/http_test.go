package transport // import "github.com/docker/docker/pkg/plugins/transport"

import (
	"io"
	"net/http"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestHTTPTransport(t *testing.T) {
	var r io.Reader
	newTransport := NewHTTPTransport(&http.Transport{}, "http", "0.0.0.0")
	request, err := newTransport.NewRequest("", r)
	if err != nil {
		t.Fatal(err)
	}
	assert.Check(t, is.Equal(http.MethodPost, request.Method))
}
