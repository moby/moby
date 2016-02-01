package loggerutils

import (
	"testing"

	"github.com/docker/docker/daemon/logger"
)

func TestParseDefaultIgnoreFlag(t *testing.T) {
	ctx := buildContext(map[string]string{})
	flag, e := ParseFailOnStartupErrorFlag(ctx)
	assertFlag(t, e, flag, true)
}

func TestParseIgnoreFlagWhenFalse(t *testing.T) {
	ctx := buildContext(map[string]string{"fail-on-startup-error": "false"})
	flag, e := ParseFailOnStartupErrorFlag(ctx)
	assertFlag(t, e, flag, false)
}

func TestParseIgnoreFlagWhenTrue(t *testing.T) {
	ctx := buildContext(map[string]string{"fail-on-startup-error": "true"})
	flag, e := ParseFailOnStartupErrorFlag(ctx)
	assertFlag(t, e, flag, true)
}

func TestParseIgnoreFlagWithError(t *testing.T) {
	ctx := buildContext(map[string]string{"fail-on-startup-error": "maybe :)"})
	flag, e := ParseFailOnStartupErrorFlag(ctx)
	if e == nil {
		t.Fatalf("Error should have happened")
	}
	assertFlag(t, nil, flag, true)
}

// Helpers

func buildConfig(cfg map[string]string) logger.Context {
	return logger.Context{
		Config: cfg,
	}
}

func assertFlag(t *testing.T, e error, flag bool, expected bool) {
	if e != nil {
		t.Fatalf("Error parsing ignore connect error flag: %q", e)
	}
	if flag != expected {
		t.Fatalf("Wrong flag: %t, should be %t", flag, expected)
	}
}
