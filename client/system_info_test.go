package client

import (
	"context"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/system"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestInfoServerError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	_, err = client.Info(context.Background(), InfoOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestInfoInvalidResponseJSONError(t *testing.T) {
	client, err := New(WithMockClient(mockResponse(http.StatusOK, nil, "invalid json")))
	assert.NilError(t, err)
	_, err = client.Info(context.Background(), InfoOptions{})
	assert.Check(t, is.ErrorContains(err, "invalid character"))
}

func TestInfo(t *testing.T) {
	const expectedURL = "/info"
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
			return nil, err
		}
		return mockJSONResponse(http.StatusOK, nil, system.Info{
			ID:         "daemonID",
			Containers: 3,
		})(req)
	}))
	assert.NilError(t, err)

	result, err := client.Info(context.Background(), InfoOptions{})
	assert.NilError(t, err)
	info := result.Info

	assert.Check(t, is.Equal(info.ID, "daemonID"))
	assert.Check(t, is.Equal(info.Containers, 3))
}

func TestInfoWithDiscoveredDevices(t *testing.T) {
	const expectedURL = "/info"
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
			return nil, err
		}
		return mockJSONResponse(http.StatusOK, nil, system.Info{
			ID:         "daemonID",
			Containers: 3,
			DiscoveredDevices: []system.DeviceInfo{
				{
					Source: "cdi",
					ID:     "vendor.com/gpu=0",
				},
				{
					Source: "cdi",
					ID:     "vendor.com/gpu=1",
				},
			},
		})(req)
	}))
	assert.NilError(t, err)

	result, err := client.Info(context.Background(), InfoOptions{})
	assert.NilError(t, err)
	info := result.Info

	assert.Check(t, is.Equal(info.ID, "daemonID"))
	assert.Check(t, is.Equal(info.Containers, 3))

	assert.Check(t, is.Len(info.DiscoveredDevices, 2))

	device0 := info.DiscoveredDevices[0]
	assert.Check(t, is.Equal(device0.Source, "cdi"))
	assert.Check(t, is.Equal(device0.ID, "vendor.com/gpu=0"))

	device1 := info.DiscoveredDevices[1]
	assert.Check(t, is.Equal(device1.Source, "cdi"))
	assert.Check(t, is.Equal(device1.ID, "vendor.com/gpu=1"))
}
