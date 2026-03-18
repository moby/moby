package netlabel

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestGetIfname(t *testing.T) {
	testcases := []struct {
		name      string
		opts      map[string]any
		expIfname string
	}{
		{
			name:      "nil opts",
			opts:      nil,
			expIfname: "",
		},
		{
			name:      "no ifname",
			opts:      map[string]any{},
			expIfname: "",
		},
		{
			name: "ifname set",
			opts: map[string]any{
				Ifname: "foobar",
			},
			expIfname: "foobar",
		},
		{
			name: "ifname set to empty string",
			opts: map[string]any{
				Ifname: "",
			},
			expIfname: "",
		},
		{
			name: "ifname set to nil",
			opts: map[string]any{
				Ifname: nil,
			},
			expIfname: "",
		},
		{
			name: "ifname set to int",
			opts: map[string]any{
				Ifname: 42,
			},
			expIfname: "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expIfname, GetIfname(tc.opts))
		})
	}
}
