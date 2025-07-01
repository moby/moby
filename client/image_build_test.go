package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/registry"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageBuildError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ImageBuild(context.Background(), nil, build.ImageBuildOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestImageBuild(t *testing.T) {
	v1 := "value1"
	v2 := "value2"
	emptyRegistryConfig := "bnVsbA=="
	buildCases := []struct {
		buildOptions           build.ImageBuildOptions
		expectedQueryParams    map[string]string
		expectedTags           []string
		expectedRegistryConfig string
	}{
		{
			buildOptions: build.ImageBuildOptions{
				SuppressOutput: true,
				NoCache:        true,
				Remove:         true,
				ForceRemove:    true,
				PullParent:     true,
			},
			expectedQueryParams: map[string]string{
				"q":       "1",
				"nocache": "1",
				"forcerm": "1",
				"pull":    "1",
			},
			expectedTags:           []string{},
			expectedRegistryConfig: emptyRegistryConfig,
		},
		{
			buildOptions: build.ImageBuildOptions{
				SuppressOutput: false,
				NoCache:        false,
				Remove:         false,
				ForceRemove:    false,
				PullParent:     false,
			},
			expectedQueryParams: map[string]string{
				"q":       "",
				"nocache": "",
				"rm":      "0",
				"forcerm": "",
				"pull":    "",
			},
			expectedTags:           []string{},
			expectedRegistryConfig: emptyRegistryConfig,
		},
		{
			buildOptions: build.ImageBuildOptions{
				RemoteContext: "remoteContext",
				Isolation:     container.Isolation("isolation"),
				CPUSetCPUs:    "2",
				CPUSetMems:    "12",
				CPUShares:     20,
				CPUQuota:      10,
				CPUPeriod:     30,
				Memory:        256,
				MemorySwap:    512,
				ShmSize:       10,
				CgroupParent:  "cgroup_parent",
				Dockerfile:    "Dockerfile",
			},
			expectedQueryParams: map[string]string{
				"remote":       "remoteContext",
				"isolation":    "isolation",
				"cpusetcpus":   "2",
				"cpusetmems":   "12",
				"cpushares":    "20",
				"cpuquota":     "10",
				"cpuperiod":    "30",
				"memory":       "256",
				"memswap":      "512",
				"shmsize":      "10",
				"cgroupparent": "cgroup_parent",
				"dockerfile":   "Dockerfile",
				"rm":           "0",
			},
			expectedTags:           []string{},
			expectedRegistryConfig: emptyRegistryConfig,
		},
		{
			buildOptions: build.ImageBuildOptions{
				BuildArgs: map[string]*string{
					"ARG1": &v1,
					"ARG2": &v2,
					"ARG3": nil,
				},
			},
			expectedQueryParams: map[string]string{
				"buildargs": `{"ARG1":"value1","ARG2":"value2","ARG3":null}`,
				"rm":        "0",
			},
			expectedTags:           []string{},
			expectedRegistryConfig: emptyRegistryConfig,
		},
		{
			buildOptions: build.ImageBuildOptions{
				Ulimits: []*container.Ulimit{
					{
						Name: "nproc",
						Hard: 65557,
						Soft: 65557,
					},
					{
						Name: "nofile",
						Hard: 20000,
						Soft: 40000,
					},
				},
			},
			expectedQueryParams: map[string]string{
				"ulimits": `[{"Name":"nproc","Hard":65557,"Soft":65557},{"Name":"nofile","Hard":20000,"Soft":40000}]`,
				"rm":      "0",
			},
			expectedTags:           []string{},
			expectedRegistryConfig: emptyRegistryConfig,
		},
		{
			buildOptions: build.ImageBuildOptions{
				AuthConfigs: map[string]registry.AuthConfig{
					"https://index.docker.io/v1/": {
						Auth: "dG90bwo=",
					},
				},
			},
			expectedQueryParams: map[string]string{
				"rm": "0",
			},
			expectedTags:           []string{},
			expectedRegistryConfig: "eyJodHRwczovL2luZGV4LmRvY2tlci5pby92MS8iOnsiYXV0aCI6ImRHOTBid289In19",
		},
	}
	for _, buildCase := range buildCases {
		expectedURL := "/build"
		client := &Client{
			client: newMockClient(func(r *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(r.URL.Path, expectedURL) {
					return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, r.URL)
				}
				// Check request headers
				registryConfig := r.Header.Get("X-Registry-Config")
				if registryConfig != buildCase.expectedRegistryConfig {
					return nil, fmt.Errorf("X-Registry-Config header not properly set in the request. Expected '%s', got %s", buildCase.expectedRegistryConfig, registryConfig)
				}
				contentType := r.Header.Get("Content-Type")
				if contentType != "application/x-tar" {
					return nil, fmt.Errorf("Content-type header not properly set in the request. Expected 'application/x-tar', got %s", contentType)
				}

				// Check query parameters
				query := r.URL.Query()
				for key, expected := range buildCase.expectedQueryParams {
					actual := query.Get(key)
					if actual != expected {
						return nil, fmt.Errorf("%s not set in URL query properly. Expected '%s', got %s", key, expected, actual)
					}
				}

				// Check tags
				if len(buildCase.expectedTags) > 0 {
					tags := query["t"]
					if !reflect.DeepEqual(tags, buildCase.expectedTags) {
						return nil, fmt.Errorf("t (tags) not set in URL query properly. Expected '%s', got %s", buildCase.expectedTags, tags)
					}
				}

				headers := http.Header{}
				headers.Add("Ostype", "MyOS")
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte("body"))),
					Header:     headers,
				}, nil
			}),
		}
		buildResponse, err := client.ImageBuild(context.Background(), nil, buildCase.buildOptions)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(buildResponse.OSType, "MyOS"))
		response, err := io.ReadAll(buildResponse.Body)
		assert.NilError(t, err)
		buildResponse.Body.Close()
		assert.Check(t, is.Equal(string(response), "body"))
	}
}
