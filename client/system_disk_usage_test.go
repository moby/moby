package client

import (
	"fmt"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/system"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestDiskUsageError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	_, err = client.DiskUsage(t.Context(), DiskUsageOptions{})
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
				ActiveCount: 0,
				TotalCount:  0,
				Reclaimable: 0,
				TotalSize:   4096,
				Items:       []image.Summary{},
			},
		})(req)
	}))
	assert.NilError(t, err)

	du, err := client.DiskUsage(t.Context(), DiskUsageOptions{})
	assert.NilError(t, err)
	assert.Equal(t, du.Images.ActiveCount, int64(0))
	assert.Equal(t, du.Images.TotalCount, int64(0))
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
			client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
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
	const legacyVersion = "1.51"
	const expectedURL = "/system/df"
	client, err := New(
		WithAPIVersion(legacyVersion),
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodGet, "/v"+legacyVersion+expectedURL); err != nil {
				return nil, err
			}

			return mockJSONResponse(http.StatusOK, nil, &legacyDiskUsage{
				LayersSize: 4096,
				Images:     []image.Summary{},
			})(req)
		}))
	assert.NilError(t, err)

	du, err := client.DiskUsage(t.Context(), DiskUsageOptions{})
	assert.NilError(t, err)
	assert.Equal(t, du.Images.ActiveCount, int64(0))
	assert.Equal(t, du.Images.TotalCount, int64(0))
	assert.Equal(t, du.Images.Reclaimable, int64(0))
	assert.Equal(t, du.Images.TotalSize, int64(4096))
	assert.Equal(t, len(du.Images.Items), 0)
}

func TestImageDiskUsageFromLegacyAPI(t *testing.T) {
	const legacyVersion = "1.51"
	const expectedURL = "/system/df"

	tests := []struct {
		name                string
		mockResponse        *legacyDiskUsage
		expectedActiveCount int64
		expectedTotalCount  int64
		expectedReclaimable int64
		expectedTotalSize   int64
	}{
		{
			name: "no images",
			mockResponse: &legacyDiskUsage{
				LayersSize: 0,
				Images:     []image.Summary{},
			},
			expectedActiveCount: 0,
			expectedTotalCount:  0,
			expectedReclaimable: 0,
			expectedTotalSize:   0,
		},
		{
			name: "images with no containers",
			mockResponse: &legacyDiskUsage{
				LayersSize: 8192,
				Images: []image.Summary{
					{ID: "image1", Size: 4096, SharedSize: 0, Containers: 0},
					{ID: "image2", Size: 4096, SharedSize: 0, Containers: 0},
				},
			},
			expectedActiveCount: 0,
			expectedTotalCount:  2,
			expectedReclaimable: 8192,
			expectedTotalSize:   8192,
		},
		{
			name: "images with containers",
			mockResponse: &legacyDiskUsage{
				LayersSize: 12288,
				Images: []image.Summary{
					{ID: "image1", Size: 4096, SharedSize: 0, Containers: 2},
					{ID: "image2", Size: 2048, SharedSize: 0, Containers: 0},
					{ID: "image3", Size: 6144, SharedSize: 0, Containers: 1},
				},
			},
			expectedActiveCount: 2,
			expectedTotalCount:  3,
			expectedReclaimable: 2048,
			expectedTotalSize:   12288,
		},
		{
			name: "images with shared size",
			mockResponse: &legacyDiskUsage{
				LayersSize: 15360,
				Images: []image.Summary{
					{ID: "image1", Size: 4096, SharedSize: 1024, Containers: 1},
					{ID: "image2", Size: 8192, SharedSize: 2048, Containers: 0},
					{ID: "image3", Size: 3072, SharedSize: 1024, Containers: 0},
				},
			},
			expectedActiveCount: 1,
			expectedTotalCount:  3,
			expectedReclaimable: 8192, // (8192-2048) + (3072-1024)
			expectedTotalSize:   15360,
		},
		{
			name: "multiplatform image with an image index",
			mockResponse: &legacyDiskUsage{
				LayersSize: 4608,
				Images: []image.Summary{
					{ID: "image1", Size: 4096, SharedSize: 0, Containers: 0, Descriptor: &ocispec.Descriptor{MediaType: ocispec.MediaTypeImageIndex, Size: 512}},
				},
			},
			expectedActiveCount: 0,
			expectedTotalCount:  1,
			expectedReclaimable: 4608, // (4096 - 0) + 512
			expectedTotalSize:   4608,
		},
		{
			name: "multiplatform image with a Docker distribution manifest",
			mockResponse: &legacyDiskUsage{
				LayersSize: 4096,
				Images: []image.Summary{
					{ID: "image1", Size: 4096, SharedSize: 0, Containers: 0, Descriptor: &ocispec.Descriptor{MediaType: "application/vnd.docker.distribution.manifest.v2+json", Size: 427}},
				},
			},
			expectedActiveCount: 0,
			expectedTotalCount:  1,
			expectedReclaimable: 4096, // (4096 - 0)
			expectedTotalSize:   4096,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(
				WithAPIVersion(legacyVersion),
				WithMockClient(func(req *http.Request) (*http.Response, error) {
					if err := assertRequest(req, http.MethodGet, "/v"+legacyVersion+expectedURL); err != nil {
						return nil, err
					}

					return mockJSONResponse(http.StatusOK, nil, tt.mockResponse)(req)
				}))
			assert.NilError(t, err)

			du, err := client.DiskUsage(t.Context(), DiskUsageOptions{Images: true})
			assert.NilError(t, err)
			assert.Equal(t, du.Images.ActiveCount, tt.expectedActiveCount)
			assert.Equal(t, du.Images.TotalCount, tt.expectedTotalCount)
			assert.Equal(t, du.Images.Reclaimable, tt.expectedReclaimable)
			assert.Equal(t, du.Images.TotalSize, tt.expectedTotalSize)
			assert.Equal(t, len(du.Images.Items), len(tt.mockResponse.Images))
		})
	}
}
