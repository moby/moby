package registry // import "github.com/docker/docker/registry"

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/containerd/log"
	"github.com/docker/docker/api/types/registry"
	"gotest.tools/v3/assert"
)

var (
	testHTTPServer  *httptest.Server
	testHTTPSServer *httptest.Server
)

func init() {
	r := http.NewServeMux()

	// /v1/
	r.HandleFunc("/v1/_ping", handlerGetPing)
	r.HandleFunc("/v1/search", handlerSearch)

	// /v2/
	r.HandleFunc("/v2/version", handlerGetPing)

	testHTTPServer = httptest.NewServer(handlerAccessLog(r))
	testHTTPSServer = httptest.NewTLSServer(handlerAccessLog(r))
}

func handlerAccessLog(handler http.Handler) http.Handler {
	logHandler := func(w http.ResponseWriter, r *http.Request) {
		log.G(context.TODO()).Debugf(`%s "%s %s"`, r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	}
	return http.HandlerFunc(logHandler)
}

func makeURL(req string) string {
	return testHTTPServer.URL + req
}

func makeHTTPSURL(req string) string {
	return testHTTPSServer.URL + req
}

func makeIndex(req string) *registry.IndexInfo {
	return &registry.IndexInfo{
		Name: makeURL(req),
	}
}

func makeHTTPSIndex(req string) *registry.IndexInfo {
	return &registry.IndexInfo{
		Name: makeHTTPSURL(req),
	}
}

func makePublicIndex() *registry.IndexInfo {
	return &registry.IndexInfo{
		Name:     IndexServer,
		Secure:   true,
		Official: true,
	}
}

func makeServiceConfig(mirrors []string, insecureRegistries []string) (*serviceConfig, error) {
	return newServiceConfig(ServiceOptions{
		Mirrors:            mirrors,
		InsecureRegistries: insecureRegistries,
	})
}

func writeHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Add("Server", "docker-tests/mock")
	h.Add("Expires", "-1")
	h.Add("Content-Type", "application/json")
	h.Add("Pragma", "no-cache")
	h.Add("Cache-Control", "no-cache")
}

func writeResponse(w http.ResponseWriter, message interface{}, code int) {
	writeHeaders(w)
	w.WriteHeader(code)
	body, err := json.Marshal(message)
	if err != nil {
		_, _ = io.WriteString(w, err.Error())
		return
	}
	_, _ = w.Write(body)
}

func handlerGetPing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeResponse(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	writeResponse(w, true, http.StatusOK)
}

func handlerSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeResponse(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	result := &registry.SearchResults{
		Query:      "fakequery",
		NumResults: 1,
		Results:    []registry.SearchResult{{Name: "fakeimage", StarCount: 42}},
	}
	writeResponse(w, result, http.StatusOK)
}

func TestPing(t *testing.T) {
	res, err := http.Get(makeURL("/v1/_ping"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, res.StatusCode, http.StatusOK, "")
	assert.Equal(t, res.Header.Get("Server"), "docker-tests/mock")
}
