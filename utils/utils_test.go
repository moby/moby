package utils

import "github.com/go-check/check"

func (s *DockerSuite) TestReplaceAndAppendEnvVars(c *check.C) {
	var (
		d = []string{"HOME=/"}
		o = []string{"HOME=/root", "TERM=xterm"}
	)

	env := ReplaceOrAppendEnvValues(d, o)
	if len(env) != 2 {
		c.Fatalf("expected len of 2 got %d", len(env))
	}
	if env[0] != "HOME=/root" {
		c.Fatalf("expected HOME=/root got '%s'", env[0])
	}
	if env[1] != "TERM=xterm" {
		c.Fatalf("expected TERM=xterm got '%s'", env[1])
	}
}
