package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageSaveError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ImageSave(context.Background(), []string{"nothing"})
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestImageSave(t *testing.T) {
	expectedURL := "/images/get"
	client := &Client{
		client: newMockClient(func(r *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(r.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, r.URL)
			}
			query := r.URL.Query()
			names := query["names"]
			expectedNames := []string{"image_id1", "image_id2"}
			if !reflect.DeepEqual(names, expectedNames) {
				return nil, fmt.Errorf("names not set in URL query properly. Expected %v, got %v", names, expectedNames)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("response"))),
			}, nil
		}),
	}
	saveResponse, err := client.ImageSave(context.Background(), []string{"image_id1", "image_id2"})
	if err != nil {
		t.Fatal(err)
	}
	response, err := io.ReadAll(saveResponse)
	if err != nil {
		t.Fatal(err)
	}
	saveResponse.Close()
	if string(response) != "response" {
		t.Fatalf("expected response to contain 'response', got %s", string(response))
	}
}
