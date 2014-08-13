package mflag

import (
	"testing"
)

func TestStringSet(t *testing.T) {
	var l Value = make(StringSet)
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
