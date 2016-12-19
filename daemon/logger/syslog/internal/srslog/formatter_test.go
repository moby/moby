package srslog

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestDefaultFormatter(t *testing.T) {
	out := DefaultFormatter(LOG_ERR, "hostname", "tag", "content")
	expected := fmt.Sprintf("<%d> %s %s %s[%d]: %s",
		LOG_ERR, time.Now().Format(time.RFC3339), "hostname", "tag", os.Getpid(), "content")
	if out != expected {
		t.Errorf("expected %v got %v", expected, out)
	}
}

func TestUnixFormatter(t *testing.T) {
	out := UnixFormatter(LOG_ERR, "hostname", "tag", "content")
	expected := fmt.Sprintf("<%d>%s %s[%d]: %s",
		LOG_ERR, time.Now().Format(time.Stamp), "tag", os.Getpid(), "content")
	if out != expected {
		t.Errorf("expected %v got %v", expected, out)
	}
}

func TestRFC3164Formatter(t *testing.T) {
	out := RFC3164Formatter(LOG_ERR, "hostname", "tag", "content")
	expected := fmt.Sprintf("<%d>%s %s %s[%d]: %s",
		LOG_ERR, time.Now().Format(time.Stamp), "hostname", "tag", os.Getpid(), "content")
	if out != expected {
		t.Errorf("expected %v got %v", expected, out)
	}
}

func TestRFC5424Formatter(t *testing.T) {
	out := RFC5424Formatter(LOG_ERR, "hostname", "tag", "content")
	expected := fmt.Sprintf("<%d>%d %s %s %s %d %s - %s",
		LOG_ERR, 1, time.Now().Format(time.RFC3339), "hostname", os.Args[0], os.Getpid(), "tag", "content")
	if out != expected {
		t.Errorf("expected %v got %v", expected, out)
	}
}
