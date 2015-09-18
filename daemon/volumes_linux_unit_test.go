// +build experimental

package daemon

import "testing"

func TestParseBindMount(t *testing.T) {
	cases := []struct {
		bind      string
		driver    string
		expDest   string
		expSource string
		expName   string
		expDriver string
		expRW     bool
		fail      bool
	}{
		{"/tmp:/tmp", "", "/tmp", "/tmp", "", "", true, false},
		{"/tmp:/tmp:ro", "", "/tmp", "/tmp", "", "", false, false},
		{"/tmp:/tmp:rw", "", "/tmp", "/tmp", "", "", true, false},
		{"/tmp:/tmp:foo", "", "/tmp", "/tmp", "", "", false, true},
		{"name:/tmp", "", "/tmp", "", "name", "local", true, false},
		{"name:/tmp", "external", "/tmp", "", "name", "external", true, false},
		{"name:/tmp:ro", "local", "/tmp", "", "name", "local", false, false},
		{"local/name:/tmp:rw", "", "/tmp", "", "local/name", "local", true, false},
		{"/tmp:tmp", "", "", "", "", "", true, true},
	}

	for _, c := range cases {
		m, err := parseBindMount(c.bind, c.driver)
		if c.fail {
			if err == nil {
				t.Fatalf("Expected error, was nil, for spec %s\n", c.bind)
			}
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
