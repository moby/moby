package image

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/moby/moby/client/pkg/versions"
	"github.com/moby/moby/v2/internal/testutil/request"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func TestImageListIdentity(t *testing.T) {
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.53"), "requires API version 1.53 or newer")

	ctx := setupTest(t)

	withoutIdentity := imageListRaw(t, ctx, "/v1.53/images/json")
	for _, img := range withoutIdentity {
		_, has := img["Identity"]
		assert.Check(t, !has, "Identity should not be present unless identity=1 is requested")
	}

	withIdentity := imageListRaw(t, ctx, "/v1.53/images/json?identity=1")
	foundIdentity := false
	for _, img := range withIdentity {
		identity, has := img["Identity"]
		if !has {
			continue
		}
		foundIdentity = true
		assert.Check(t, identity != nil)
		_, isObject := identity.(map[string]any)
		assert.Check(t, isObject, "Identity should be a JSON object when present")
	}
	if !foundIdentity {
		t.Skip("no images with identity metadata were available in this environment")
	}
}

func TestImageInspectIdentity(t *testing.T) {
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.53"), "requires API version 1.53 or newer")

	ctx := setupTest(t)

	withIdentity := imageListRaw(t, ctx, "/v1.53/images/json?identity=1")
	imageID := ""
	for _, img := range withIdentity {
		if _, has := img["Identity"]; !has {
			continue
		}
		id, _ := img["Id"].(string)
		if id == "" {
			continue
		}
		imageID = id
		break
	}
	if imageID == "" {
		t.Skip("no image with identity metadata found to validate inspect response")
	}

	imagePath := url.PathEscape(imageID)
	current := imageInspectRaw(t, ctx, fmt.Sprintf("/v1.53/images/%s/json", imagePath))
	_, hasCurrent := current["Identity"]
	assert.Check(t, hasCurrent, "Identity should be present in API 1.53 image inspect response")
}

func imageListRaw(t *testing.T, ctx context.Context, endpoint string) []map[string]any {
	t.Helper()

	resp, body, err := request.Get(ctx, endpoint, request.JSON)
	assert.NilError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	buf, err := request.ReadBody(body)
	assert.NilError(t, err)

	var images []map[string]any
	assert.NilError(t, json.Unmarshal(buf, &images), string(buf))
	return images
}

func imageInspectRaw(t *testing.T, ctx context.Context, endpoint string) map[string]any {
	t.Helper()

	resp, body, err := request.Get(ctx, endpoint, request.JSON)
	assert.NilError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	buf, err := request.ReadBody(body)
	assert.NilError(t, err)

	var image map[string]any
	assert.NilError(t, json.Unmarshal(buf, &image), string(buf))
	return image
}
