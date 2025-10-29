package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/registry"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageBuildError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	_, err = client.ImageBuild(context.Background(), nil, ImageBuildOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestImageBuild(t *testing.T) {
	v1 := "value1"
	v2 := "value2"
	emptyRegistryConfig := "bnVsbA=="
	buildCases := []struct {
		buildOptions           ImageBuildOptions
		expectedQueryParams    map[string]string
		expectedTags           []string
		expectedRegistryConfig string
	}{
		{
			buildOptions: ImageBuildOptions{
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
			buildOptions: ImageBuildOptions{
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
			buildOptions: ImageBuildOptions{
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
			buildOptions: ImageBuildOptions{
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
			buildOptions: ImageBuildOptions{
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
			buildOptions: ImageBuildOptions{
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
	const expectedURL = "/build"
	for _, buildCase := range buildCases {
		client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
				return nil, err
			}
			// Check request headers
			registryConfig := req.Header.Get("X-Registry-Config")
			if registryConfig != buildCase.expectedRegistryConfig {
				return nil, fmt.Errorf("X-Registry-Config header not properly set in the request. Expected '%s', got %s", buildCase.expectedRegistryConfig, registryConfig)
			}
			contentType := req.Header.Get("Content-Type")
			if contentType != "application/x-tar" {
				return nil, fmt.Errorf("Content-type header not properly set in the request. Expected 'application/x-tar', got %s", contentType)
			}

			// Check query parameters
			query := req.URL.Query()
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

			return mockResponse(http.StatusOK, nil, "body")(req)
		}))
		assert.NilError(t, err)
		buildResponse, err := client.ImageBuild(context.Background(), nil, buildCase.buildOptions)
		assert.NilError(t, err)
		response, err := io.ReadAll(buildResponse.Body)
		assert.NilError(t, err)
		_ = buildResponse.Body.Close()
		assert.Check(t, is.Equal(string(response), "body"))
	}
}
