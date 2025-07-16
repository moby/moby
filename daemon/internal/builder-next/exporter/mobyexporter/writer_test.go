package mobyexporter

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestPatchImageConfig(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cfgJSON string
		err     string
	}{
		{
			name:    "empty",
			cfgJSON: "{}",
		},
		{
			name:    "history only",
			cfgJSON: `{"history": []}`,
		},
		{
			name:    "rootfs only",
			cfgJSON: `{"rootfs": {}}`,
		},
		{
			name:    "null",
			cfgJSON: "null",
			err:     "null image config",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := patchImageConfig([]byte(tc.cfgJSON), nil, nil, nil)
			if tc.err == "" {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.err)
			}
		})
	}
}
