package mounts // import "github.com/docker/docker/volume/mounts"

import (
	"strings"
	"testing"

	"github.com/docker/docker/api/types/mount"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestLCOWParseMountRaw(t *testing.T) {
	valid := []string{
		`/foo`,
		`/foo/`,
		`/foo bar`,
		`c:\:/foo`,
		`c:\windows\:/foo`,
		`c:\windows:/s p a c e`,
		`c:\windows:/s p a c e:RW`,
		`c:\program files:/s p a c e i n h o s t d i r`,
		`0123456789name:/foo`,
		`MiXeDcAsEnAmE:/foo`,
		`name:/foo`,
		`name:/foo:rW`,
		`name:/foo:RW`,
		`name:/foo:RO`,
		`c:/:/forward/slashes/are/good/too`,
		`c:/:/including with/spaces:ro`,
		`/Program Files (x86)`, // With capitals and brackets
	}

	invalid := map[string]string{
		``:                                   "invalid volume specification: ",
		`.`:                                  "invalid volume specification: ",
		`c:`:                                 "invalid volume specification: ",
		`c:\`:                                "invalid volume specification: ",
		`../`:                                "invalid volume specification: ",
		`c:\:../`:                            "invalid volume specification: ",
		`c:\:/foo:xyzzy`:                     "invalid volume specification: ",
		`/`:                                  "destination can't be '/'",
		`/..`:                                "destination can't be '/'",
		`c:\notexist:/foo`:                   `source path does not exist: c:\notexist`,
		`c:\windows\system32\ntdll.dll:/foo`: `source path must be a directory`,
		`name<:/foo`:                         `invalid volume specification`,
		`name>:/foo`:                         `invalid volume specification`,
		`name::/foo`:                         `invalid volume specification`,
		`name":/foo`:                         `invalid volume specification`,
		`name\:/foo`:                         `invalid volume specification`,
		`name*:/foo`:                         `invalid volume specification`,
		`name|:/foo`:                         `invalid volume specification`,
		`name?:/foo`:                         `invalid volume specification`,
		`name/:/foo`:                         `invalid volume specification`,
		`/foo:rw`:                            `invalid volume specification`,
		`/foo:ro`:                            `invalid volume specification`,
		`con:/foo`:                           `cannot be a reserved word for Windows filenames`,
		`PRN:/foo`:                           `cannot be a reserved word for Windows filenames`,
		`aUx:/foo`:                           `cannot be a reserved word for Windows filenames`,
		`nul:/foo`:                           `cannot be a reserved word for Windows filenames`,
		`com1:/foo`:                          `cannot be a reserved word for Windows filenames`,
		`com2:/foo`:                          `cannot be a reserved word for Windows filenames`,
		`com3:/foo`:                          `cannot be a reserved word for Windows filenames`,
		`com4:/foo`:                          `cannot be a reserved word for Windows filenames`,
		`com5:/foo`:                          `cannot be a reserved word for Windows filenames`,
		`com6:/foo`:                          `cannot be a reserved word for Windows filenames`,
		`com7:/foo`:                          `cannot be a reserved word for Windows filenames`,
		`com8:/foo`:                          `cannot be a reserved word for Windows filenames`,
		`com9:/foo`:                          `cannot be a reserved word for Windows filenames`,
		`lpt1:/foo`:                          `cannot be a reserved word for Windows filenames`,
		`lpt2:/foo`:                          `cannot be a reserved word for Windows filenames`,
		`lpt3:/foo`:                          `cannot be a reserved word for Windows filenames`,
		`lpt4:/foo`:                          `cannot be a reserved word for Windows filenames`,
		`lpt5:/foo`:                          `cannot be a reserved word for Windows filenames`,
		`lpt6:/foo`:                          `cannot be a reserved word for Windows filenames`,
		`lpt7:/foo`:                          `cannot be a reserved word for Windows filenames`,
		`lpt8:/foo`:                          `cannot be a reserved word for Windows filenames`,
		`lpt9:/foo`:                          `cannot be a reserved word for Windows filenames`,
		`\\.\pipe\foo:/foo`:                  `Linux containers on Windows do not support named pipe mounts`,
	}

	parser := NewLCOWParser()
	if p, ok := parser.(*lcowParser); ok {
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

func TestLCOWParseMountRawSplit(t *testing.T) {
	cases := []struct {
		bind     string
		driver   string
		expected *MountPoint
		expErr   string
	}{
		{
			bind:   `c:\:/foo`,
			driver: "local",
			expected: &MountPoint{
				Source:      `c:\`,
				Destination: "/foo",
				RW:          true,
				Type:        mount.TypeBind,
				Propagation: "", // Propagation is not set on LCOW.
				Spec: mount.Mount{
					Source:   `c:\`,
					Target:   "/foo",
					ReadOnly: false,
					Type:     mount.TypeBind,
				},
			},
		},
		{
			bind:   `c:\:/foo:ro`,
			driver: "local",
			expected: &MountPoint{
				Source:      `c:\`,
				Destination: "/foo",
				RW:          false,
				Type:        mount.TypeBind,
				Mode:        "ro",
				Propagation: "", // Propagation is not set on LCOW.
				Spec: mount.Mount{
					Source:   `c:\`,
					Target:   "/foo",
					ReadOnly: true,
					Type:     mount.TypeBind,
				},
			},
		},
		{
			bind:   `c:\:/foo:rw`,
			driver: "local",
			expected: &MountPoint{
				Source:      `c:\`,
				Destination: "/foo",
				RW:          true,
				Type:        mount.TypeBind,
				Mode:        "rw",
				Propagation: "", // Propagation is not set on LCOW.
				Spec: mount.Mount{
					Source:   `c:\`,
					Target:   "/foo",
					ReadOnly: false,
					Type:     mount.TypeBind,
				},
			},
		},
		{
			bind:   `c:\:/foo:foo`,
			driver: "local",
			expErr: `invalid volume specification: 'c:\:/foo:foo'`,
		},
		{
			bind:   `name:/foo:rw`,
			driver: "local",
			expected: &MountPoint{
				Destination: "/foo",
				RW:          true,
				Name:        "name",
				Driver:      "local",
				Type:        mount.TypeVolume,
				Mode:        "rw",
				Propagation: "", // Propagation is not set on LCOW.
				Spec: mount.Mount{
					Source:        `name`,
					Target:        "/foo",
					ReadOnly:      false,
					Type:          mount.TypeVolume,
					VolumeOptions: &mount.VolumeOptions{DriverConfig: &mount.Driver{Name: "local"}},
				},
			},
		},
		{
			bind:   `name:/foo`,
			driver: "local",
			expected: &MountPoint{
				Destination: "/foo",
				RW:          true,
				Name:        "name",
				Driver:      "local",
				Type:        mount.TypeVolume,
				Mode:        "", // FIXME(thaJeztah): why is this different than an explicit "rw" ?
				Propagation: "", // Propagation is not set on LCOW.
				Spec: mount.Mount{
					Source:        `name`,
					Target:        "/foo",
					ReadOnly:      false,
					Type:          mount.TypeVolume,
					VolumeOptions: &mount.VolumeOptions{DriverConfig: &mount.Driver{Name: "local"}},
				},
			},
		},
		{
			bind:   `name:/foo:ro`,
			driver: "local",
			expected: &MountPoint{
				Destination: "/foo",
				RW:          false,
				Name:        "name",
				Driver:      "local",
				Type:        mount.TypeVolume,
				Mode:        "ro",
				Propagation: "", // Propagation is not set on LCOW.
				Spec: mount.Mount{
					Source:        `name`,
					Target:        "/foo",
					ReadOnly:      true,
					Type:          mount.TypeVolume,
					VolumeOptions: &mount.VolumeOptions{DriverConfig: &mount.Driver{Name: "local"}},
				},
			},
		},
		{
			bind:   `name:/`,
			expErr: `invalid volume specification: 'name:/': invalid mount config for type "volume": invalid specification: destination can't be '/'`,
		},
		{
			bind:   `driver/name:/`,
			expErr: `invalid volume specification: 'driver/name:/'`,
		},
		{
			bind:   `\\.\pipe\foo:\\.\pipe\bar`,
			driver: "local",
			expErr: `invalid volume specification: '\\.\pipe\foo:\\.\pipe\bar'`,
		},
		{
			bind:   `\\.\pipe\foo:/data`,
			driver: "local",
			expErr: `invalid volume specification: '\\.\pipe\foo:/data': invalid mount config for type "npipe": Linux containers on Windows do not support named pipe mounts`,
		},
		{
			bind:   `c:\foo\bar:\\.\pipe\foo`,
			driver: "local",
			expErr: `invalid volume specification: 'c:\foo\bar:\\.\pipe\foo'`,
		},
	}

	parser := NewLCOWParser()
	if p, ok := parser.(*lcowParser); ok {
		p.fi = mockFiProvider{}
	}

	for _, tc := range cases {
		tc := tc
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
