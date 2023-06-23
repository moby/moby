package distribution

import (
	"github.com/docker/distribution/reference"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDistributionRepositoryWithManifestInfo_ModifyRequest(t *testing.T) {
	assertExpectedHeaders := func(t *testing.T, req *http.Request, tag string) {
		var expectedTags []string = nil
		if tag != "" {
			expectedTags = []string{tag}
		}
		tagValues := req.Header.Values("Docker-Manifest-Tag")
		assert.Check(t, cmp.DeepEqual(expectedTags, tagValues), "manifest tag values in header not as expected")
	}

	repo := &distributionRepositoryWithManifestInfo{}
	newRequest := func() *http.Request { return httptest.NewRequest(http.MethodGet, "https://www.example.com", nil) }
	refName, _ := reference.WithName("foo")
	refWithTag1, _ := reference.WithTag(refName, "1.0")

	t.Run("initial values", func(t *testing.T) {
		req := newRequest()
		err := repo.ModifyRequest(req)
		assert.NilError(t, err)
		assertExpectedHeaders(t, req, "")
	})

	t.Run("update ref tag", func(t *testing.T) {
		req := newRequest()
		updateRepoWithManifestInfo(repo, refWithTag1)
		err := repo.ModifyRequest(req)
		assert.NilError(t, err)
		assertExpectedHeaders(t, req, "1.0")
	})
}

func TestMetaHeadersWithManifestTagHeader(t *testing.T) {
	namedRef, _ := reference.WithName("foo")
	taggedRef, _ := reference.WithTag(namedRef, "1.0")
	var metaHeaders map[string][]string

	t.Run("nil meta headers map, ref not tagged", func(t *testing.T) {
		result := metaHeadersWithManifestTagHeader(metaHeaders, namedRef)
		// Original not changed
		assert.Check(t, metaHeaders == nil)
		// Result is the same as original
		assert.DeepEqual(t, metaHeaders, result)
	})

	t.Run("nil meta headers map, tagged ref", func(t *testing.T) {
		result := metaHeadersWithManifestTagHeader(metaHeaders, taggedRef)
		// Original not changed
		assert.Check(t, metaHeaders == nil)
		// Result includes the added header
		assert.DeepEqual(t, result, map[string][]string{"Docker-Manifest-Tag": {"1.0"}})
	})

	metaHeaders = map[string][]string{"foo": {"1"}, "bar": {"2", "3"}}

	t.Run("non-empty meta headers map, ref not tagged", func(t *testing.T) {
		result := metaHeadersWithManifestTagHeader(metaHeaders, namedRef)
		// Original not changed
		assert.DeepEqual(t, metaHeaders, map[string][]string{"foo": {"1"}, "bar": {"2", "3"}})
		// Result is the same as original
		assert.DeepEqual(t, metaHeaders, result)
	})

	t.Run("non-empty meta headers map, tagged ref", func(t *testing.T) {
		result := metaHeadersWithManifestTagHeader(metaHeaders, taggedRef)
		// Original not changed
		assert.DeepEqual(t, metaHeaders, map[string][]string{"foo": {"1"}, "bar": {"2", "3"}})
		// Result is the same as original, plus the additional header
		assert.DeepEqual(t, result, map[string][]string{"foo": {"1"}, "bar": {"2", "3"}, "Docker-Manifest-Tag": {"1.0"}})
	})
}
