package inspect

import (
	"bytes"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/templates"
)

type testElement struct {
	DNS string `json:"Dns"`
}

func TestTemplateInspectorDefault(t *testing.T) {
	b := new(bytes.Buffer)
	tmpl, err := templates.Parse("{{.DNS}}")
	if err != nil {
		t.Fatal(err)
	}
	i := NewTemplateInspector(b, tmpl)
	if err := i.Inspect(testElement{"0.0.0.0"}, nil); err != nil {
		t.Fatal(err)
	}

	if err := i.Flush(); err != nil {
		t.Fatal(err)
	}
	if b.String() != "0.0.0.0\n" {
		t.Fatalf("Expected `0.0.0.0\\n`, got `%s`", b.String())
	}
}

func TestTemplateInspectorEmpty(t *testing.T) {
	b := new(bytes.Buffer)
	tmpl, err := templates.Parse("{{.DNS}}")
	if err != nil {
		t.Fatal(err)
	}
	i := NewTemplateInspector(b, tmpl)

	if err := i.Flush(); err != nil {
		t.Fatal(err)
	}
	if b.String() != "\n" {
		t.Fatalf("Expected `\\n`, got `%s`", b.String())
	}
}

func TestTemplateInspectorTemplateError(t *testing.T) {
	b := new(bytes.Buffer)
	tmpl, err := templates.Parse("{{.Foo}}")
	if err != nil {
		t.Fatal(err)
	}
	i := NewTemplateInspector(b, tmpl)

	err = i.Inspect(testElement{"0.0.0.0"}, nil)
	if err == nil {
		t.Fatal("Expected error got nil")
	}

	if !strings.HasPrefix(err.Error(), "Template parsing error") {
		t.Fatalf("Expected template error, got %v", err)
	}
}

func TestTemplateInspectorRawFallback(t *testing.T) {
	b := new(bytes.Buffer)
	tmpl, err := templates.Parse("{{.Dns}}")
	if err != nil {
		t.Fatal(err)
	}
	i := NewTemplateInspector(b, tmpl)
	if err := i.Inspect(testElement{"0.0.0.0"}, []byte(`{"Dns": "0.0.0.0"}`)); err != nil {
		t.Fatal(err)
	}

	if err := i.Flush(); err != nil {
		t.Fatal(err)
	}
	if b.String() != "0.0.0.0\n" {
		t.Fatalf("Expected `0.0.0.0\\n`, got `%s`", b.String())
	}
}

func TestTemplateInspectorRawFallbackError(t *testing.T) {
	b := new(bytes.Buffer)
	tmpl, err := templates.Parse("{{.Dns}}")
	if err != nil {
		t.Fatal(err)
	}
	i := NewTemplateInspector(b, tmpl)
	err = i.Inspect(testElement{"0.0.0.0"}, []byte(`{"Foo": "0.0.0.0"}`))
	if err == nil {
		t.Fatal("Expected error got nil")
	}

	if !strings.HasPrefix(err.Error(), "Template parsing error") {
		t.Fatalf("Expected template error, got %v", err)
	}
}

func TestTemplateInspectorMultiple(t *testing.T) {
	b := new(bytes.Buffer)
	tmpl, err := templates.Parse("{{.DNS}}")
	if err != nil {
		t.Fatal(err)
	}
	i := NewTemplateInspector(b, tmpl)

	if err := i.Inspect(testElement{"0.0.0.0"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := i.Inspect(testElement{"1.1.1.1"}, nil); err != nil {
		t.Fatal(err)
	}

	if err := i.Flush(); err != nil {
		t.Fatal(err)
	}
	if b.String() != "0.0.0.0\n1.1.1.1\n" {
		t.Fatalf("Expected `0.0.0.0\\n1.1.1.1\\n`, got `%s`", b.String())
	}
}

func TestIndentedInspectorDefault(t *testing.T) {
	b := new(bytes.Buffer)
	i := NewIndentedInspector(b)
	if err := i.Inspect(testElement{"0.0.0.0"}, nil); err != nil {
		t.Fatal(err)
	}

	if err := i.Flush(); err != nil {
		t.Fatal(err)
	}

	expected := `[
    {
        "Dns": "0.0.0.0"
    }
]
`
	if b.String() != expected {
		t.Fatalf("Expected `%s`, got `%s`", expected, b.String())
	}
}

func TestIndentedInspectorMultiple(t *testing.T) {
	b := new(bytes.Buffer)
	i := NewIndentedInspector(b)
	if err := i.Inspect(testElement{"0.0.0.0"}, nil); err != nil {
		t.Fatal(err)
	}

	if err := i.Inspect(testElement{"1.1.1.1"}, nil); err != nil {
		t.Fatal(err)
	}

	if err := i.Flush(); err != nil {
		t.Fatal(err)
	}

	expected := `[
    {
        "Dns": "0.0.0.0"
    },
    {
        "Dns": "1.1.1.1"
    }
]
`
	if b.String() != expected {
		t.Fatalf("Expected `%s`, got `%s`", expected, b.String())
	}
}

func TestIndentedInspectorEmpty(t *testing.T) {
	b := new(bytes.Buffer)
	i := NewIndentedInspector(b)

	if err := i.Flush(); err != nil {
		t.Fatal(err)
	}

	expected := "[]\n"
	if b.String() != expected {
		t.Fatalf("Expected `%s`, got `%s`", expected, b.String())
	}
}

func TestIndentedInspectorRawElements(t *testing.T) {
	b := new(bytes.Buffer)
	i := NewIndentedInspector(b)
	if err := i.Inspect(testElement{"0.0.0.0"}, []byte(`{"Dns": "0.0.0.0", "Node": "0"}`)); err != nil {
		t.Fatal(err)
	}

	if err := i.Inspect(testElement{"1.1.1.1"}, []byte(`{"Dns": "1.1.1.1", "Node": "1"}`)); err != nil {
		t.Fatal(err)
	}

	if err := i.Flush(); err != nil {
		t.Fatal(err)
	}

	expected := `[
    {
        "Dns": "0.0.0.0",
        "Node": "0"
    },
    {
        "Dns": "1.1.1.1",
        "Node": "1"
    }
]
`
	if b.String() != expected {
		t.Fatalf("Expected `%s`, got `%s`", expected, b.String())
	}
}
