package ulimit

import "testing"

func TestParseInvalidLimitType(t *testing.T) {
	if _, err := Parse("notarealtype=1024:1024"); err == nil {
		t.Fatalf("expected error on invalid ulimit type")
	}
}

func TestParseBadFormat(t *testing.T) {
	if _, err := Parse("nofile:1024:1024"); err == nil {
		t.Fatal("expected error on bad syntax")
	}

	if _, err := Parse("nofile"); err == nil {
		t.Fatal("expected error on bad syntax")
	}

	if _, err := Parse("nofile="); err == nil {
		t.Fatal("expected error on bad syntax")
	}
	if _, err := Parse("nofile=:"); err == nil {
		t.Fatal("expected error on bad syntax")
	}
	if _, err := Parse("nofile=:1024"); err == nil {
		t.Fatal("expected error on bad syntax")
	}
}

func TestParseHardLessThanSoft(t *testing.T) {
	if _, err := Parse("nofile:1024:1"); err == nil {
		t.Fatal("expected error on hard limit less than soft limit")
	}
}

func TestParseInvalidValueType(t *testing.T) {
	if _, err := Parse("nofile:asdf"); err == nil {
		t.Fatal("expected error on bad value type")
	}
}
