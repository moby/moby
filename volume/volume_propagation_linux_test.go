// +build linux

package volume

import (
	"strings"
	"testing"
)

func TestParseMountSpecPropagation(t *testing.T) {
	var (
		valid   []string
		invalid map[string]string
	)

	valid = []string{
		"/hostPath:/containerPath:shared",
		"/hostPath:/containerPath:rshared",
		"/hostPath:/containerPath:slave",
		"/hostPath:/containerPath:rslave",
		"/hostPath:/containerPath:private",
		"/hostPath:/containerPath:rprivate",
		"/hostPath:/containerPath:ro,shared",
		"/hostPath:/containerPath:ro,slave",
		"/hostPath:/containerPath:ro,private",
		"/hostPath:/containerPath:ro,z,shared",
		"/hostPath:/containerPath:ro,Z,slave",
		"/hostPath:/containerPath:Z,ro,slave",
		"/hostPath:/containerPath:slave,Z,ro",
		"/hostPath:/containerPath:Z,slave,ro",
		"/hostPath:/containerPath:slave,ro,Z",
		"/hostPath:/containerPath:rslave,ro,Z",
		"/hostPath:/containerPath:ro,rshared,Z",
		"/hostPath:/containerPath:ro,Z,rprivate",
	}
	invalid = map[string]string{
		"/path:/path:ro,rshared,rslave":   `invalid mode: ro,rshared,rslave`,
		"/path:/path:ro,z,rshared,rslave": `invalid mode: ro,z,rshared,rslave`,
		"/path:shared":                    "Invalid volume specification",
		"/path:slave":                     "Invalid volume specification",
		"/path:private":                   "Invalid volume specification",
		"name:/absolute-path:shared":      "Invalid volume specification",
		"name:/absolute-path:rshared":     "Invalid volume specification",
		"name:/absolute-path:slave":       "Invalid volume specification",
		"name:/absolute-path:rslave":      "Invalid volume specification",
		"name:/absolute-path:private":     "Invalid volume specification",
		"name:/absolute-path:rprivate":    "Invalid volume specification",
	}

	for _, path := range valid {
		if _, err := ParseMountSpec(path, "local"); err != nil {
			t.Fatalf("ParseMountSpec(`%q`) should succeed: error %q", path, err)
		}
	}

	for path, expectedError := range invalid {
		if _, err := ParseMountSpec(path, "local"); err == nil {
			t.Fatalf("ParseMountSpec(`%q`) should have failed validation. Err %v", path, err)
		} else {
			if !strings.Contains(err.Error(), expectedError) {
				t.Fatalf("ParseMountSpec(`%q`) error should contain %q, got %v", path, expectedError, err.Error())
			}
		}
	}
}
