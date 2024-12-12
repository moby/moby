package registry

import (
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestServiceConfigMarshalLegacyFields(t *testing.T) {
	t.Run("without legacy fields", func(t *testing.T) {
		b, err := json.Marshal(&ServiceConfig{})
		assert.NilError(t, err)
		const expected = `{"IndexConfigs":null,"InsecureRegistryCIDRs":null,"Mirrors":null}`
		assert.Check(t, is.Equal(string(b), expected), "Legacy nondistributable-artifacts fields should be omitted in output")
	})

	// Legacy fields should be returned when set to an empty slice. This is
	// used for API versions < 1.49.
	t.Run("with legacy fields", func(t *testing.T) {
		b, err := json.Marshal(&ServiceConfig{
			AllowNondistributableArtifactsCIDRs:     []*NetIPNet{},
			AllowNondistributableArtifactsHostnames: []string{},
		})
		assert.NilError(t, err)
		const expected = `{"AllowNondistributableArtifactsCIDRs":null,"AllowNondistributableArtifactsHostnames":null,"IndexConfigs":null,"InsecureRegistryCIDRs":null,"Mirrors":null}`
		assert.Check(t, is.Equal(string(b), expected))
	})
}
