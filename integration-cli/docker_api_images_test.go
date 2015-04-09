package main

import (
	"encoding/json"
	"testing"

	"github.com/docker/docker/api/types"
)

func TestLegacyImages(t *testing.T) {
	body, err := sockRequest("GET", "/v1.6/images/json", nil)
	if err != nil {
		t.Fatalf("Error on GET: %s", err)
	}

	images := []types.LegacyImage{}
	if err = json.Unmarshal(body, &images); err != nil {
		t.Fatalf("Error on unmarshal: %s", err)
	}

	if len(images) == 0 || images[0].Tag == "" || images[0].Repository == "" {
		t.Fatalf("Bad data: %q", images)
	}

	logDone("images - checking legacy json")
}
