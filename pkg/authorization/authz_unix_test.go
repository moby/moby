//go:build !windows

// TODO Windows: This uses a Unix socket for testing. This might be possible
// to port to Windows using a named pipe instead.

package authorization

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/docker/go-connections/tlsconfig"
	"github.com/gorilla/mux"
	"github.com/moby/moby/v2/pkg/plugins"
)

const (
	pluginAddress = "authz-test-plugin.sock"
)

func TestAuthZRequestPluginError(t *testing.T) {
	server := authZPluginTestServer{t: t}
	server.start()
	defer server.stop()

	authZPlugin := createTestPlugin(t, server.socketAddress())

	request := Request{
		User:           "user",
		RequestBody:    []byte("sample body"),
		RequestURI:     "www.authz.com/auth",
		RequestMethod:  http.MethodGet,
		RequestHeaders: map[string]string{"header": "value"},
	}
	server.replayResponse = Response{
		Err: "an error",
	}

	actualResponse, err := authZPlugin.AuthZRequest(&request)
	if err != nil {
		t.Fatalf("Failed to authorize request %v", err)
	}

	if !reflect.DeepEqual(server.replayResponse, *actualResponse) {
		t.Fatal("Response must be equal")
	}
	if !reflect.DeepEqual(request, server.recordedRequest) {
		t.Fatal("Requests must be equal")
	}
}

func TestAuthZRequestPlugin(t *testing.T) {
	server := authZPluginTestServer{t: t}
	server.start()
	defer server.stop()

	authZPlugin := createTestPlugin(t, server.socketAddress())

	request := Request{
		User:           "user",
		RequestBody:    []byte("sample body"),
		RequestURI:     "www.authz.com/auth",
		RequestMethod:  http.MethodGet,
		RequestHeaders: map[string]string{"header": "value"},
	}
	server.replayResponse = Response{
		Allow: true,
		Msg:   "Sample message",
	}

	actualResponse, err := authZPlugin.AuthZRequest(&request)
	if err != nil {
		t.Fatalf("Failed to authorize request %v", err)
	}

	if !reflect.DeepEqual(server.replayResponse, *actualResponse) {
		t.Fatal("Response must be equal")
	}
	if !reflect.DeepEqual(request, server.recordedRequest) {
		t.Fatal("Requests must be equal")
	}
}

func TestAuthZResponsePlugin(t *testing.T) {
	server := authZPluginTestServer{t: t}
	server.start()
	defer server.stop()

	authZPlugin := createTestPlugin(t, server.socketAddress())

	request := Request{
		User:        "user",
		RequestURI:  "something.com/auth",
		RequestBody: []byte("sample body"),
	}
	server.replayResponse = Response{
		Allow: true,
		Msg:   "Sample message",
	}

	actualResponse, err := authZPlugin.AuthZResponse(&request)
	if err != nil {
		t.Fatalf("Failed to authorize request %v", err)
	}

	if !reflect.DeepEqual(server.replayResponse, *actualResponse) {
		t.Fatal("Response must be equal")
	}
	if !reflect.DeepEqual(request, server.recordedRequest) {
		t.Fatal("Requests must be equal")
	}
}

func TestResponseModifier(t *testing.T) {
	r := httptest.NewRecorder()
	m := NewResponseModifier(r)
	m.Header().Set("h1", "v1")
	m.Write([]byte("body"))
	m.WriteHeader(http.StatusInternalServerError)

	m.FlushAll()
	if r.Header().Get("h1") != "v1" {
		t.Fatalf("Header value must exists %s", r.Header().Get("h1"))
	}
	if !reflect.DeepEqual(r.Body.Bytes(), []byte("body")) {
		t.Fatalf("Body value must exists %s", r.Body.Bytes())
	}
	if r.Code != http.StatusInternalServerError {
		t.Fatalf("Status code must be correct %d", r.Code)
	}
}

type recordingPlugin struct {
	recordedRequest Request
}

func (p *recordingPlugin) Name() string { return "recording-plugin" }

func (p *recordingPlugin) AuthZRequest(authReq *Request) (*Response, error) {
	p.recordedRequest = *authReq
	p.recordedRequest.RequestBody = bytes.Clone(authReq.RequestBody)
	return &Response{Allow: true}, nil
}

func (p *recordingPlugin) AuthZResponse(_ *Request) (*Response, error) {
	return &Response{Allow: true}, nil
}

func TestAuthZRequestBodyWithinLimit(t *testing.T) {
	payload := strings.Repeat("a", maxBodySize)
	plugin := &recordingPlugin{}
	ctx := NewCtx([]Plugin{plugin}, "user", "tls", http.MethodPost, "/containers/create")

	req := httptest.NewRequest(http.MethodPost, "http://example.com/containers/create", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	if err := ctx.AuthZRequest(httptest.NewRecorder(), req); err != nil {
		t.Fatalf("AuthZRequest failed: %v", err)
	}

	if string(plugin.recordedRequest.RequestBody) != payload {
		t.Fatalf("expected full request body to be sent to plugin, got length %d, expected %d", len(plugin.recordedRequest.RequestBody), len(payload))
	}

	remaining, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("failed to read request body after authz: %v", err)
	}
	if string(remaining) != payload {
		t.Fatalf("request body should be preserved for downstream readers")
	}
}

func TestAuthZRequestBodyOverLimit(t *testing.T) {
	payload := strings.Repeat("a", maxBodySize+1)
	plugin := &recordingPlugin{}
	ctx := NewCtx([]Plugin{plugin}, "user", "tls", http.MethodPost, "/containers/create")

	req := httptest.NewRequest(http.MethodPost, "http://example.com/containers/create", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	err := ctx.AuthZRequest(httptest.NewRecorder(), req)
	if err == nil {
		t.Fatal("expected AuthZRequest to reject body over max size")
	}
	if !strings.Contains(err.Error(), "request body too large for authorization plugin") {
		t.Fatalf("unexpected error: %v", err)
	}

	remaining, readErr := io.ReadAll(req.Body)
	if readErr != nil {
		t.Fatalf("failed to read request body after authz error: %v", readErr)
	}
	if string(remaining) != payload {
		t.Fatalf("request body should still be preserved after over-limit check")
	}
}

func TestSendBody(t *testing.T) {
	testcases := []struct {
		url         string
		contentType string
		expected    bool
	}{
		{
			contentType: "application/json",
			expected:    true,
		},
		{
			contentType: "Application/json",
			expected:    true,
		},
		{
			contentType: "application/JSON",
			expected:    true,
		},
		{
			contentType: "APPLICATION/JSON",
			expected:    true,
		},
		{
			contentType: "application/json; charset=utf-8",
			expected:    true,
		},
		{
			contentType: "application/json;charset=utf-8",
			expected:    true,
		},
		{
			contentType: "application/json; charset=UTF8",
			expected:    true,
		},
		{
			contentType: "application/json;charset=UTF8",
			expected:    true,
		},
		{
			contentType: "text/html",
			expected:    false,
		},
		{
			contentType: "",
			expected:    false,
		},
		{
			url:         "nothing.com/auth",
			contentType: "",
			expected:    false,
		},
		{
			url:         "nothing.com/auth",
			contentType: "application/json;charset=UTF8",
			expected:    false,
		},
		{
			url:         "nothing.com/auth?p1=test",
			contentType: "application/json;charset=UTF8",
			expected:    false,
		},
		{
			url:         "nothing.com/test?p1=/auth",
			contentType: "application/json;charset=UTF8",
			expected:    true,
		},
		{
			url:         "nothing.com/something/auth",
			contentType: "application/json;charset=UTF8",
			expected:    true,
		},
		{
			url:         "nothing.com/auth/test",
			contentType: "application/json;charset=UTF8",
			expected:    false,
		},
		{
			url:         "nothing.com/v1.24/auth/test",
			contentType: "application/json;charset=UTF8",
			expected:    false,
		},
		{
			url:         "nothing.com/v1/auth/test",
			contentType: "application/json;charset=UTF8",
			expected:    false,
		},
		{
			url:         "www.nothing.com/v1.24/auth/test",
			contentType: "application/json;charset=UTF8",
			expected:    false,
		},
		{
			url:         "https://www.nothing.com/v1.24/auth/test",
			contentType: "application/json;charset=UTF8",
			expected:    false,
		},
		{
			url:         "http://nothing.com/v1.24/auth/test",
			contentType: "application/json;charset=UTF8",
			expected:    false,
		},
		{
			url:         "www.nothing.com/test?p1=/auth",
			contentType: "application/json;charset=UTF8",
			expected:    true,
		},
		{
			url:         "http://www.nothing.com/test?p1=/auth",
			contentType: "application/json;charset=UTF8",
			expected:    true,
		},
		{
			url:         "www.nothing.com/something/auth",
			contentType: "application/json;charset=UTF8",
			expected:    true,
		},
		{
			url:         "https://www.nothing.com/something/auth",
			contentType: "application/json;charset=UTF8",
			expected:    true,
		},
	}

	for _, testcase := range testcases {
		header := http.Header{}
		header.Set("Content-Type", testcase.contentType)
		if testcase.url == "" {
			testcase.url = "nothing.com"
		}

		if b := sendBody(testcase.url, header); b != testcase.expected {
			t.Fatalf("sendBody failed: url: %s, content-type: %s; Expected: %t, Actual: %t", testcase.url, testcase.contentType, testcase.expected, b)
		}
	}
}

func TestResponseModifierOverride(t *testing.T) {
	r := httptest.NewRecorder()
	m := NewResponseModifier(r)
	m.Header().Set("h1", "v1")
	m.Write([]byte("body"))
	m.WriteHeader(http.StatusInternalServerError)

	overrideHeader := make(http.Header)
	overrideHeader.Add("h1", "v2")
	overrideHeaderBytes, err := json.Marshal(overrideHeader)
	if err != nil {
		t.Fatalf("override header failed %v", err)
	}

	m.OverrideHeader(overrideHeaderBytes)
	m.OverrideBody([]byte("override body"))
	m.OverrideStatusCode(http.StatusNotFound)
	m.FlushAll()
	if r.Header().Get("h1") != "v2" {
		t.Fatalf("Header value must exists %s", r.Header().Get("h1"))
	}
	if !reflect.DeepEqual(r.Body.Bytes(), []byte("override body")) {
		t.Fatalf("Body value must exists %s", r.Body.Bytes())
	}
	if r.Code != http.StatusNotFound {
		t.Fatalf("Status code must be correct %d", r.Code)
	}
}

// createTestPlugin creates a new sample authorization plugin
func createTestPlugin(t *testing.T, socketAddress string) *authorizationPlugin {
	client, err := plugins.NewClient("unix:///"+socketAddress, &tlsconfig.Options{InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("Failed to create client %v", err)
	}

	return &authorizationPlugin{name: "plugin", plugin: client}
}

// AuthZPluginTestServer is a simple server that implements the authZ plugin interface
type authZPluginTestServer struct {
	listener net.Listener
	t        *testing.T
	// request stores the request sent from the daemon to the plugin
	recordedRequest Request
	// response stores the response sent from the plugin to the daemon
	replayResponse Response
	server         *httptest.Server
	tmpDir         string
}

func (t *authZPluginTestServer) socketAddress() string {
	return path.Join(t.tmpDir, pluginAddress)
}

// start starts the test server that implements the plugin
func (t *authZPluginTestServer) start() {
	var err error
	t.tmpDir, err = os.MkdirTemp("", "authz")
	if err != nil {
		t.t.Fatal(err)
	}

	r := mux.NewRouter()
	l, err := net.Listen("unix", t.socketAddress())
	if err != nil {
		t.t.Fatal(err)
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

			ReadHeaderTimeout: 5 * time.Minute, // "G112: Potential Slowloris Attack (gosec)"; not a real concern for our use, so setting a long timeout.
		},
	}
	t.server.Start()
}

// stop stops the test server that implements the plugin
func (t *authZPluginTestServer) stop() {
	t.server.Close()
	_ = os.RemoveAll(t.tmpDir)
	if t.listener != nil {
		t.listener.Close()
	}
}

// auth is a used to record/replay the authentication api messages
func (t *authZPluginTestServer) auth(w http.ResponseWriter, r *http.Request) {
	t.recordedRequest = Request{}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.t.Fatal(err)
	}
	r.Body.Close()
	json.Unmarshal(body, &t.recordedRequest)
	b, err := json.Marshal(t.replayResponse)
	if err != nil {
		t.t.Fatal(err)
	}
	w.Write(b)
}

func (t *authZPluginTestServer) activate(w http.ResponseWriter, r *http.Request) {
	b, err := json.Marshal(plugins.Manifest{Implements: []string{AuthZApiImplements}})
	if err != nil {
		t.t.Fatal(err)
	}
	w.Write(b)
}
