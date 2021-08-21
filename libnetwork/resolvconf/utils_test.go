package resolvconf

import (
	"strings"
	"testing"
)

func TestHashData(t *testing.T) {
	reader := strings.NewReader("hash-me")
	actual, err := hashData(reader)
	if err != nil {
		t.Fatal(err)
	}
	expected := "sha256:4d11186aed035cc624d553e10db358492c84a7cd6b9670d92123c144930450aa"
	if actual != expected {
		t.Fatalf("Expecting %s, got %s", expected, actual)
	}
}
