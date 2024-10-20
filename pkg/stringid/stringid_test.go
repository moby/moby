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

func TestTruncateID(t *testing.T) {
	tests := []struct {
		doc, id, expected string
	}{
		{
			doc:      "empty ID",
			id:       "",
			expected: "",
		},
		{
			// IDs are expected to be 12 (short) or 64 characters, and not be numeric only,
			// but TruncateID should handle these gracefully.
			doc:      "invalid ID",
			id:       "1234",
			expected: "1234",
		},
		{
			doc:      "full ID",
			id:       "90435eec5c4e124e741ef731e118be2fc799a68aba0466ec17717f24ce2ae6a2",
			expected: "90435eec5c4e",
		},
		{
			doc:      "digest",
			id:       "sha256:90435eec5c4e124e741ef731e118be2fc799a68aba0466ec17717f24ce2ae6a2",
			expected: "90435eec5c4e",
		},
		{
			doc:      "very long ID",
			id:       "90435eec5c4e124e741ef731e118be2fc799a68aba0466ec17717f24ce2ae6a290435eec5c4e124e741ef731e118be2fc799a68aba0466ec17717f24ce2ae6a2",
			expected: "90435eec5c4e",
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			actual := TruncateID(tc.id)
			if actual != tc.expected {
				t.Errorf("expected: %q, got: %q", tc.expected, actual)
			}
		})
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
