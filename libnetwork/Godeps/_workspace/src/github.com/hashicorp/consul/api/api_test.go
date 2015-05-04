package api

import (
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/hashicorp/consul/testutil"
)

var consulConfig = `{
	"ports": {
		"dns": 19000,
		"http": 18800,
		"rpc": 18600,
		"serf_lan": 18200,
		"serf_wan": 18400,
		"server": 18000
	},
	"bind_addr": "127.0.0.1",
	"data_dir": "%s",
	"bootstrap": true,
	"log_level": "debug",
	"server": true
}`

type testServer struct {
	pid        int
	dataDir    string
	configFile string
}

type testPortConfig struct {
	DNS     int `json:"dns,omitempty"`
	HTTP    int `json:"http,omitempty"`
	RPC     int `json:"rpc,omitempty"`
	SerfLan int `json:"serf_lan,omitempty"`
	SerfWan int `json:"serf_wan,omitempty"`
	Server  int `json:"server,omitempty"`
}

type testAddressConfig struct {
	HTTP string `json:"http,omitempty"`
}

type testServerConfig struct {
	Bootstrap bool               `json:"bootstrap,omitempty"`
	Server    bool               `json:"server,omitempty"`
	DataDir   string             `json:"data_dir,omitempty"`
	LogLevel  string             `json:"log_level,omitempty"`
	Addresses *testAddressConfig `json:"addresses,omitempty"`
	Ports     testPortConfig     `json:"ports,omitempty"`
}

// Callback functions for modifying config
type configCallback func(c *Config)
type serverConfigCallback func(c *testServerConfig)

func defaultConfig() *testServerConfig {
	return &testServerConfig{
		Bootstrap: true,
		Server:    true,
		LogLevel:  "debug",
		Ports: testPortConfig{
			DNS:     19000,
			HTTP:    18800,
			RPC:     18600,
			SerfLan: 18200,
			SerfWan: 18400,
			Server:  18000,
		},
	}
}

func (s *testServer) stop() {
	defer os.RemoveAll(s.dataDir)
	defer os.RemoveAll(s.configFile)

	cmd := exec.Command("kill", "-9", fmt.Sprintf("%d", s.pid))
	if err := cmd.Run(); err != nil {
		panic(err)
	}
}

func newTestServer(t *testing.T) *testServer {
	return newTestServerWithConfig(t, func(c *testServerConfig) {})
}

func newTestServerWithConfig(t *testing.T, cb serverConfigCallback) *testServer {
	if path, err := exec.LookPath("consul"); err != nil || path == "" {
		t.Log("consul not found on $PATH, skipping")
		t.SkipNow()
	}

	pidFile, err := ioutil.TempFile("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	pidFile.Close()
	os.Remove(pidFile.Name())

	dataDir, err := ioutil.TempDir("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	configFile, err := ioutil.TempFile("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	consulConfig := defaultConfig()
	consulConfig.DataDir = dataDir

	cb(consulConfig)

	configContent, err := json.Marshal(consulConfig)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if _, err := configFile.Write(configContent); err != nil {
		t.Fatalf("err: %s", err)
	}
	configFile.Close()

	// Start the server
	cmd := exec.Command("consul", "agent", "-config-file", configFile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	return &testServer{
		pid:        cmd.Process.Pid,
		dataDir:    dataDir,
		configFile: configFile.Name(),
	}
}

func makeClient(t *testing.T) (*Client, *testServer) {
	return makeClientWithConfig(t, func(c *Config) {
		c.Address = "127.0.0.1:18800"
	}, func(c *testServerConfig) {})
}

func makeClientWithConfig(t *testing.T, cb1 configCallback, cb2 serverConfigCallback) (*Client, *testServer) {
	// Make client config
	conf := DefaultConfig()
	cb1(conf)

	// Create client
	client, err := NewClient(conf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create server
	server := newTestServerWithConfig(t, cb2)

	// Allow the server some time to start, and verify we have a leader.
	testutil.WaitForResult(func() (bool, error) {
		req := client.newRequest("GET", "/v1/catalog/nodes")
		_, resp, err := client.doRequest(req)
		if err != nil {
			return false, err
		}
		resp.Body.Close()

		// Ensure we have a leader and a node registeration
		if leader := resp.Header.Get("X-Consul-KnownLeader"); leader != "true" {
			return false, fmt.Errorf("Consul leader status: %#v", leader)
		}
		if resp.Header.Get("X-Consul-Index") == "0" {
			return false, fmt.Errorf("Consul index is 0")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	return client, server
}

func testKey() string {
	buf := make([]byte, 16)
	if _, err := crand.Read(buf); err != nil {
		panic(fmt.Errorf("Failed to read random bytes: %v", err))
	}

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16])
}

func TestSetQueryOptions(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	r := c.newRequest("GET", "/v1/kv/foo")
	q := &QueryOptions{
		Datacenter:        "foo",
		AllowStale:        true,
		RequireConsistent: true,
		WaitIndex:         1000,
		WaitTime:          100 * time.Second,
		Token:             "12345",
	}
	r.setQueryOptions(q)

	if r.params.Get("dc") != "foo" {
		t.Fatalf("bad: %v", r.params)
	}
	if _, ok := r.params["stale"]; !ok {
		t.Fatalf("bad: %v", r.params)
	}
	if _, ok := r.params["consistent"]; !ok {
		t.Fatalf("bad: %v", r.params)
	}
	if r.params.Get("index") != "1000" {
		t.Fatalf("bad: %v", r.params)
	}
	if r.params.Get("wait") != "100000ms" {
		t.Fatalf("bad: %v", r.params)
	}
	if r.params.Get("token") != "12345" {
		t.Fatalf("bad: %v", r.params)
	}
}

func TestSetWriteOptions(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	r := c.newRequest("GET", "/v1/kv/foo")
	q := &WriteOptions{
		Datacenter: "foo",
		Token:      "23456",
	}
	r.setWriteOptions(q)

	if r.params.Get("dc") != "foo" {
		t.Fatalf("bad: %v", r.params)
	}
	if r.params.Get("token") != "23456" {
		t.Fatalf("bad: %v", r.params)
	}
}

func TestRequestToHTTP(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	r := c.newRequest("DELETE", "/v1/kv/foo")
	q := &QueryOptions{
		Datacenter: "foo",
	}
	r.setQueryOptions(q)
	req, err := r.toHTTP()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if req.Method != "DELETE" {
		t.Fatalf("bad: %v", req)
	}
	if req.URL.RequestURI() != "/v1/kv/foo?dc=foo" {
		t.Fatalf("bad: %v", req)
	}
}

func TestParseQueryMeta(t *testing.T) {
	resp := &http.Response{
		Header: make(map[string][]string),
	}
	resp.Header.Set("X-Consul-Index", "12345")
	resp.Header.Set("X-Consul-LastContact", "80")
	resp.Header.Set("X-Consul-KnownLeader", "true")

	qm := &QueryMeta{}
	if err := parseQueryMeta(resp, qm); err != nil {
		t.Fatalf("err: %v", err)
	}

	if qm.LastIndex != 12345 {
		t.Fatalf("Bad: %v", qm)
	}
	if qm.LastContact != 80*time.Millisecond {
		t.Fatalf("Bad: %v", qm)
	}
	if !qm.KnownLeader {
		t.Fatalf("Bad: %v", qm)
	}
}

func TestAPI_UnixSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}

	tempDir, err := ioutil.TempDir("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(tempDir)
	socket := filepath.Join(tempDir, "test.sock")

	c, s := makeClientWithConfig(t, func(c *Config) {
		c.Address = "unix://" + socket
	}, func(c *testServerConfig) {
		c.Addresses = &testAddressConfig{
			HTTP: "unix://" + socket,
		}
	})
	defer s.stop()

	agent := c.Agent()

	info, err := agent.Self()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if info["Config"]["NodeName"] == "" {
		t.Fatalf("bad: %v", info)
	}
}
