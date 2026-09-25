package safepath

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestIsLocalTo(t *testing.T) {
	for _, tc := range []struct {
		name    string
		subpath string
		result  bool
	}{
		{name: "same", subpath: "/volume", result: true},
		{name: "1 level subpath", subpath: "/volume/sub", result: true},
		{name: "2 level subpath", subpath: "/volume/sub/path", result: true},
		{name: "absolute", subpath: "/etc/passwd", result: false},
		{name: "backtrack", subpath: "/volume/../", result: false},
		{name: "backtrack inside", subpath: "/volume/sub/../", result: true},
		{name: "relative path", subpath: "./rel", result: false},
		{name: "file with dots", subpath: "/volume/file..with.dots", result: true},
		{name: "file starting with dots", subpath: "/volume/..file", result: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := isLocalTo(tc.subpath, "/volume")
			assert.Check(t, is.Equal(result, tc.result))
		})
	}
}
