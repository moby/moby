package client

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageTagError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.ImageTag(context.Background(), ImageTagOptions{Source: "image_id", Target: "repo:tag"})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

// Note: this is not testing all the InvalidReference as it's the responsibility
// of distribution/reference package.
func TestImageTagInvalidReference(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.ImageTag(context.Background(), ImageTagOptions{Source: "image_id", Target: "aa/asdf$$^/aa"})
	assert.Check(t, is.Error(err, `error parsing reference: "aa/asdf$$^/aa" is not a valid repository/tag: invalid reference format`))
}

// Ensure we don't allow the use of invalid repository names or tags; these tag operations should fail.
func TestImageTagInvalidSourceImageName(t *testing.T) {
	ctx := context.Background()

	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "client should not have made an API call")))
	assert.NilError(t, err)

	invalidRepos := []string{"fo$z$", "Foo@3cc", "Foo$3", "Foo*3", "Fo^3", "Foo!3", "F)xcz(", "fo%asd", "aa/asdf$$^/aa"}
	for _, repo := range invalidRepos {
		t.Run("invalidRepo/"+repo, func(t *testing.T) {
			t.Parallel()
			_, err := client.ImageTag(ctx, ImageTagOptions{Source: "busybox", Target: repo})
			assert.Check(t, is.ErrorContains(err, "not a valid repository/tag"))
		})
	}

	longTag := generateRandomAlphaOnlyString(121)
	invalidTags := []string{"repo:fo$z$", "repo:Foo@3cc", "repo:Foo$3", "repo:Foo*3", "repo:Fo^3", "repo:Foo!3", "repo:%goodbye", "repo:#hashtagit", "repo:F)xcz(", "repo:-foo", "repo:..", longTag}
	for _, repotag := range invalidTags {
		t.Run("invalidTag/"+repotag, func(t *testing.T) {
			t.Parallel()
			_, err := client.ImageTag(ctx, ImageTagOptions{Source: "busybox", Target: repotag})
			assert.Check(t, is.ErrorContains(err, "not a valid repository/tag"))
		})
	}

	t.Run("test repository name begin with '-'", func(t *testing.T) {
		t.Parallel()
		_, err := client.ImageTag(ctx, ImageTagOptions{Source: "busybox:latest", Target: "-busybox:test"})
		assert.Check(t, is.ErrorContains(err, "error parsing reference"))
	})

	t.Run("test namespace name begin with '-'", func(t *testing.T) {
		t.Parallel()
		_, err := client.ImageTag(ctx, ImageTagOptions{Source: "busybox:latest", Target: "-test/busybox:test"})
		assert.Check(t, is.ErrorContains(err, "error parsing reference"))
	})

	t.Run("test index name begin with '-'", func(t *testing.T) {
		t.Parallel()
		_, err := client.ImageTag(ctx, ImageTagOptions{Source: "busybox:latest", Target: "-index:5000/busybox:test"})
		assert.Check(t, is.ErrorContains(err, "error parsing reference"))
	})
}

func generateRandomAlphaOnlyString(n int) string {
	// make a really long string
	letters := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))] //nolint: gosec // G404: Use of weak random number generator (math/rand instead of crypto/rand)
	}
	return string(b)
}

func TestImageTagHexSource(t *testing.T) {
	client, err := New(WithMockClient(mockResponse(http.StatusOK, nil, "OK")))
	assert.NilError(t, err)

	_, err = client.ImageTag(context.Background(), ImageTagOptions{Source: "0d409d33b27e47423b049f7f863faa08655a8c901749c2b25b93ca67d01a470d", Target: "repo:tag"})
	assert.NilError(t, err)
}

func TestImageTag(t *testing.T) {
	const expectedURL = "/images/image_id/tag"
	tagCases := []struct {
		reference           string
		expectedQueryParams map[string]string
	}{
		{
			reference: "repository:tag1",
			expectedQueryParams: map[string]string{
				"repo": "docker.io/library/repository",
				"tag":  "tag1",
			},
		}, {
			reference: "another_repository:latest",
			expectedQueryParams: map[string]string{
				"repo": "docker.io/library/another_repository",
				"tag":  "latest",
			},
		}, {
			reference: "another_repository",
			expectedQueryParams: map[string]string{
				"repo": "docker.io/library/another_repository",
				"tag":  "latest",
			},
		}, {
			reference: "test/another_repository",
			expectedQueryParams: map[string]string{
				"repo": "docker.io/test/another_repository",
				"tag":  "latest",
			},
		}, {
			reference: "test/another_repository:tag1",
			expectedQueryParams: map[string]string{
				"repo": "docker.io/test/another_repository",
				"tag":  "tag1",
			},
		}, {
			reference: "test/test/another_repository:tag1",
			expectedQueryParams: map[string]string{
				"repo": "docker.io/test/test/another_repository",
				"tag":  "tag1",
			},
		}, {
			reference: "test:5000/test/another_repository:tag1",
			expectedQueryParams: map[string]string{
				"repo": "test:5000/test/another_repository",
				"tag":  "tag1",
			},
		}, {
			reference: "test:5000/test/another_repository",
			expectedQueryParams: map[string]string{
				"repo": "test:5000/test/another_repository",
				"tag":  "latest",
			},
		},
	}
	for _, tagCase := range tagCases {
		client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
				return nil, err
			}
			query := req.URL.Query()
			for key, expected := range tagCase.expectedQueryParams {
				actual := query.Get(key)
				if actual != expected {
					return nil, fmt.Errorf("%s not set in URL query properly. Expected '%s', got %s", key, expected, actual)
				}
			}
			return mockResponse(http.StatusOK, nil, "")(req)
		}))
		assert.NilError(t, err)
		_, err = client.ImageTag(context.Background(), ImageTagOptions{Source: "image_id", Target: tagCase.reference})
		assert.NilError(t, err)
	}
}
