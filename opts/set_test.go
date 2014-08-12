package opts

import (
	"flag"
	"testing"
)

func TestSet(t *testing.T) {
	var l flag.Value = make(Set)
	l.Set("ga")
	l.Set("bu")
	l.Set("meu")
	l.Set("ga")
	l.Set("ga")
	l.Set("ga")
	l.Set("ga")
	l.Set("zo")
	if out := l.String(); out != "bu,ga,meu,zo" {
		t.Fatalf("%#v", out)
	}
}
