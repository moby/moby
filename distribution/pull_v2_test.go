package distribution // import "github.com/docker/docker/distribution"

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/distribution/reference"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/image"
	"github.com/docker/docker/registry"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestFormatPlatform(t *testing.T) {
	var platform ocispec.Platform
	result := formatPlatform(platform)
	if strings.HasPrefix(result, "unknown") {
		t.Fatal("expected formatPlatform to show a known platform")
	}
	if !strings.HasPrefix(result, runtime.GOOS) {
		t.Fatal("expected formatPlatform to show the current platform")
	}
	if runtime.GOOS == "windows" {
		if !strings.HasPrefix(result, "windows") {
			t.Fatal("expected formatPlatform to show windows platform")
		}
		matches, _ := regexp.MatchString("windows.* [0-9]", result)
		if !matches {
			t.Fatalf("expected formatPlatform to show windows platform with a version, but got '%s'", result)
		}
	}
}

func TestPullSchema2Config(t *testing.T) {
	ctx := context.Background()

	const imageJSON = `{
	"architecture": "amd64",
	"os": "linux",
	"config": {},
	"rootfs": {
		"type": "layers",
		"diff_ids": []
	}
}`
	expectedDigest := digest.Digest("sha256:66ad98165d38f53ee73868f82bd4eed60556ddfee824810a4062c4f777b20a5b")

	tests := []struct {
		name           string
		handler        func(callCount int, w http.ResponseWriter)
		expectError    string
		expectAttempts int64
	}{
		{
			name: "success first time",
			handler: func(callCount int, w http.ResponseWriter) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(imageJSON))
			},
			expectAttempts: 1,
		},
		{
			name: "500 status",
			handler: func(callCount int, w http.ResponseWriter) {
				if callCount == 1 {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(imageJSON))
			},
			expectAttempts: 2,
		},
		{
			name: "EOF",
			handler: func(callCount int, w http.ResponseWriter) {
				if callCount == 1 {
					panic("intentional panic")
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(imageJSON))
			},
			expectAttempts: 2,
		},
		{
			name: "unauthorized",
			handler: func(callCount int, w http.ResponseWriter) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte("you need to be authenticated"))
			},
			expectError:    "unauthorized: you need to be authenticated",
			expectAttempts: 1,
		},
		{
			name: "unauthorized JSON",
			handler: func(callCount int, w http.ResponseWriter) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`					{ "errors":	[{"code": "UNAUTHORIZED", "message": "you need to be authenticated", "detail": "more detail"}]}`))
			},
			expectError:    "unauthorized: you need to be authenticated",
			expectAttempts: 1,
		},
		{
			name: "unauthorized JSON no body",
			handler: func(callCount int, w http.ResponseWriter) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
			},
			expectError:    "unauthorized: authentication required",
			expectAttempts: 1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var callCount int64
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Logf("HTTP %s %s", r.Method, r.URL.Path)
				defer r.Body.Close()
				switch {
				case r.Method == "GET" && r.URL.Path == "/v2":
					w.WriteHeader(http.StatusOK)
				case r.Method == "GET" && r.URL.Path == "/v2/docker.io/library/testremotename/blobs/"+expectedDigest.String():
					tt.handler(int(atomic.AddInt64(&callCount, 1)), w)
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer ts.Close()

			p := testNewPuller(t, ts.URL)

			config, err := p.pullSchema2Config(ctx, expectedDigest)
			if tt.expectError == "" {
				if err != nil {
					t.Fatal(err)
				}

				_, err = image.NewFromJSON(config)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error to contain %q", tt.expectError)
				}
				if !strings.Contains(err.Error(), tt.expectError) {
					t.Fatalf("expected error=%q to contain %q", err, tt.expectError)
				}
			}

			if callCount != tt.expectAttempts {
				t.Fatalf("got callCount=%d but expected=%d", callCount, tt.expectAttempts)
			}
		})
	}
}

func testNewPuller(t *testing.T, rawurl string) *puller {
	t.Helper()

	uri, err := url.Parse(rawurl)
	if err != nil {
		t.Fatalf("could not parse url from test server: %v", err)
	}

	endpoint := registry.APIEndpoint{
		Mirror:       false,
		URL:          uri,
		Official:     false,
		TrimHostname: false,
		TLSConfig:    nil,
	}
	n, _ := reference.ParseNormalizedNamed("testremotename")
	repoInfo := &registry.RepositoryInfo{
		Name: n,
		Index: &registrytypes.IndexInfo{
			Name:     "testrepo",
			Mirrors:  nil,
			Secure:   false,
			Official: false,
		},
		Official: false,
	}
	imagePullConfig := &ImagePullConfig{
		Config: Config{
			MetaHeaders: http.Header{},
			AuthConfig: &registrytypes.AuthConfig{
				RegistryToken: secretRegistryToken,
			},
		},
	}

	p := newPuller(endpoint, repoInfo, imagePullConfig, nil)
	p.repo, err = newRepository(context.Background(), p.repoInfo, p.endpoint, p.config.MetaHeaders, p.config.AuthConfig, "pull")
	if err != nil {
		t.Fatal(err)
	}
	return p
}
