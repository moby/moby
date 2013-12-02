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
	jp := JSONProgress{}
	if jp.String() != "" {
		t.Fatalf("Expected empty string, got '%s'", jp.String())
	}

	jp2 := JSONProgress{Current: 1}
	if jp2.String() != "     1 B" {
		t.Fatalf("Expected '     1 B', got '%s'", jp2.String())
	}

	jp3 := JSONProgress{Current: 50, Total: 100}
	if jp3.String() != "[=========================>                         ]     50 B/100 B" {
		t.Fatalf("Expected '[=========================>                         ]     50 B/100 B', got '%s'", jp3.String())
	}
}
