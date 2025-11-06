package client

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/system"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestDiskUsageError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	_, err = client.DiskUsage(context.Background(), DiskUsageOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestDiskUsage(t *testing.T) {
	const expectedURL = "/system/df"
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
			return nil, err
		}

		return mockJSONResponse(http.StatusOK, nil, system.DiskUsage{
			ImageUsage: &image.DiskUsage{
				ActiveImages: 0,
				TotalImages:  0,
				Reclaimable:  0,
				TotalSize:    4096,
				Items:        []image.Summary{},
			},
		})(req)
	}))
	assert.NilError(t, err)

	du, err := client.DiskUsage(context.Background(), DiskUsageOptions{})
	assert.NilError(t, err)
	assert.Equal(t, du.Images.ActiveImages, int64(0))
	assert.Equal(t, du.Images.TotalImages, int64(0))
	assert.Equal(t, du.Images.Reclaimable, int64(0))
	assert.Equal(t, du.Images.TotalSize, int64(4096))
	assert.Equal(t, len(du.Images.Items), 0)
}

func TestDiskUsageWithOptions(t *testing.T) {
	const expectedURL = "/system/df"

	tests := []struct {
		options       DiskUsageOptions
		expectedQuery string
	}{
		{
			options: DiskUsageOptions{
				Containers: true,
			},
			expectedQuery: "type=container",
		},
		{
			options: DiskUsageOptions{
				Images: true,
			},
			expectedQuery: "type=image",
		},
		{
			options: DiskUsageOptions{
				Volumes: true,
			},
			expectedQuery: "type=volume",
		},
		{
			options: DiskUsageOptions{
				BuildCache: true,
			},
			expectedQuery: "type=build-cache",
		},
		{
			options: DiskUsageOptions{
				Containers: true,
				Images:     true,
			},
			expectedQuery: "type=container&type=image",
		},
		{
			options: DiskUsageOptions{
				Containers: true,
				Images:     true,
				Volumes:    true,
				BuildCache: true,
			},
			expectedQuery: "type=container&type=image&type=volume&type=build-cache",
		},
		{
			options: DiskUsageOptions{
				Containers: true,
				Verbose:    true,
			},
			expectedQuery: "type=container&verbose=1",
		},
		{
			options: DiskUsageOptions{
				Containers: true,
				Images:     true,
				Volumes:    true,
				BuildCache: true,
				Verbose:    true,
			},
			expectedQuery: "type=container&type=image&type=volume&type=build-cache&verbose=1",
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("options=%+v", tt.options), func(t *testing.T) {
			client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
				if err := assertRequestWithQuery(req, http.MethodGet, expectedURL, tt.expectedQuery); err != nil {
					return nil, err
				}

				return mockJSONResponse(http.StatusOK, nil, system.DiskUsage{})(req)
			}))
			assert.NilError(t, err)
			_, err = client.DiskUsage(t.Context(), tt.options)
			assert.NilError(t, err)
		})
	}
}

func TestLegacyDiskUsage(t *testing.T) {
	const expectedURL = "/system/df"
	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
			return nil, err
		}

		return mockJSONResponse(http.StatusOK, nil, system.DiskUsage{
			LegacyDiskUsage: system.LegacyDiskUsage{
				LayersSize: 4096,
				Images:     []image.Summary{},
			},
		})(req)
	}))
	assert.NilError(t, err)

	du, err := client.DiskUsage(context.Background(), DiskUsageOptions{})
	assert.NilError(t, err)
	assert.Equal(t, du.Images.ActiveImages, int64(0))
	assert.Equal(t, du.Images.TotalImages, int64(0))
	assert.Equal(t, du.Images.Reclaimable, int64(0))
	assert.Equal(t, du.Images.TotalSize, int64(4096))
	assert.Equal(t, len(du.Images.Items), 0)
}
