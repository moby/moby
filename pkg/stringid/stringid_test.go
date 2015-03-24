package stringid

import "testing"

func TestGenerateRandomID(t *testing.T) {
	id := GenerateRandomID()

	if len(id) != 64 {
		t.Fatalf("Id returned is incorrect: %s", id)
	}
}

func TestShortenId(t *testing.T) {
	id := GenerateRandomID()
	truncID := TruncateID(id)
	if len(truncID) != 12 {
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
