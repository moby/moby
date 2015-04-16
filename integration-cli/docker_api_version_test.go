package main

import (
	"encoding/json"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/autogen/dockerversion"
)

func TestGetVersion(t *testing.T) {
	_, body, err := sockRequest("GET", "/version", nil)
	if err != nil {
		t.Fatal(err)
	}
	var v types.Version
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatal(err)
	}

	if v.Version != dockerversion.VERSION {
		t.Fatal("Version mismatch")
	}
}
