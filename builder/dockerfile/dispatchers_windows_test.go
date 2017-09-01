// +build windows

package dockerfile

import "testing"

func TestNormalizeWorkdir(t *testing.T) {
	tests := []struct{ platform, current, requested, expected, etext string }{
		{"windows", ``, ``, ``, `cannot normalize nothing`},
		{"windows", ``, `C:`, ``, `C:. is not a directory. If you are specifying a drive letter, please add a trailing '\'`},
		{"windows", ``, `C:.`, ``, `C:. is not a directory. If you are specifying a drive letter, please add a trailing '\'`},
		{"windows", `c:`, `\a`, ``, `c:. is not a directory. If you are specifying a drive letter, please add a trailing '\'`},
		{"windows", `c:.`, `\a`, ``, `c:. is not a directory. If you are specifying a drive letter, please add a trailing '\'`},
		{"windows", ``, `a`, `C:\a`, ``},
		{"windows", ``, `c:\foo`, `C:\foo`, ``},
		{"windows", ``, `c:\\foo`, `C:\foo`, ``},
		{"windows", ``, `\foo`, `C:\foo`, ``},
		{"windows", ``, `\\foo`, `C:\foo`, ``},
		{"windows", ``, `/foo`, `C:\foo`, ``},
		{"windows", ``, `C:/foo`, `C:\foo`, ``},
		{"windows", `C:\foo`, `bar`, `C:\foo\bar`, ``},
		{"windows", `C:\foo`, `/bar`, `C:\bar`, ``},
		{"windows", `C:\foo`, `\bar`, `C:\bar`, ``},
		{"linux", ``, ``, ``, `cannot normalize nothing`},
		{"linux", ``, `foo`, `/foo`, ``},
		{"linux", ``, `/foo`, `/foo`, ``},
		{"linux", `/foo`, `bar`, `/foo/bar`, ``},
		{"linux", `/foo`, `/bar`, `/bar`, ``},
		{"linux", `\a`, `b\c`, `/a/b/c`, ``},
	}
	for _, i := range tests {
		r, e := normalizeWorkdir(i.platform, i.current, i.requested)

		if i.etext != "" && e == nil {
			t.Fatalf("TestNormalizeWorkingDir Expected error %s for '%s' '%s', got no error", i.etext, i.current, i.requested)
		}

		if i.etext != "" && e.Error() != i.etext {
			t.Fatalf("TestNormalizeWorkingDir Expected error %s for '%s' '%s', got %s", i.etext, i.current, i.requested, e.Error())
		}

		if r != i.expected {
			t.Fatalf("TestNormalizeWorkingDir Expected '%s' for '%s' '%s', got '%s'", i.expected, i.current, i.requested, r)
		}
	}
}
