package stringid // import "github.com/docker/docker/pkg/stringid"

import (
	"strings"
	"testing"
)

func TestGenerateRandomID(t *testing.T) {
	id := GenerateRandomID()

	if len(id) != fullLen {
		t.Fatalf("Id returned is incorrect: %s", id)
	}
}

func TestShortenId(t *testing.T) {
	id := "90435eec5c4e124e741ef731e118be2fc799a68aba0466ec17717f24ce2ae6a2"
	truncID := TruncateID(id)
	if truncID != "90435eec5c4e" {
		t.Fatalf("Id returned is incorrect: truncate on %s returned %s", id, truncID)
	}
}

func TestShortenSha256Id(t *testing.T) {
	id := "sha256:4e38e38c8ce0b8d9041a9c4fefe786631d1416225e13b0bfe8cfa2321aec4bba"
	truncID := TruncateID(id)
	if truncID != "4e38e38c8ce0" {
		t.Fatalf("Id returned is incorrect: truncate on %s returned %s", id, truncID)
	}
}

func TestShortenIdEmpty(t *testing.T) {
	id := ""
	truncID := TruncateID(id)
	if len(truncID) > len(id) {
		t.Fatalf("Id returned is incorrect: truncate on %s returned %s", id, truncID)
	}
}

func TestShortenIdInvalid(t *testing.T) {
	id := "1234"
	truncID := TruncateID(id)
	if len(truncID) != len(id) {
		t.Fatalf("Id returned is incorrect: truncate on %s returned %s", id, truncID)
	}
}

func TestIsShortIDNonHex(t *testing.T) {
	id := "some non-hex value"
	if IsShortID(id) {
		t.Fatalf("%s is not a short ID", id)
	}
}

func TestIsShortIDNotCorrectSize(t *testing.T) {
	id := strings.Repeat("a", shortLen+1)
	if IsShortID(id) {
		t.Fatalf("%s is not a short ID", id)
	}
	id = strings.Repeat("a", shortLen-1)
	if IsShortID(id) {
		t.Fatalf("%s is not a short ID", id)
	}
}

var testIDs = []string{
	"4e38e38c8ce0",
	strings.Repeat("a", shortLen+1),
	strings.Repeat("a", 16000),
	"90435eec5c4e124e741ef731e118be2fc799a68aba0466ec17717f24ce2ae6a2",
}

func BenchmarkIsShortID(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, id := range testIDs {
			_ = IsShortID(id)
		}
	}
}

func BenchmarkValidateID(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, id := range testIDs {
			_ = ValidateID(id)
		}
	}
}
