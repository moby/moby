package tarexport

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestValidateManifest(t *testing.T) {
	cases := map[string]struct {
		manifest    []manifestItem
		valid       bool
		errContains string
	}{
		"nil": {
			manifest:    nil,
			valid:       false,
			errContains: "manifest cannot be null",
		},
		"non-nil": {
			manifest: []manifestItem{},
			valid:    true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := validateManifest(tc.manifest)
			if tc.valid {
				assert.Check(t, is.Nil(err))
			} else {
				assert.Check(t, is.ErrorContains(err, tc.errContains))
			}
		})
	}
}
