package utils

import (
	"testing"
)

func TestError(t *testing.T) {
	je := JSONError{404, "Not found"}
	if je.Error() != "Not found" {
		t.Fatalf("Expected 'Not found' got '%s'", je.Error())
	}
}

func TestProgress(t *testing.T) {
	jp := JSONProgress{0, 0, 0}
	if jp.String() != "" {
		t.Fatalf("Expected empty string, got '%s'", jp.String())
	}

	jp2 := JSONProgress{1, 0, 0}
	if jp2.String() != "     1 B/?" {
		t.Fatalf("Expected '     1/?', got '%s'", jp2.String())
	}
}
