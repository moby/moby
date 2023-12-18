package specialimage

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
)

type SpecialImageFunc func(string) error

func Load(ctx context.Context, t *testing.T, apiClient client.APIClient, imageFunc SpecialImageFunc) string {
	tempDir := t.TempDir()

	err := imageFunc(tempDir)
	assert.NilError(t, err)

	rc, err := archive.TarWithOptions(tempDir, &archive.TarOptions{})
	assert.NilError(t, err)

	defer rc.Close()

	resp, err := apiClient.ImageLoad(ctx, rc, true)
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

	decoder := json.NewDecoder(resp.Body)
	for {
		var msg jsonmessage.JSONMessage
		err := decoder.Decode(&msg)
		if errors.Is(err, io.EOF) {
			break
		} else {
			assert.NilError(t, err)
		}

		msg.Stream = strings.TrimSpace(msg.Stream)

		if _, imageID, hasID := strings.Cut(msg.Stream, "Loaded image ID: "); hasID {
			return imageID
		}
		if _, imageRef, hasRef := strings.Cut(msg.Stream, "Loaded image: "); hasRef {
			return imageRef
		}
	}

	t.Fatal("failed to read image ID")
	return ""
}
