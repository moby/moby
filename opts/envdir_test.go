package opts

import (
	"testing"
)

func TestParseEnvDir(t *testing.T) {
	fixtures := []struct {
		dir     string
		results []string
		failure bool
	}{
		{"fixtures/envdir/simple", []string{"BAR=baz", "FOO=bar"}, false},
		{"fixtures/envdir/firstline", []string{"FOO=bar"}, false},
		{"fixtures/envdir/junk", []string{"FOO=bar"}, false},
		{"fixtures/envdir/trim", []string{"FOO=bar"}, false},
		{"fixtures/envdir/null", []string{"FOO=ba\nr"}, false},
		{"fixtures/envdir/zerobyte", []string{"FOO="}, false},
		{"fixtures/envdir/empty", []string{}, false},
		{"fixtures/envdir/whitespace", []string{}, true},
	}

	for _, f := range fixtures {
		vars, err := ParseEnvDir(f.dir)
		if f.failure {
			if err == nil {
				t.Fatalf("Expected a failure, but succeeded instead: %v", f.dir)
			}
		} else {
			if err != nil {
				t.Fatalf("Unexepcted error on %v: %v", f.dir, err)
			}
		}
		if err != nil {
			if !f.failure {
				t.Fatal(err)
			}
		}
		if ok := assertSliceOfStrings(vars, f.results); !ok {
			t.Fatalf("Expected %v, found %v\n", f.results, vars)
		}
	}
}

func assertSliceOfStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for _, item := range a {
		if !stringInSlice(item, b) {
			return false
		}
	}

	return true
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
