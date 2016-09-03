// +build !windows

// TODO Windows: This uses a Unix socket for testing. This might be possible
// to port to Windows using a named pipe instead.

package authorization

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/go-check/check"
	"github.com/gorilla/mux"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

const (
	pluginAddress = "authz-test-plugin.sock"
)

func (s *DockerSuite) TestAuthZRequestPluginError(c *check.C) {
	server := authZPluginTestServer{c: c}
	server.start()
	defer server.stop()

	authZPlugin := createTestPlugin(c)

	request := Request{
		User:           "user",
		RequestBody:    []byte("sample body"),
		RequestURI:     "www.authz.com/auth",
		RequestMethod:  "GET",
		RequestHeaders: map[string]string{"header": "value"},
	}
	server.replayResponse = Response{
		Err: "an error",
	}

	actualResponse, err := authZPlugin.AuthZRequest(&request)
	if err != nil {
		c.Fatalf("Failed to authorize request %v", err)
	}

	if !reflect.DeepEqual(server.replayResponse, *actualResponse) {
		c.Fatal("Response must be equal")
	}
	if !reflect.DeepEqual(request, server.recordedRequest) {
		c.Fatal("Requests must be equal")
	}
}

func (s *DockerSuite) TestAuthZRequestPlugin(c *check.C) {
	server := authZPluginTestServer{c: c}
	server.start()
	defer server.stop()

	authZPlugin := createTestPlugin(c)

	request := Request{
		User:           "user",
		RequestBody:    []byte("sample body"),
		RequestURI:     "www.authz.com/auth",
		RequestMethod:  "GET",
		RequestHeaders: map[string]string{"header": "value"},
	}
	server.replayResponse = Response{
		Allow: true,
		Msg:   "Sample message",
	}

	actualResponse, err := authZPlugin.AuthZRequest(&request)
	if err != nil {
		c.Fatalf("Failed to authorize request %v", err)
	}

	if !reflect.DeepEqual(server.replayResponse, *actualResponse) {
		c.Fatal("Response must be equal")
	}
	if !reflect.DeepEqual(request, server.recordedRequest) {
		c.Fatal("Requests must be equal")
	}
}

func (s *DockerSuite) TestAuthZResponsePlugin(c *check.C) {
	server := authZPluginTestServer{c: c}
	server.start()
	defer server.stop()

	authZPlugin := createTestPlugin(c)

	request := Request{
		User:        "user",
		RequestURI:  "someting.com/auth",
		RequestBody: []byte("sample body"),
	}
	server.replayResponse = Response{
		Allow: true,
		Msg:   "Sample message",
	}

	actualResponse, err := authZPlugin.AuthZResponse(&request)
	if err != nil {
		c.Fatalf("Failed to authorize request %v", err)
	}

	if !reflect.DeepEqual(server.replayResponse, *actualResponse) {
		c.Fatal("Response must be equal")
	}
	if !reflect.DeepEqual(request, server.recordedRequest) {
		c.Fatal("Requests must be equal")
	}
}

func (s *DockerSuite) TestResponseModifier(c *check.C) {
	r := httptest.NewRecorder()
	m := NewResponseModifier(r)
	m.Header().Set("h1", "v1")
	m.Write([]byte("body"))
	m.WriteHeader(http.StatusInternalServerError)

	m.FlushAll()
	if r.Header().Get("h1") != "v1" {
		c.Fatalf("Header value must exists %s", r.Header().Get("h1"))
	}
	if !reflect.DeepEqual(r.Body.Bytes(), []byte("body")) {
		c.Fatalf("Body value must exists %s", r.Body.Bytes())
	}
	if r.Code != http.StatusInternalServerError {
		c.Fatalf("Status code must be correct %d", r.Code)
	}
}

func (s *DockerSuite) TestDrainBody(c *check.C) {
	tests := []struct {
		length             int // length is the message length send to drainBody
		expectedBodyLength int // expectedBodyLength is the expected body length after drainBody is called
	}{
		{10, 10}, // Small message size
		{maxBodySize - 1, maxBodySize - 1}, // Max message size
		{maxBodySize * 2, 0},               // Large message size (skip copying body)

	}

	for _, test := range tests {
		msg := strings.Repeat("a", test.length)
		body, closer, err := drainBody(ioutil.NopCloser(bytes.NewReader([]byte(msg))))
		if err != nil {
			c.Fatal(err)
		}
		if len(body) != test.expectedBodyLength {
			c.Fatalf("Body must be copied, actual length: '%d'", len(body))
		}
		if closer == nil {
			c.Fatal("Closer must not be nil")
		}
		modified, err := ioutil.ReadAll(closer)
		if err != nil {
			c.Fatalf("Error must not be nil: '%v'", err)
		}
		if len(modified) != len(msg) {
			c.Fatalf("Result should not be truncated. Original length: '%d', new length: '%d'", len(msg), len(modified))
		}
	}
}

func (s *DockerSuite) TestResponseModifierOverride(c *check.C) {
	r := httptest.NewRecorder()
	m := NewResponseModifier(r)
	m.Header().Set("h1", "v1")
	m.Write([]byte("body"))
	m.WriteHeader(http.StatusInternalServerError)

	overrideHeader := make(http.Header)
	overrideHeader.Add("h1", "v2")
	overrideHeaderBytes, err := json.Marshal(overrideHeader)
	if err != nil {
		c.Fatalf("override header failed %v", err)
	}

	m.OverrideHeader(overrideHeaderBytes)
	m.OverrideBody([]byte("override body"))
	m.OverrideStatusCode(http.StatusNotFound)
	m.FlushAll()
	if r.Header().Get("h1") != "v2" {
		c.Fatalf("Header value must exists %s", r.Header().Get("h1"))
	}
	if !reflect.DeepEqual(r.Body.Bytes(), []byte("override body")) {
		c.Fatalf("Body value must exists %s", r.Body.Bytes())
	}
	if r.Code != http.StatusNotFound {
		c.Fatalf("Status code must be correct %d", r.Code)
	}
}

// createTestPlugin creates a new sample authorization plugin
func createTestPlugin(c *check.C) *authorizationPlugin {
	pwd, err := os.Getwd()
	if err != nil {
		c.Fatal(err)
	}

	client, err := plugins.NewClient("unix:///"+path.Join(pwd, pluginAddress), &tlsconfig.Options{InsecureSkipVerify: true})
	if err != nil {
		c.Fatalf("Failed to create client %v", err)
	}

	return &authorizationPlugin{name: "plugin", plugin: client}
}

// AuthZPluginTestServer is a simple server that implements the authZ plugin interface
type authZPluginTestServer struct {
	listener net.Listener
	c        *check.C
	// request stores the request sent from the daemon to the plugin
	recordedRequest Request
	// response stores the response sent from the plugin to the daemon
	replayResponse Response
	server         *httptest.Server
}

// start starts the test server that implements the plugin
func (t *authZPluginTestServer) start() {
	r := mux.NewRouter()
	l, err := net.Listen("unix", pluginAddress)
	if err != nil {
		t.c.Fatal(err)
	}
	t.listener = l
	r.HandleFunc("/Plugin.Activate", t.activate)
	r.HandleFunc("/"+AuthZApiRequest, t.auth)
	r.HandleFunc("/"+AuthZApiResponse, t.auth)
	t.server = &httptest.Server{
		Listener: l,
		Config: &http.Server{
			Handler: r,
			Addr:    pluginAddress,
		},
	}
	t.server.Start()
}

// stop stops the test server that implements the plugin
func (t *authZPluginTestServer) stop() {
	t.server.Close()
	os.Remove(pluginAddress)
	if t.listener != nil {
		t.listener.Close()
	}
}

// auth is a used to record/replay the authentication api messages
func (t *authZPluginTestServer) auth(w http.ResponseWriter, r *http.Request) {
	t.recordedRequest = Request{}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		t.c.Fatal(err)
	}
	r.Body.Close()
	json.Unmarshal(body, &t.recordedRequest)
	b, err := json.Marshal(t.replayResponse)
	if err != nil {
		t.c.Fatal(err)
	}
	w.Write(b)
}

func (t *authZPluginTestServer) activate(w http.ResponseWriter, r *http.Request) {
	b, err := json.Marshal(plugins.Manifest{Implements: []string{AuthZApiImplements}})
	if err != nil {
		t.c.Fatal(err)
	}
	w.Write(b)
}
