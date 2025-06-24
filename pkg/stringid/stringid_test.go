package stringid

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
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

func TestUUIDv7(t *testing.T) {
	idv7, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	cleanID := strings.ReplaceAll(idv7.String(), "-", "")
	fullID := cleanID + padding
	shortID := cleanID[:shortLen]
	t.Log("UUIDv7:       ", idv7.String(), "(", len(idv7.String()), " chars )")
	t.Log("UUIDv7 clean: ", cleanID, "(", len(cleanID), "chars )")
	t.Log("FULL ID:      ", fullID, "(", len(fullID), "chars )")
	t.Log("SHORT ID:     ", shortID, "(", len(shortID), "chars )")
	t.Log("URN:          ", idv7.URN())
	t.Log("VERSION:      ", idv7.Version())
	t.Log("VARIANT:      ", idv7.Variant())
	t.Log("TIME:         ", idv7.Time())
	t.Log("RFC3339:      ", time.Unix(idv7.Time().UnixTime()).Format(time.RFC3339Nano))
	parsed, _ := toUUID(cleanID)
	t.Log("Parsed:       ", parsed.String())
	t.Log("Parsed short: ", uuid.Must(toUUID(shortID)))
	t.Log("LEGACY:       ", GenerateRandomID())
}
