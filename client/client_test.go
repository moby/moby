package client

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"golang.org/x/net/context"
)

func TestNewEnvClient(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping unix only test for windows")
	}
	cases := []struct {
		envs            map[string]string
		expectedError   string
		expectedVersion string
	}{
		{
			envs:            map[string]string{},
			expectedVersion: DefaultVersion,
		},
		{
			envs: map[string]string{
				"DOCKER_CERT_PATH": "invalid/path",
			},
			expectedError: "Could not load X509 key pair: open invalid/path/cert.pem: no such file or directory. Make sure the key is not encrypted",
		},
		{
			envs: map[string]string{
				"DOCKER_CERT_PATH": "testdata/",
			},
			expectedVersion: DefaultVersion,
		},
		{
			envs: map[string]string{
				"DOCKER_CERT_PATH":  "testdata/",
				"DOCKER_TLS_VERIFY": "1",
			},
			expectedVersion: DefaultVersion,
		},
		{
			envs: map[string]string{
				"DOCKER_CERT_PATH": "testdata/",
				"DOCKER_HOST":      "https://notaunixsocket",
			},
			expectedVersion: DefaultVersion,
		},
		{
			envs: map[string]string{
				"DOCKER_HOST": "host",
			},
			expectedError: "unable to parse docker host `host`",
		},
		{
			envs: map[string]string{
				"DOCKER_HOST": "invalid://url",
			},
			expectedVersion: DefaultVersion,
		},
		{
			envs: map[string]string{
				"DOCKER_API_VERSION": "anything",
			},
			expectedVersion: "anything",
		},
		{
			envs: map[string]string{
				"DOCKER_API_VERSION": "1.22",
			},
			expectedVersion: "1.22",
		},
	}
	for _, c := range cases {
		recoverEnvs := setupEnvs(t, c.envs)
		apiclient, err := NewEnvClient()
		if c.expectedError != "" {
			if err == nil {
				t.Errorf("expected an error for %v", c)
			} else if err.Error() != c.expectedError {
				t.Errorf("expected an error %s, got %s, for %v", c.expectedError, err.Error(), c)
			}
		} else {
			if err != nil {
				t.Error(err)
			}
			version := apiclient.ClientVersion()
			if version != c.expectedVersion {
				t.Errorf("expected %s, got %s, for %v", c.expectedVersion, version, c)
			}
		}

		if c.envs["DOCKER_TLS_VERIFY"] != "" {
			// pedantic checking that this is handled correctly
			tr := apiclient.client.Transport.(*http.Transport)
			if tr.TLSClientConfig == nil {
				t.Errorf("no tls config found when DOCKER_TLS_VERIFY enabled")
			}

			if tr.TLSClientConfig.InsecureSkipVerify {
				t.Errorf("tls verification should be enabled")
			}
		}

		recoverEnvs(t)
	}
}

func setupEnvs(t *testing.T, envs map[string]string) func(*testing.T) {
	oldEnvs := map[string]string{}
	for key, value := range envs {
		oldEnv := os.Getenv(key)
		oldEnvs[key] = oldEnv
		err := os.Setenv(key, value)
		if err != nil {
			t.Error(err)
		}
	}
	return func(t *testing.T) {
		for key, value := range oldEnvs {
			err := os.Setenv(key, value)
			if err != nil {
				t.Error(err)
			}
		}
	}
}

func TestGetAPIPath(t *testing.T) {
	cases := []struct {
		v string
		p string
		q url.Values
		e string
	}{
		{"", "/containers/json", nil, "/containers/json"},
		{"", "/containers/json", url.Values{}, "/containers/json"},
		{"", "/containers/json", url.Values{"s": []string{"c"}}, "/containers/json?s=c"},
		{"1.22", "/containers/json", nil, "/v1.22/containers/json"},
		{"1.22", "/containers/json", url.Values{}, "/v1.22/containers/json"},
		{"1.22", "/containers/json", url.Values{"s": []string{"c"}}, "/v1.22/containers/json?s=c"},
		{"v1.22", "/containers/json", nil, "/v1.22/containers/json"},
		{"v1.22", "/containers/json", url.Values{}, "/v1.22/containers/json"},
		{"v1.22", "/containers/json", url.Values{"s": []string{"c"}}, "/v1.22/containers/json?s=c"},
		{"v1.22", "/networks/kiwl$%^", nil, "/v1.22/networks/kiwl$%25%5E"},
	}

	for _, cs := range cases {
		c, err := NewClient("unix:///var/run/docker.sock", cs.v, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		g := c.getAPIPath(cs.p, cs.q)
		if g != cs.e {
			t.Fatalf("Expected %s, got %s", cs.e, g)
		}

		err = c.Close()
		if nil != err {
			t.Fatalf("close client failed, error message: %s", err)
		}
	}
}

func TestParseHost(t *testing.T) {
	cases := []struct {
		host  string
		proto string
		addr  string
		base  string
		err   bool
	}{
		{"", "", "", "", true},
		{"foobar", "", "", "", true},
		{"foo://bar", "foo", "bar", "", false},
		{"tcp://localhost:2476", "tcp", "localhost:2476", "", false},
		{"tcp://localhost:2476/path", "tcp", "localhost:2476", "/path", false},
	}

	for _, cs := range cases {
		p, a, b, e := ParseHost(cs.host)
		if cs.err && e == nil {
			t.Fatalf("expected error, got nil")
		}
		if !cs.err && e != nil {
			t.Fatal(e)
		}
		if cs.proto != p {
			t.Fatalf("expected proto %s, got %s", cs.proto, p)
		}
		if cs.addr != a {
			t.Fatalf("expected addr %s, got %s", cs.addr, a)
		}
		if cs.base != b {
			t.Fatalf("expected base %s, got %s", cs.base, b)
		}
	}
}

func TestUpdateClientVersion(t *testing.T) {
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			splitQuery := strings.Split(req.URL.Path, "/")
			queryVersion := splitQuery[1]
			b, err := json.Marshal(types.Version{
				APIVersion: queryVersion,
			})
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(bytes.NewReader(b)),
			}, nil
		}),
	}

	cases := []struct {
		v string
	}{
		{"1.20"},
		{"v1.21"},
		{"1.22"},
		{"v1.22"},
	}

	for _, cs := range cases {
		client.UpdateClientVersion(cs.v)
		r, err := client.ServerVersion(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimPrefix(r.APIVersion, "v") != strings.TrimPrefix(cs.v, "v") {
			t.Fatalf("Expected %s, got %s", cs.v, r.APIVersion)
		}
	}
}

func TestNewEnvClientSetsDefaultVersion(t *testing.T) {
	// Unset environment variables
	envVarKeys := []string{
		"DOCKER_HOST",
		"DOCKER_API_VERSION",
		"DOCKER_TLS_VERIFY",
		"DOCKER_CERT_PATH",
	}
	envVarValues := make(map[string]string)
	for _, key := range envVarKeys {
		envVarValues[key] = os.Getenv(key)
		os.Setenv(key, "")
	}

	client, err := NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}
	if client.version != DefaultVersion {
		t.Fatalf("Expected %s, got %s", DefaultVersion, client.version)
	}

	expected := "1.22"
	os.Setenv("DOCKER_API_VERSION", expected)
	client, err = NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}
	if client.version != expected {
		t.Fatalf("Expected %s, got %s", expected, client.version)
	}

	// Restore environment variables
	for _, key := range envVarKeys {
		os.Setenv(key, envVarValues[key])
	}
}
