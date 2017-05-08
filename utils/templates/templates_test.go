package templates

import (
	"bytes"
	"testing"
)

func TestParseStringFunctions(t *testing.T) {
	tm, err := Parse(`{{join (split . ":") "/"}}`)
	if err != nil {
		t.Fatal(err)
	}

	var b bytes.Buffer
	if err := tm.Execute(&b, "text:with:colon"); err != nil {
		t.Fatal(err)
	}
	want := "text/with/colon"
	if b.String() != want {
		t.Fatalf("expected %s, got %s", want, b.String())
	}
}

func TestNewParse(t *testing.T) {
	tm, err := NewParse("foo", "this is a {{ . }}")
	if err != nil {
		t.Fatal(err)
	}

	var b bytes.Buffer
	if err := tm.Execute(&b, "string"); err != nil {
		t.Fatal(err)
	}
	want := "this is a string"
	if b.String() != want {
		t.Fatalf("expected %s, got %s", want, b.String())
	}
}
