package mounts // import "github.com/docker/docker/volume/mounts"

import (
	"fmt"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/mount"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestLinuxParseMountRaw(t *testing.T) {
	valid := []string{
		"/home",
		"/home:/home",
		"/home:/something/else",
		"/with space",
		"/home:/with space",
		"relative:/absolute-path",
		"hostPath:/containerPath:ro",
		"/hostPath:/containerPath:rw",
		"/rw:/ro",
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

	invalid := map[string]string{
		"":                                "invalid volume specification",
		"./":                              "mount path must be absolute",
		"../":                             "mount path must be absolute",
		"/:../":                           "mount path must be absolute",
		"/:path":                          "mount path must be absolute",
		":":                               "invalid volume specification",
		"/tmp:":                           "invalid volume specification",
		":test":                           "invalid volume specification",
		":/test":                          "invalid volume specification",
		"tmp:":                            "invalid volume specification",
		":test:":                          "invalid volume specification",
		"::":                              "invalid volume specification",
		":::":                             "invalid volume specification",
		"/tmp:::":                         "invalid volume specification",
		":/tmp::":                         "invalid volume specification",
		"/path:rw":                        "invalid volume specification",
		"/path:ro":                        "invalid volume specification",
		"/rw:rw":                          "invalid volume specification",
		"path:ro":                         "invalid volume specification",
		"/path:/path:sw":                  `invalid mode`,
		"/path:/path:rwz":                 `invalid mode`,
		"/path:/path:ro,rshared,rslave":   `invalid mode`,
		"/path:/path:ro,z,rshared,rslave": `invalid mode`,
		"/path:shared":                    "invalid volume specification",
		"/path:slave":                     "invalid volume specification",
		"/path:private":                   "invalid volume specification",
		"name:/absolute-path:shared":      "invalid volume specification",
		"name:/absolute-path:rshared":     "invalid volume specification",
		"name:/absolute-path:slave":       "invalid volume specification",
		"name:/absolute-path:rslave":      "invalid volume specification",
		"name:/absolute-path:private":     "invalid volume specification",
		"name:/absolute-path:rprivate":    "invalid volume specification",
	}

	parser := NewLinuxParser()
	if p, ok := parser.(*linuxParser); ok {
		p.fi = mockFiProvider{}
	}

	for _, path := range valid {
		if _, err := parser.ParseMountRaw(path, "local"); err != nil {
			t.Errorf("ParseMountRaw(%q) should succeed: error %q", path, err)
		}
	}

	for path, expectedError := range invalid {
		if mp, err := parser.ParseMountRaw(path, "local"); err == nil {
			t.Errorf("ParseMountRaw(%q) should have failed validation. Err '%v' - MP: %v", path, err, mp)
		} else {
			if !strings.Contains(err.Error(), expectedError) {
				t.Errorf("ParseMountRaw(%q) error should contain %q, got %v", path, expectedError, err.Error())
			}
		}
	}
}

func TestLinuxParseMountRawSplit(t *testing.T) {
	tests := []struct {
		bind     string
		driver   string
		expected *MountPoint
		expErr   string
	}{
		{
			bind: "/tmp:/tmp1",
			expected: &MountPoint{
				Source:      "/tmp",
				Destination: "/tmp1",
				RW:          true,
				Type:        mount.TypeBind,
				Propagation: "rprivate",
				Spec: mount.Mount{
					Source:   "/tmp",
					Target:   "/tmp1",
					ReadOnly: false,
					Type:     mount.TypeBind,
				},
			},
		},
		{
			bind: "/tmp:/tmp2:ro",
			expected: &MountPoint{
				Source:      "/tmp",
				Destination: "/tmp2",
				RW:          false,
				Type:        mount.TypeBind,
				Mode:        "ro",
				Propagation: "rprivate",
				Spec: mount.Mount{
					Source:   "/tmp",
					Target:   "/tmp2",
					ReadOnly: true,
					Type:     mount.TypeBind,
				},
			},
		},
		{
			bind: "/tmp:/tmp3:rw",
			expected: &MountPoint{
				Source:      "/tmp",
				Destination: "/tmp3",
				RW:          true,
				Type:        mount.TypeBind,
				Mode:        "rw",
				Propagation: "rprivate",
				Spec: mount.Mount{
					Source:   "/tmp",
					Target:   "/tmp3",
					ReadOnly: false,
					Type:     mount.TypeBind,
				},
			},
		},
		{
			bind:   "/tmp:/tmp4:foo",
			expErr: `invalid mode: foo`,
		},
		{
			bind: "name:/named1",
			expected: &MountPoint{
				Destination: "/named1",
				RW:          true,
				Name:        "name",
				Type:        mount.TypeVolume,
				Mode:        "", // FIXME(thaJeztah): why is this different than an explicit "rw" ?
				Propagation: "",
				CopyData:    true,
				Spec: mount.Mount{
					Source:   "name",
					Target:   "/named1",
					ReadOnly: false,
					Type:     mount.TypeVolume,
				},
			},
		},
		{
			bind:   "name:/named2",
			driver: "external",
			expected: &MountPoint{
				Destination: "/named2",
				RW:          true,
				Name:        "name",
				Driver:      "external",
				Type:        mount.TypeVolume,
				Mode:        "", // FIXME(thaJeztah): why is this different than an explicit "rw" ?
				Propagation: "",
				CopyData:    true,
				Spec: mount.Mount{
					Source:        "name",
					Target:        "/named2",
					ReadOnly:      false,
					Type:          mount.TypeVolume,
					VolumeOptions: &mount.VolumeOptions{DriverConfig: &mount.Driver{Name: "external"}},
				},
			},
		},
		{
			bind:   "name:/named3:ro",
			driver: "local",
			expected: &MountPoint{
				Destination: "/named3",
				RW:          false,
				Name:        "name",
				Driver:      "local",
				Type:        mount.TypeVolume,
				Mode:        "ro",
				Propagation: "",
				CopyData:    true,
				Spec: mount.Mount{
					Source:        "name",
					Target:        "/named3",
					ReadOnly:      true,
					Type:          mount.TypeVolume,
					VolumeOptions: &mount.VolumeOptions{DriverConfig: &mount.Driver{Name: "local"}},
				},
			},
		},
		{
			bind: "local/name:/tmp:rw",
			expected: &MountPoint{
				Destination: "/tmp",
				RW:          true,
				Name:        "local/name",
				Type:        mount.TypeVolume,
				Mode:        "rw",
				Propagation: "",
				CopyData:    true,
				Spec: mount.Mount{
					Source:   "local/name",
					Target:   "/tmp",
					ReadOnly: false,
					Type:     mount.TypeVolume,
				},
			},
		},
		{
			bind:   "/tmp:tmp",
			expErr: `invalid volume specification: '/tmp:tmp': invalid mount config for type "bind": invalid mount path: 'tmp' mount path must be absolute`,
		},
	}

	parser := NewLinuxParser()
	if p, ok := parser.(*linuxParser); ok {
		p.fi = mockFiProvider{}
	}

	for _, tc := range tests {
		t.Run(tc.bind, func(t *testing.T) {
			m, err := parser.ParseMountRaw(tc.bind, tc.driver)
			if tc.expErr != "" {
				assert.Check(t, is.Nil(m))
				assert.Check(t, is.Error(err, tc.expErr))
				return
			}

			assert.NilError(t, err)
			assert.Check(t, is.DeepEqual(*m, *tc.expected, cmpopts.IgnoreUnexported(MountPoint{})))
		})
	}
}

// TestLinuxParseMountSpecBindWithFileinfoError makes sure that the parser returns
// the error produced by the fileinfo provider.
//
// Some extra context for the future in case of changes and possible wtf are we
// testing this for:
//
// Currently this "fileInfoProvider" returns (bool, bool, error)
// The 1st bool is "does this path exist"
// The 2nd bool is "is this path a dir"
// Then of course the error is an error.
//
// The issue is the parser was ignoring the error and only looking at the
// "does this path exist" boolean, which is always false if there is an error.
// Then the error returned to the caller was a (slightly, maybe) friendlier
// error string than what comes from `os.Stat`
// So ...the caller was always getting an error saying the path doesn't exist
// even if it does exist but got some other error (like a permission error).
// This is confusing to users.
func TestLinuxParseMountSpecBindWithFileinfoError(t *testing.T) {
	parser := NewLinuxParser()
	testErr := fmt.Errorf("some crazy error")
	if pr, ok := parser.(*linuxParser); ok {
		pr.fi = &mockFiProviderWithError{err: testErr}
	}

	_, err := parser.ParseMountSpec(mount.Mount{
		Type:   mount.TypeBind,
		Source: `/bananas`,
		Target: `/bananas`,
	})
	assert.ErrorContains(t, err, testErr.Error())
}

func TestConvertTmpfsOptions(t *testing.T) {
	type testCase struct {
		opt                  mount.TmpfsOptions
		readOnly             bool
		expectedSubstrings   []string
		unexpectedSubstrings []string
		err                  bool
	}
	cases := []testCase{
		{
			opt:                  mount.TmpfsOptions{SizeBytes: 1024 * 1024, Mode: 0o700},
			readOnly:             false,
			expectedSubstrings:   []string{"size=1m", "mode=700"},
			unexpectedSubstrings: []string{"ro"},
		},
		{
			opt:                  mount.TmpfsOptions{},
			readOnly:             true,
			expectedSubstrings:   []string{"ro"},
			unexpectedSubstrings: []string{},
		},
		{
			opt:                  mount.TmpfsOptions{Options: [][]string{{"exec"}}},
			readOnly:             true,
			expectedSubstrings:   []string{"ro", "exec"},
			unexpectedSubstrings: []string{"noexec"},
		},
		{
			opt: mount.TmpfsOptions{Options: [][]string{{"INVALID"}}},
			err: true,
		},
	}
	p := NewLinuxParser()
	for _, tc := range cases {
		opt := tc.opt
		data, err := p.ConvertTmpfsOptions(&opt, tc.readOnly)
		if tc.err {
			if err == nil {
				t.Fatalf("expected error for %+v, got nil", opt)
			}
			continue
		}
		if err != nil {
			t.Fatalf("could not convert %+v (readOnly: %v) to string: %v",
				tc.opt, tc.readOnly, err)
		}
		t.Logf("data=%q", data)
		for _, s := range tc.expectedSubstrings {
			if !strings.Contains(data, s) {
				t.Fatalf("expected substring: %s, got %v (case=%+v)", s, data, tc)
			}
		}
		for _, s := range tc.unexpectedSubstrings {
			if strings.Contains(data, s) {
				t.Fatalf("unexpected substring: %s, got %v (case=%+v)", s, data, tc)
			}
		}
	}
}
