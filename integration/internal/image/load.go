package image

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/moby/go-archive"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/jsonmessage"
	"github.com/moby/moby/v2/internal/testutils/specialimage"
	"gotest.tools/v3/assert"
)

func Load(ctx context.Context, t *testing.T, apiClient client.APIClient, imageFunc specialimage.SpecialImageFunc) string {
	tempDir := t.TempDir()

	_, err := imageFunc(tempDir)
	assert.NilError(t, err)

	rc, err := archive.TarWithOptions(tempDir, &archive.TarOptions{})
	assert.NilError(t, err)

	defer rc.Close()

	resp, err := apiClient.ImageLoad(ctx, rc, client.ImageLoadWithQuiet(true))
	assert.NilError(t, err, "Failed to load dangling image")

	defer resp.Body.Close()

	if !assert.Check(t, err) {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
			return ""
		}
		t.Fatalf("Failed load: %s", string(respBody))
	}

	all, err := io.ReadAll(resp.Body)
	assert.NilError(t, err)

	decoder := json.NewDecoder(bytes.NewReader(all))
	for {
		var msg jsonmessage.JSONMessage
		err := decoder.Decode(&msg)
		if errors.Is(err, io.EOF) {
			break
		}
		assert.NilError(t, err)

		msg.Stream = strings.TrimSpace(msg.Stream)

		if _, imageID, hasID := strings.Cut(msg.Stream, "Loaded image ID: "); hasID {
			return imageID
		}
		if _, imageRef, hasRef := strings.Cut(msg.Stream, "Loaded image: "); hasRef {
			return imageRef
		}
	}

	t.Fatalf("failed to read image ID\n%s", string(all))
	return ""
}
