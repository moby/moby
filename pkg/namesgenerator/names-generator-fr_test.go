package namesgenerator

import (
	"strings"
	"testing"
)

// Make sure the generated french names are awesome
func TestGenerateAwesomeFrenchNames(t *testing.T) {
	name := GetRandomFrenchName(0)
	if !isAwesome(name) {
		t.Fatalf("Generated name '%s' is not awesome.", name)
	}
}

func TestFrenchNameFormat(t *testing.T) {
	name := GetRandomFrenchName(0)
	if !strings.Contains(name, "_") {
		t.Fatalf("Generated name does not contain an underscore")
	}
	if strings.ContainsAny(name, "0123456789") {
		t.Fatalf("Generated name contains numbers!")
	}
}

func TestFrenchNameRetries(t *testing.T) {
	name := GetRandomFrenchName(1)
	if !strings.Contains(name, "_") {
		t.Fatalf("Generated name does not contain an underscore")
	}
	if !strings.ContainsAny(name, "0123456789") {
		t.Fatalf("Generated name doesn't contain a number")
	}
}
