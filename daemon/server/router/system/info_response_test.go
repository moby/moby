package system

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/system"
)

func TestLegacyFields(t *testing.T) {
	infoResp := &infoResponse{
		Info: &system.Info{
			Containers: 10,
		},
		extraFields: map[string]any{
			"LegacyFoo": false,
			"LegacyBar": true,
		},
	}

	data, err := json.MarshalIndent(infoResp, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if expected := `"LegacyFoo": false`; !strings.Contains(string(data), expected) {
		t.Errorf("legacy fields should contain %s: %s", expected, string(data))
	}
	if expected := `"LegacyBar": true`; !strings.Contains(string(data), expected) {
		t.Errorf("legacy fields should contain %s: %s", expected, string(data))
	}
}
