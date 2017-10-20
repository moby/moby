package authorization

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewResponseModifier(t *testing.T) {
	recorder := httptest.NewRecorder()
	modifier := NewResponseModifier(recorder)
	modifier.Header().Set("H1", "V1")
	modifier.Write([]byte("body"))
	require.False(t, modifier.Hijacked())
	modifier.WriteHeader(http.StatusInternalServerError)
	require.NotNil(t, modifier.RawBody())

	raw, err := modifier.RawHeaders()
	require.NotNil(t, raw)
	require.Nil(t, err)

	headerData := strings.Split(strings.TrimSpace(string(raw)), ":")
	require.EqualValues(t, "H1", strings.TrimSpace(headerData[0]))
	require.EqualValues(t, "V1", strings.TrimSpace(headerData[1]))

	modifier.Flush()
	modifier.FlushAll()

	if recorder.Header().Get("H1") != "V1" {
		t.Fatalf("Header value must exists %s", recorder.Header().Get("H1"))
	}

}
