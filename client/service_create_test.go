package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	registrytypes "github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/api/types/swarm"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestServiceCreateError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	_, err = client.ServiceCreate(t.Context(), ServiceCreateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

// TestServiceCreateConnectionError verifies that connection errors occurring
// during API-version negotiation are not shadowed by API-version errors.
//
// Regression test for https://github.com/docker/cli/issues/4890
func TestServiceCreateConnectionError(t *testing.T) {
	client, err := New(WithAPIVersionNegotiation(), WithHost("tcp://no-such-host.invalid"))
	assert.NilError(t, err)

	_, err = client.ServiceCreate(t.Context(), ServiceCreateOptions{})
	assert.Check(t, is.ErrorType(err, IsErrConnectionFailed))
}

func TestServiceCreate(t *testing.T) {
	const expectedURL = "/services/create"
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		return mockJSONResponse(http.StatusOK, nil, swarm.ServiceCreateResponse{
			ID: "service_id",
		})(req)
	}))
	assert.NilError(t, err)

	r, err := client.ServiceCreate(t.Context(), ServiceCreateOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(r.ID, "service_id"))
}

func TestServiceCreateCompatiblePlatforms(t *testing.T) {
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if strings.HasPrefix(req.URL.Path, defaultAPIPath+"/services/create") {
			var serviceSpec swarm.ServiceSpec

			// check if the /distribution endpoint returned correct output
			err := json.NewDecoder(req.Body).Decode(&serviceSpec)
			if err != nil {
				return nil, err
			}

			assert.Check(t, is.Equal("foobar:1.0@sha256:c0537ff6a5218ef531ece93d4984efc99bbf3f7497c0a7726c88e2bb7584dc96", serviceSpec.TaskTemplate.ContainerSpec.Image))
			assert.Check(t, is.Len(serviceSpec.TaskTemplate.Placement.Platforms, 1))

			p := serviceSpec.TaskTemplate.Placement.Platforms[0]
			return mockJSONResponse(http.StatusOK, nil, swarm.ServiceCreateResponse{
				ID: "service_" + p.OS + "_" + p.Architecture,
			})(req)
		} else if strings.HasPrefix(req.URL.Path, defaultAPIPath+"/distribution/") {
			return mockJSONResponse(http.StatusOK, nil, registrytypes.DistributionInspect{
				Descriptor: ocispec.Descriptor{
					Digest: "sha256:c0537ff6a5218ef531ece93d4984efc99bbf3f7497c0a7726c88e2bb7584dc96",
				},
				Platforms: []ocispec.Platform{
					{
						Architecture: "amd64",
						OS:           "linux",
					},
				},
			})(req)
		} else {
			return nil, fmt.Errorf("unexpected URL '%s'", req.URL.Path)
		}
	}))
	assert.NilError(t, err)

	r, err := client.ServiceCreate(t.Context(), ServiceCreateOptions{
		Spec: swarm.ServiceSpec{
			TaskTemplate: swarm.TaskSpec{
				ContainerSpec: &swarm.ContainerSpec{Image: "foobar:1.0"},
			},
		},
		QueryRegistry: true,
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal("service_linux_amd64", r.ID))
}

func TestServiceCreateDigestPinning(t *testing.T) {
	dgst := "sha256:c0537ff6a5218ef531ece93d4984efc99bbf3f7497c0a7726c88e2bb7584dc96"
	dgstAlt := "sha256:37ffbf3f7497c07584dc9637ffbf3f7497c0758c0537ffbf3f7497c0c88e2bb7"
	serviceCreateImage := ""
	pinByDigestTests := []struct {
		img      string // input image provided by the user
		expected string // expected image after digest pinning
	}{
		// default registry returns familiar string
		{"docker.io/library/alpine", "alpine:latest@" + dgst},
		// provided tag is preserved and digest added
		{"alpine:edge", "alpine:edge@" + dgst},
		// image with provided alternative digest remains unchanged
		{"alpine@" + dgstAlt, "alpine@" + dgstAlt},
		// image with provided tag and alternative digest remains unchanged
		{"alpine:edge@" + dgstAlt, "alpine:edge@" + dgstAlt},
		// image on alternative registry does not result in familiar string
		{"alternate.registry/library/alpine", "alternate.registry/library/alpine:latest@" + dgst},
		// unresolvable image does not get a digest
		{"cannotresolve", "cannotresolve:latest"},
	}

	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if strings.HasPrefix(req.URL.Path, defaultAPIPath+"/services/create") {
			// reset and set image received by the service create endpoint
			serviceCreateImage = ""
			var service swarm.ServiceSpec
			if err := json.NewDecoder(req.Body).Decode(&service); err != nil {
				return nil, errors.New("could not parse service create request")
			}
			serviceCreateImage = service.TaskTemplate.ContainerSpec.Image

			return mockJSONResponse(http.StatusOK, nil, swarm.ServiceCreateResponse{
				ID: "service_id",
			})(req)
		} else if strings.HasPrefix(req.URL.Path, defaultAPIPath+"/distribution/cannotresolve") {
			// unresolvable image
			return nil, errors.New("cannot resolve image")
		} else if strings.HasPrefix(req.URL.Path, defaultAPIPath+"/distribution/") {
			// resolvable images
			return mockJSONResponse(http.StatusOK, nil, registrytypes.DistributionInspect{
				Descriptor: ocispec.Descriptor{
					Digest: digest.Digest(dgst),
				},
			})(req)
		}
		return nil, fmt.Errorf("unexpected URL '%s'", req.URL.Path)
	}))
	assert.NilError(t, err)

	// run pin by digest tests
	for _, p := range pinByDigestTests {
		r, err := client.ServiceCreate(t.Context(), ServiceCreateOptions{
			Spec: swarm.ServiceSpec{
				TaskTemplate: swarm.TaskSpec{
					ContainerSpec: &swarm.ContainerSpec{
						Image: p.img,
					},
				},
			},
			QueryRegistry: true,
		})
		assert.NilError(t, err)
		assert.Check(t, is.Equal(r.ID, "service_id"))
		assert.Check(t, is.Equal(p.expected, serviceCreateImage))
	}
}
