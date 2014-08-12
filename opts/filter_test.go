package opts

import (
	"flag"
	"fmt"
	"testing"
)

func TestFilter(t *testing.T) {
	var f flag.Value = Filter(new(List), func(val string) (string, error) {
		if val == "gloubiboulga" {
			return "", fmt.Errorf("denied value")
		}
		return val + "-foo", nil
	})
	if err := f.Set("ga"); err != nil {
		t.Fatal(err)
	}
	if err := f.Set("bu"); err != nil {
		t.Fatal(err)
	}
	if err := f.Set("zo"); err != nil {
		t.Fatal(err)
	}
	if err := f.Set("gloubiboulga"); err == nil {
		t.Fatalf("%#v", f)
	}
	if err := f.Set("meu"); err != nil {
		t.Fatal(err)
	}
	if out := f.String(); out != "[ga-foo bu-foo zo-foo meu-foo]" {
		t.Fatalf("%#v", out)
	}
}
