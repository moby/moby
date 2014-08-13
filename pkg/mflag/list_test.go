package mflag

import (
	"testing"
)

func TestList(t *testing.T) {
	var l Value = new(List)
	l.Set("ga")
	l.Set("bu")
	l.Set("zo")
	l.Set("meu")
	if out := l.String(); out != "[ga bu zo meu]" {
		t.Fatalf("%#v", out)
	}
}
