package registry // import "github.com/docker/docker/registry"

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/api/types/registry"
	"github.com/sirupsen/logrus"
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

	// override net.LookupIP
	lookupIP = func(host string) ([]net.IP, error) {
		if host == "127.0.0.1" {
			// I believe in future Go versions this will fail, so let's fix it later
			return net.LookupIP(host)
		}
		mockHosts := map[string][]net.IP{
			"":            {net.ParseIP("0.0.0.0")},
			"localhost":   {net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
			"example.com": {net.ParseIP("42.42.42.42")},
			"other.com":   {net.ParseIP("43.43.43.43")},
		}
		for h, addrs := range mockHosts {
			if host == h {
				return addrs, nil
			}
			for _, addr := range addrs {
				if addr.String() == host {
					return []net.IP{addr}, nil
				}
			}
		}
		return nil, errors.New("lookup: no such host")
	}
}

func handlerAccessLog(handler http.Handler) http.Handler {
	logHandler := func(w http.ResponseWriter, r *http.Request) {
		logrus.Debugf(`%s "%s %s"`, r.RemoteAddr, r.Method, r.URL)
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
	index := &registry.IndexInfo{
		Name: makeURL(req),
	}
	return index
}

func makeHTTPSIndex(req string) *registry.IndexInfo {
	index := &registry.IndexInfo{
		Name: makeHTTPSURL(req),
	}
	return index
}

func makePublicIndex() *registry.IndexInfo {
	index := &registry.IndexInfo{
		Name:     IndexServer,
		Secure:   true,
		Official: true,
	}
	return index
}

func makeServiceConfig(mirrors []string, insecureRegistries []string) (*serviceConfig, error) {
	options := ServiceOptions{
		Mirrors:            mirrors,
		InsecureRegistries: insecureRegistries,
	}

	return newServiceConfig(options)
}

func writeHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Add("Server", "docker-tests/mock")
	h.Add("Expires", "-1")
	h.Add("Content-Type", "application/json")
	h.Add("Pragma", "no-cache")
	h.Add("Cache-Control", "no-cache")
	h.Add("X-Docker-Registry-Version", "0.0.0")
	h.Add("X-Docker-Registry-Config", "mock")
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
	assert.Equal(t, res.Header.Get("X-Docker-Registry-Config"), "mock", "This is not a Mocked Registry")
}
