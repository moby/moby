package mounts // import "github.com/docker/docker/volume/mounts"

import (
	"strings"
	"testing"

	"github.com/docker/docker/api/types/mount"
)

func TestLCOWParseMountRaw(t *testing.T) {
	previousProvider := currentFileInfoProvider
	defer func() { currentFileInfoProvider = previousProvider }()
	currentFileInfoProvider = mockFiProvider{}

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

	for _, path := range valid {
		if _, err := parser.ParseMountRaw(path, "local"); err != nil {
			t.Errorf("ParseMountRaw(`%q`) should succeed: error %q", path, err)
		}
	}

	for path, expectedError := range invalid {
		if mp, err := parser.ParseMountRaw(path, "local"); err == nil {
			t.Errorf("ParseMountRaw(`%q`) should have failed validation. Err '%v' - MP: %v", path, err, mp)
		} else {
			if !strings.Contains(err.Error(), expectedError) {
				t.Errorf("ParseMountRaw(`%q`) error should contain %q, got %v", path, expectedError, err.Error())
			}
		}
	}
}

func TestLCOWParseMountRawSplit(t *testing.T) {
	previousProvider := currentFileInfoProvider
	defer func() { currentFileInfoProvider = previousProvider }()
	currentFileInfoProvider = mockFiProvider{}

	cases := []struct {
		bind      string
		driver    string
		expType   mount.Type
		expDest   string
		expSource string
		expName   string
		expDriver string
		expRW     bool
		fail      bool
	}{
		{`c:\:/foo`, "local", mount.TypeBind, `/foo`, `c:\`, ``, "", true, false},
		{`c:\:/foo:ro`, "local", mount.TypeBind, `/foo`, `c:\`, ``, "", false, false},
		{`c:\:/foo:rw`, "local", mount.TypeBind, `/foo`, `c:\`, ``, "", true, false},
		{`c:\:/foo:foo`, "local", mount.TypeBind, `/foo`, `c:\`, ``, "", false, true},
		{`name:/foo:rw`, "local", mount.TypeVolume, `/foo`, ``, `name`, "local", true, false},
		{`name:/foo`, "local", mount.TypeVolume, `/foo`, ``, `name`, "local", true, false},
		{`name:/foo:ro`, "local", mount.TypeVolume, `/foo`, ``, `name`, "local", false, false},
		{`name:/`, "", mount.TypeVolume, ``, ``, ``, "", true, true},
		{`driver/name:/`, "", mount.TypeVolume, ``, ``, ``, "", true, true},
		{`\\.\pipe\foo:\\.\pipe\bar`, "local", mount.TypeNamedPipe, `\\.\pipe\bar`, `\\.\pipe\foo`, "", "", true, true},
		{`\\.\pipe\foo:/data`, "local", mount.TypeNamedPipe, ``, ``, "", "", true, true},
		{`c:\foo\bar:\\.\pipe\foo`, "local", mount.TypeNamedPipe, ``, ``, "", "", true, true},
	}

	parser := NewLCOWParser()
	for i, c := range cases {
		t.Logf("case %d", i)
		m, err := parser.ParseMountRaw(c.bind, c.driver)
		if c.fail {
			if err == nil {
				t.Errorf("Expected error, was nil, for spec %s\n", c.bind)
			}
			continue
		}

		if m == nil || err != nil {
			t.Errorf("ParseMountRaw failed for spec '%s', driver '%s', error '%v'", c.bind, c.driver, err)
			continue
		}

		if m.Destination != c.expDest {
			t.Errorf("Expected destination '%s, was %s', for spec '%s'", c.expDest, m.Destination, c.bind)
		}

		if m.Source != c.expSource {
			t.Errorf("Expected source '%s', was '%s', for spec '%s'", c.expSource, m.Source, c.bind)
		}

		if m.Name != c.expName {
			t.Errorf("Expected name '%s', was '%s' for spec '%s'", c.expName, m.Name, c.bind)
		}

		if m.Driver != c.expDriver {
			t.Errorf("Expected driver '%s', was '%s', for spec '%s'", c.expDriver, m.Driver, c.bind)
		}

		if m.RW != c.expRW {
			t.Errorf("Expected RW '%v', was '%v' for spec '%s'", c.expRW, m.RW, c.bind)
		}
		if m.Type != c.expType {
			t.Fatalf("Expected type '%s', was '%s', for spec '%s'", c.expType, m.Type, c.bind)
		}
	}
}
