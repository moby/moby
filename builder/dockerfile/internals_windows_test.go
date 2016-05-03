// +build windows

package dockerfile

import "testing"

func TestNormaliseDest(t *testing.T) {
	tests := []struct{ current, requested, expected, etext string }{
		{``, `D:\`, ``, `Windows does not support TEST with a destinations not on the system drive (C:)`},
		{``, `e:/`, ``, `Windows does not support TEST with a destinations not on the system drive (C:)`},
		{`invalid`, `./c1`, ``, `Current WorkingDir invalid is not platform consistent`},
		{`C:`, ``, ``, `Current WorkingDir C: is not platform consistent`},
		{`C`, ``, ``, `Current WorkingDir C is not platform consistent`},
		{`D:\`, `.`, ``, "Windows does not support TEST with relative paths when WORKDIR is not the system drive"},
		{``, `D`, `D`, ``},
		{``, `./a1`, `.\a1`, ``},
		{``, `.\b1`, `.\b1`, ``},
		{``, `/`, `\`, ``},
		{``, `\`, `\`, ``},
		{``, `c:/`, `\`, ``},
		{``, `c:\`, `\`, ``},
		{``, `.`, `.`, ``},
		{`C:\wdd`, `./a1`, `\wdd\a1`, ``},
		{`C:\wde`, `.\b1`, `\wde\b1`, ``},
		{`C:\wdf`, `/`, `\`, ``},
		{`C:\wdg`, `\`, `\`, ``},
		{`C:\wdh`, `c:/`, `\`, ``},
		{`C:\wdi`, `c:\`, `\`, ``},
		{`C:\wdj`, `.`, `\wdj`, ``},
		{`C:\wdk`, `foo/bar`, `\wdk\foo\bar`, ``},
		{`C:\wdl`, `foo\bar`, `\wdl\foo\bar`, ``},
		{`C:\wdm`, `foo/bar/`, `\wdm\foo\bar\`, ``},
		{`C:\wdn`, `foo\bar/`, `\wdn\foo\bar\`, ``},
	}
	for _, i := range tests {
		got, err := normaliseDest("TEST", i.current, i.requested)
		if err != nil && i.etext == "" {
			t.Fatalf("TestNormaliseDest Got unexpected error %q for %s %s. ", err.Error(), i.current, i.requested)
		}
		if i.etext != "" && ((err == nil) || (err != nil && err.Error() != i.etext)) {
			if err == nil {
				t.Fatalf("TestNormaliseDest Expected an error for %s %s but didn't get one", i.current, i.requested)
			} else {
				t.Fatalf("TestNormaliseDest Wrong error text for %s %s - %s", i.current, i.requested, err.Error())
			}
		}
		if i.etext == "" && got != i.expected {
			t.Fatalf("TestNormaliseDest Expected %q for %q and %q. Got %q", i.expected, i.current, i.requested, got)
		}
	}
}
