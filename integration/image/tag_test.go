package image

import (
	"fmt"
	"testing"

	"github.com/moby/moby/client"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// tagging a named image in a new unprefixed repo should work
func TestTagUnprefixedRepoByNameOrName(t *testing.T) {
	ctx := setupTest(t)

	apiClient := testEnv.APIClient()

	// By name
	_, err := apiClient.ImageTag(ctx, client.ImageTagOptions{Source: "busybox:latest", Target: "testfoobarbaz"})
	assert.NilError(t, err)

	// By ID
	insp, err := apiClient.ImageInspect(ctx, "busybox")
	assert.NilError(t, err)
	_, err = apiClient.ImageTag(ctx, client.ImageTagOptions{Source: insp.ID, Target: "testfoobarbaz"})
	assert.NilError(t, err)
}

func TestTagUsingDigestAlgorithmAsName(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()
	_, err := apiClient.ImageTag(ctx, client.ImageTagOptions{Source: "busybox:latest", Target: "sha256:sometag"})
	assert.Check(t, is.ErrorContains(err, "refusing to create an ambiguous tag using digest algorithm as name"))
}

// ensure we allow the use of valid tags
func TestTagValidPrefixedRepo(t *testing.T) {
	ctx := setupTest(t)

	apiClient := testEnv.APIClient()

	validRepos := []string{"fooo/bar", "fooaa/test", "foooo:t", "HOSTNAME.DOMAIN.COM:443/foo/bar"}

	for _, repo := range validRepos {
		t.Run(repo, func(t *testing.T) {
			t.Parallel()
			_, err := apiClient.ImageTag(ctx, client.ImageTagOptions{Source: "busybox", Target: repo})
			assert.NilError(t, err)
		})
	}
}

// tag an image with an existed tag name without -f option should work
func TestTagExistedNameWithoutForce(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	_, err := apiClient.ImageTag(ctx, client.ImageTagOptions{Source: "busybox:latest", Target: "busybox:test"})
	assert.NilError(t, err)
}

// ensure tagging using official names works
// ensure all tags result in the same name
func TestTagOfficialNames(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	names := []string{
		"docker.io/busybox",
		"index.docker.io/busybox",
		"library/busybox",
		"docker.io/library/busybox",
		"index.docker.io/library/busybox",
	}

	for _, name := range names {
		t.Run("tag from busybox to "+name, func(t *testing.T) {
			_, err := apiClient.ImageTag(ctx, client.ImageTagOptions{Source: "busybox", Target: name + ":latest"})
			assert.NilError(t, err)

			// ensure we don't have multiple tag names.
			insp, err := apiClient.ImageInspect(ctx, "busybox")
			assert.NilError(t, err)
			// TODO(vvoland): Not sure what's actually being tested here. Is is still doing anything useful?
			assert.Assert(t, !is.Contains(insp.RepoTags, name)().Success())

			_, err = apiClient.ImageTag(ctx, client.ImageTagOptions{Source: name + ":latest", Target: "test-tag-official-names/foobar:latest"})
			assert.NilError(t, err)
		})
	}
}

// ensure tags can not match digests
func TestTagMatchesDigest(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	digest := "busybox@sha256:abcdef76720241213f5303bda7704ec4c2ef75613173910a56fb1b6e20251507"
	// test setting tag fails
	_, err := apiClient.ImageTag(ctx, client.ImageTagOptions{Source: "busybox:latest", Target: digest})
	assert.Check(t, is.ErrorContains(err, "refusing to create a tag with a digest reference"))

	// check that no new image matches the digest
	_, err = apiClient.ImageInspect(ctx, digest)
	assert.Check(t, is.ErrorContains(err, fmt.Sprintf("No such image: %s", digest)))
}
