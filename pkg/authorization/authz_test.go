package authorization

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/pkg/tlsconfig"
	"github.com/gorilla/mux"
)

const pluginAddress = "authzplugin.sock"

func TestAuthZRequestPluginError(t *testing.T) {
	server := authZPluginTestServer{t: t}
	go server.start()
	defer server.stop()

	authZPlugin := createTestPlugin(t)

	request := Request{
		User:           "user",
		RequestBody:    []byte("sample body"),
		RequestURI:     "www.authz.com",
		RequestMethod:  "GET",
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
		t.Fatalf("Response must be equal")
	}
	if !reflect.DeepEqual(request, server.recordedRequest) {
		t.Fatalf("Requests must be equal")
	}
}

func TestAuthZRequestPlugin(t *testing.T) {
	server := authZPluginTestServer{t: t}
	go server.start()
	defer server.stop()

	authZPlugin := createTestPlugin(t)

	request := Request{
		User:           "user",
		RequestBody:    []byte("sample body"),
		RequestURI:     "www.authz.com",
		RequestMethod:  "GET",
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
		t.Fatalf("Response must be equal")
	}
	if !reflect.DeepEqual(request, server.recordedRequest) {
		t.Fatalf("Requests must be equal")
	}
}

func TestAuthZResponsePlugin(t *testing.T) {
	server := authZPluginTestServer{t: t}
	go server.start()
	defer server.stop()

	authZPlugin := createTestPlugin(t)

	request := Request{
		User:        "user",
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
		t.Fatalf("Response must be equal")
	}
	if !reflect.DeepEqual(request, server.recordedRequest) {
		t.Fatalf("Requests must be equal")
	}
}

func TestResponseModifier(t *testing.T) {
	r := httptest.NewRecorder()
	m := NewResponseModifier(r)
	m.Header().Set("h1", "v1")
	m.Write([]byte("body"))
	m.WriteHeader(500)

	m.Flush()
	if r.Header().Get("h1") != "v1" {
		t.Fatalf("Header value must exists %s", r.Header().Get("h1"))
	}
	if !reflect.DeepEqual(r.Body.Bytes(), []byte("body")) {
		t.Fatalf("Body value must exists %s", r.Body.Bytes())
	}
	if r.Code != 500 {
		t.Fatalf("Status code must be correct %d", r.Code)
	}
}

func TestResponseModifierOverride(t *testing.T) {
	r := httptest.NewRecorder()
	m := NewResponseModifier(r)
	m.Header().Set("h1", "v1")
	m.Write([]byte("body"))
	m.WriteHeader(500)

	overrideHeader := make(http.Header)
	overrideHeader.Add("h1", "v2")
	overrideHeaderBytes, err := json.Marshal(overrideHeader)
	if err != nil {
		t.Fatalf("override header failed %v", err)
	}

	m.OverrideHeader(overrideHeaderBytes)
	m.OverrideBody([]byte("override body"))
	m.OverrideStatusCode(404)
	m.Flush()
	if r.Header().Get("h1") != "v2" {
		t.Fatalf("Header value must exists %s", r.Header().Get("h1"))
	}
	if !reflect.DeepEqual(r.Body.Bytes(), []byte("override body")) {
		t.Fatalf("Body value must exists %s", r.Body.Bytes())
	}
	if r.Code != 404 {
		t.Fatalf("Status code must be correct %d", r.Code)
	}
}

// createTestPlugin creates a new sample authorization plugin
func createTestPlugin(t *testing.T) *authorizationPlugin {
	plugin := &plugins.Plugin{Name: "authz"}
	pwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	plugin.Client, err = plugins.NewClient("unix:///"+path.Join(pwd, pluginAddress), tlsconfig.Options{InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("Failed to create client %v", err)
	}

	return &authorizationPlugin{name: "plugin", plugin: plugin}
}

// AuthZPluginTestServer is a simple server that implements the authZ plugin interface
type authZPluginTestServer struct {
	listener net.Listener
	t        *testing.T
	// request stores the request sent from the daemon to the plugin
	recordedRequest Request
	// response stores the response sent from the plugin to the daemon
	replayResponse Response
}

// start starts the test server that implements the plugin
func (t *authZPluginTestServer) start() {
	r := mux.NewRouter()
	os.Remove(pluginAddress)
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: pluginAddress, Net: "unix"})
	if err != nil {
		t.t.Fatalf("Failed to listen %v", err)
	}
	t.listener = l

	r.HandleFunc("/Plugin.Activate", t.activate)
	r.HandleFunc("/"+AuthZApiRequest, t.auth)
	r.HandleFunc("/"+AuthZApiResponse, t.auth)
	t.listener, err = net.Listen("tcp", pluginAddress)
	server := http.Server{Handler: r, Addr: pluginAddress}
	server.Serve(l)
}

// stop stops the test server that implements the plugin
func (t *authZPluginTestServer) stop() {
	os.Remove(pluginAddress)
	if t.listener != nil {
		t.listener.Close()
	}
}

// auth is a used to record/replay the authentication api messages
func (t *authZPluginTestServer) auth(w http.ResponseWriter, r *http.Request) {
	t.recordedRequest = Request{}
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	json.Unmarshal(body, &t.recordedRequest)
	b, err := json.Marshal(t.replayResponse)
	if err != nil {
		log.Fatal(err)
	}
	w.Write(b)
}

func (t *authZPluginTestServer) activate(w http.ResponseWriter, r *http.Request) {
	b, err := json.Marshal(plugins.Manifest{Implements: []string{AuthZApiImplements}})
	if err != nil {
		log.Fatal(err)
	}
	w.Write(b)
}
