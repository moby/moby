package opts

import (
	"flag"
	"testing"
)

func TestList(t *testing.T) {
	var l flag.Value = new(List)
	l.Set("ga")
	l.Set("bu")
	l.Set("zo")
	l.Set("meu")
	if out := l.String(); out != "[ga bu zo meu]" {
		t.Fatalf("%#v", out)
	}
}
