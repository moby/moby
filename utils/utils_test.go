package utils

import "testing"

func TestReplaceAndAppendEnvVars(t *testing.T) {
	var (
		d = []string{"HOME=/"}
		o = []string{"HOME=/root", "TERM=xterm"}
	)

	env := ReplaceOrAppendEnvValues(d, o)
	if len(env) != 2 {
		t.Fatalf("expected len of 2 got %d", len(env))
	}
	if env[0] != "HOME=/root" {
		t.Fatalf("expected HOME=/root got '%s'", env[0])
	}
	if env[1] != "TERM=xterm" {
		t.Fatalf("expected TERM=xterm got '%s'", env[1])
	}
}
