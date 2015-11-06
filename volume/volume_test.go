package volume

import (
	"runtime"
	"strings"
	"testing"
)

func TestParseMountSpec(t *testing.T) {
	var (
		valid   []string
		invalid map[string]string
	)

	if runtime.GOOS == "windows" {
		valid = []string{
			`d:\`,
			`d:`,
			`d:\path`,
			`d:\path with space`,
			// TODO Windows post TP4 - readonly support `d:\pathandmode:ro`,
			`c:\:d:\`,
			`c:\windows\:d:`,
			`c:\windows:d:\s p a c e`,
			`c:\windows:d:\s p a c e:RW`,
			`c:\program files:d:\s p a c e i n h o s t d i r`,
			`0123456789name:d:`,
			`MiXeDcAsEnAmE:d:`,
			`name:D:`,
			`name:D::rW`,
			`name:D::RW`,
			// TODO Windows post TP4 - readonly support `name:D::RO`,
			`c:/:d:/forward/slashes/are/good/too`,
			// TODO Windows post TP4 - readonly support `c:/:d:/including with/spaces:ro`,
			`c:\Windows`,             // With capital
			`c:\Program Files (x86)`, // With capitals and brackets
		}
		invalid = map[string]string{
			``:                                 "Invalid volume specification: ",
			`.`:                                "Invalid volume specification: ",
			`..\`:                              "Invalid volume specification: ",
			`c:\:..\`:                          "Invalid volume specification: ",
			`c:\:d:\:xyzzy`:                    "Invalid volume specification: ",
			`c:`:                               "cannot be c:",
			`c:\`:                              `cannot be c:\`,
			`c:\notexist:d:`:                   `The system cannot find the file specified`,
			`c:\windows\system32\ntdll.dll:d:`: `Source 'c:\windows\system32\ntdll.dll' is not a directory`,
			`name<:d:`:                         `Invalid volume specification`,
			`name>:d:`:                         `Invalid volume specification`,
			`name::d:`:                         `Invalid volume specification`,
			`name":d:`:                         `Invalid volume specification`,
			`name\:d:`:                         `Invalid volume specification`,
			`name*:d:`:                         `Invalid volume specification`,
			`name|:d:`:                         `Invalid volume specification`,
			`name?:d:`:                         `Invalid volume specification`,
			`name/:d:`:                         `Invalid volume specification`,
			`d:\pathandmode:rw`:                `Invalid volume specification`,
			`con:d:`:                           `cannot be a reserved word for Windows filenames`,
			`PRN:d:`:                           `cannot be a reserved word for Windows filenames`,
			`aUx:d:`:                           `cannot be a reserved word for Windows filenames`,
			`nul:d:`:                           `cannot be a reserved word for Windows filenames`,
			`com1:d:`:                          `cannot be a reserved word for Windows filenames`,
			`com2:d:`:                          `cannot be a reserved word for Windows filenames`,
			`com3:d:`:                          `cannot be a reserved word for Windows filenames`,
			`com4:d:`:                          `cannot be a reserved word for Windows filenames`,
			`com5:d:`:                          `cannot be a reserved word for Windows filenames`,
			`com6:d:`:                          `cannot be a reserved word for Windows filenames`,
			`com7:d:`:                          `cannot be a reserved word for Windows filenames`,
			`com8:d:`:                          `cannot be a reserved word for Windows filenames`,
			`com9:d:`:                          `cannot be a reserved word for Windows filenames`,
			`lpt1:d:`:                          `cannot be a reserved word for Windows filenames`,
			`lpt2:d:`:                          `cannot be a reserved word for Windows filenames`,
			`lpt3:d:`:                          `cannot be a reserved word for Windows filenames`,
			`lpt4:d:`:                          `cannot be a reserved word for Windows filenames`,
			`lpt5:d:`:                          `cannot be a reserved word for Windows filenames`,
			`lpt6:d:`:                          `cannot be a reserved word for Windows filenames`,
			`lpt7:d:`:                          `cannot be a reserved word for Windows filenames`,
			`lpt8:d:`:                          `cannot be a reserved word for Windows filenames`,
			`lpt9:d:`:                          `cannot be a reserved word for Windows filenames`,
		}

	} else {
		valid = []string{
			"/home",
			"/home:/home",
			"/home:/something/else",
			"/with space",
			"/home:/with space",
			"relative:/absolute-path",
			"hostPath:/containerPath:ro",
			"/hostPath:/containerPath:rw",
			"/rw:/ro",
		}
		invalid = map[string]string{
			"":                "Invalid volume specification",
			"./":              "Invalid volume destination",
			"../":             "Invalid volume destination",
			"/:../":           "Invalid volume destination",
			"/:path":          "Invalid volume destination",
			":":               "Invalid volume specification",
			"/tmp:":           "Invalid volume destination",
			":test":           "Invalid volume specification",
			":/test":          "Invalid volume specification",
			"tmp:":            "Invalid volume destination",
			":test:":          "Invalid volume specification",
			"::":              "Invalid volume specification",
			":::":             "Invalid volume specification",
			"/tmp:::":         "Invalid volume specification",
			":/tmp::":         "Invalid volume specification",
			"/path:rw":        "Invalid volume specification",
			"/path:ro":        "Invalid volume specification",
			"/rw:rw":          "Invalid volume specification",
			"path:ro":         "Invalid volume specification",
			"/path:/path:sw":  `invalid mode: "sw"`,
			"/path:/path:rwz": `invalid mode: "rwz"`,
		}
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

func TestSplitN(t *testing.T) {
	for _, x := range []struct {
		input    string
		n        int
		expected []string
	}{
		{`C:\foo:d:`, -1, []string{`C:\foo`, `d:`}},
		{`:C:\foo:d:`, -1, nil},
		{`/foo:/bar:ro`, 3, []string{`/foo`, `/bar`, `ro`}},
		{`/foo:/bar:ro`, 2, []string{`/foo`, `/bar:ro`}},
		{`C:\foo\:/foo`, -1, []string{`C:\foo\`, `/foo`}},

		{`d:\`, -1, []string{`d:\`}},
		{`d:`, -1, []string{`d:`}},
		{`d:\path`, -1, []string{`d:\path`}},
		{`d:\path with space`, -1, []string{`d:\path with space`}},
		{`d:\pathandmode:rw`, -1, []string{`d:\pathandmode`, `rw`}},
		{`c:\:d:\`, -1, []string{`c:\`, `d:\`}},
		{`c:\windows\:d:`, -1, []string{`c:\windows\`, `d:`}},
		{`c:\windows:d:\s p a c e`, -1, []string{`c:\windows`, `d:\s p a c e`}},
		{`c:\windows:d:\s p a c e:RW`, -1, []string{`c:\windows`, `d:\s p a c e`, `RW`}},
		{`c:\program files:d:\s p a c e i n h o s t d i r`, -1, []string{`c:\program files`, `d:\s p a c e i n h o s t d i r`}},
		{`0123456789name:d:`, -1, []string{`0123456789name`, `d:`}},
		{`MiXeDcAsEnAmE:d:`, -1, []string{`MiXeDcAsEnAmE`, `d:`}},
		{`name:D:`, -1, []string{`name`, `D:`}},
		{`name:D::rW`, -1, []string{`name`, `D:`, `rW`}},
		{`name:D::RW`, -1, []string{`name`, `D:`, `RW`}},
		{`c:/:d:/forward/slashes/are/good/too`, -1, []string{`c:/`, `d:/forward/slashes/are/good/too`}},
		{`c:\Windows`, -1, []string{`c:\Windows`}},
		{`c:\Program Files (x86)`, -1, []string{`c:\Program Files (x86)`}},

		{``, -1, nil},
		{`.`, -1, []string{`.`}},
		{`..\`, -1, []string{`..\`}},
		{`c:\:..\`, -1, []string{`c:\`, `..\`}},
		{`c:\:d:\:xyzzy`, -1, []string{`c:\`, `d:\`, `xyzzy`}},
	} {
		res := SplitN(x.input, x.n)
		if len(res) < len(x.expected) {
			t.Fatalf("input: %v, expected: %v, got: %v", x.input, x.expected, res)
		}
		for i, e := range res {
			if e != x.expected[i] {
				t.Fatalf("input: %v, expected: %v, got: %v", x.input, x.expected, res)
			}
		}
	}
}

// testParseMountSpec is a structure used by TestParseMountSpecSplit for
// specifying test cases for the ParseMountSpec() function.
type testParseMountSpec struct {
	bind      string
	driver    string
	expDest   string
	expSource string
	expName   string
	expDriver string
	expRW     bool
	fail      bool
}

func TestParseMountSpecSplit(t *testing.T) {
	var cases []testParseMountSpec
	if runtime.GOOS == "windows" {
		cases = []testParseMountSpec{
			{`c:\:d:`, "local", `d:`, `c:\`, ``, "", true, false},
			{`c:\:d:\`, "local", `d:\`, `c:\`, ``, "", true, false},
			// TODO Windows post TP4 - Add readonly support {`c:\:d:\:ro`, "local", `d:\`, `c:\`, ``, "", false, false},
			{`c:\:d:\:rw`, "local", `d:\`, `c:\`, ``, "", true, false},
			{`c:\:d:\:foo`, "local", `d:\`, `c:\`, ``, "", false, true},
			{`name:d::rw`, "local", `d:`, ``, `name`, "local", true, false},
			{`name:d:`, "local", `d:`, ``, `name`, "local", true, false},
			// TODO Windows post TP4 - Add readonly support {`name:d::ro`, "local", `d:`, ``, `name`, "local", false, false},
			{`name:c:`, "", ``, ``, ``, "", true, true},
			{`driver/name:c:`, "", ``, ``, ``, "", true, true},
		}
	} else {
		cases = []testParseMountSpec{
			{"/tmp:/tmp1", "", "/tmp1", "/tmp", "", "", true, false},
			{"/tmp:/tmp2:ro", "", "/tmp2", "/tmp", "", "", false, false},
			{"/tmp:/tmp3:rw", "", "/tmp3", "/tmp", "", "", true, false},
			{"/tmp:/tmp4:foo", "", "", "", "", "", false, true},
			{"name:/named1", "", "/named1", "", "name", "local", true, false},
			{"name:/named2", "external", "/named2", "", "name", "external", true, false},
			{"name:/named3:ro", "local", "/named3", "", "name", "local", false, false},
			{"local/name:/tmp:rw", "", "/tmp", "", "local/name", "local", true, false},
			{"/tmp:tmp", "", "", "", "", "", true, true},
		}
	}

	for _, c := range cases {
		m, err := ParseMountSpec(c.bind, c.driver)
		if c.fail {
			if err == nil {
				t.Fatalf("Expected error, was nil, for spec %s\n", c.bind)
			}
			continue
		}

		if m == nil || err != nil {
			t.Fatalf("ParseMountSpec failed for spec %s driver %s error %v\n", c.bind, c.driver, err.Error())
			continue
		}

		if m.Destination != c.expDest {
			t.Fatalf("Expected destination %s, was %s, for spec %s\n", c.expDest, m.Destination, c.bind)
		}

		if m.Source != c.expSource {
			t.Fatalf("Expected source %s, was %s, for spec %s\n", c.expSource, m.Source, c.bind)
		}

		if m.Name != c.expName {
			t.Fatalf("Expected name %s, was %s for spec %s\n", c.expName, m.Name, c.bind)
		}

		if m.Driver != c.expDriver {
			t.Fatalf("Expected driver %s, was %s, for spec %s\n", c.expDriver, m.Driver, c.bind)
		}

		if m.RW != c.expRW {
			t.Fatalf("Expected RW %v, was %v for spec %s\n", c.expRW, m.RW, c.bind)
		}
	}
}
