// +build windows

package dockerfile

import "github.com/go-check/check"

func (s *DockerSuite) TestNormaliseWorkdir(c *check.C) {
	tests := []struct{ current, requested, expected, etext string }{
		{``, ``, ``, `cannot normalise nothing`},
		{``, `a`, `C:\a`, ``},
		{``, `c:\foo`, `C:\foo`, ``},
		{``, `\foo`, `C:\foo`, ``},
		{``, `/foo`, `C:\foo`, ``},
		{``, `C:/foo`, `C:\foo`, ``},
		{`C:\foo`, `bar`, `C:\foo\bar`, ``},
		{`C:\foo`, `/bar`, `C:\bar`, ``},
		{`C:\foo`, `\bar`, `C:\bar`, ``},
	}
	for _, i := range tests {
		r, e := normaliseWorkdir(i.current, i.requested)

		if i.etext != "" && e == nil {
			c.Fatalf("TestNormaliseWorkingDir Expected error %s", i.etext)
		}

		if i.etext != "" && e.Error() != i.etext {
			c.Fatalf("TestNormaliseWorkingDir Expected error %s, got %s", i.etext, e.Error())
		}

		if r != i.expected {
			c.Fatalf("TestNormaliseWorkingDir Expected %s for %s %s", i.expected, i.current, i.requested)
		}
	}
}
