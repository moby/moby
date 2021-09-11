package namesgenerator_test // import "github.com/docker/docker/pkg/namesgenerator"

import (
	"strings"
	"testing"

	"github.com/docker/docker/pkg/namesgenerator"
)

func TestNameFormat(t *testing.T) {
	name := namesgenerator.GetRandomName(0)
	if !strings.Contains(name, "_") {
		t.Fatalf("Generated name does not contain an underscore")
	}
	if strings.ContainsAny(name, "0123456789") {
		t.Fatalf("Generated name contains numbers!")
	}
}

func TestNameRetries(t *testing.T) {
	name := namesgenerator.GetRandomName(1)
	if !strings.Contains(name, "_") {
		t.Fatalf("Generated name does not contain an underscore")
	}
	if !strings.ContainsAny(name, "0123456789") {
		t.Fatalf("Generated name doesn't contain a number")
	}

}
