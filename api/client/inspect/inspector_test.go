package inspect

import (
	"bytes"
	"strings"
	"testing"

	"github.com/docker/docker/utils/templates"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

type testElement struct {
	DNS string `json:"Dns"`
}

func (s *DockerSuite) TestTemplateInspectorDefault(c *check.C) {
	b := new(bytes.Buffer)
	tmpl, err := templates.Parse("{{.DNS}}")
	if err != nil {
		c.Fatal(err)
	}
	i := NewTemplateInspector(b, tmpl)
	if err := i.Inspect(testElement{"0.0.0.0"}, nil); err != nil {
		c.Fatal(err)
	}

	if err := i.Flush(); err != nil {
		c.Fatal(err)
	}
	if b.String() != "0.0.0.0\n" {
		c.Fatalf("Expected `0.0.0.0\\n`, got `%s`", b.String())
	}
}

func (s *DockerSuite) TestTemplateInspectorEmpty(c *check.C) {
	b := new(bytes.Buffer)
	tmpl, err := templates.Parse("{{.DNS}}")
	if err != nil {
		c.Fatal(err)
	}
	i := NewTemplateInspector(b, tmpl)

	if err := i.Flush(); err != nil {
		c.Fatal(err)
	}
	if b.String() != "\n" {
		c.Fatalf("Expected `\\n`, got `%s`", b.String())
	}
}

func (s *DockerSuite) TestTemplateInspectorTemplateError(c *check.C) {
	b := new(bytes.Buffer)
	tmpl, err := templates.Parse("{{.Foo}}")
	if err != nil {
		c.Fatal(err)
	}
	i := NewTemplateInspector(b, tmpl)

	err = i.Inspect(testElement{"0.0.0.0"}, nil)
	if err == nil {
		c.Fatal("Expected error got nil")
	}

	if !strings.HasPrefix(err.Error(), "Template parsing error") {
		c.Fatalf("Expected template error, got %v", err)
	}
}

func (s *DockerSuite) TestTemplateInspectorRawFallback(c *check.C) {
	b := new(bytes.Buffer)
	tmpl, err := templates.Parse("{{.Dns}}")
	if err != nil {
		c.Fatal(err)
	}
	i := NewTemplateInspector(b, tmpl)
	if err := i.Inspect(testElement{"0.0.0.0"}, []byte(`{"Dns": "0.0.0.0"}`)); err != nil {
		c.Fatal(err)
	}

	if err := i.Flush(); err != nil {
		c.Fatal(err)
	}
	if b.String() != "0.0.0.0\n" {
		c.Fatalf("Expected `0.0.0.0\\n`, got `%s`", b.String())
	}
}

func (s *DockerSuite) TestTemplateInspectorRawFallbackError(c *check.C) {
	b := new(bytes.Buffer)
	tmpl, err := templates.Parse("{{.Dns}}")
	if err != nil {
		c.Fatal(err)
	}
	i := NewTemplateInspector(b, tmpl)
	err = i.Inspect(testElement{"0.0.0.0"}, []byte(`{"Foo": "0.0.0.0"}`))
	if err == nil {
		c.Fatal("Expected error got nil")
	}

	if !strings.HasPrefix(err.Error(), "Template parsing error") {
		c.Fatalf("Expected template error, got %v", err)
	}
}

func (s *DockerSuite) TestTemplateInspectorMultiple(c *check.C) {
	b := new(bytes.Buffer)
	tmpl, err := templates.Parse("{{.DNS}}")
	if err != nil {
		c.Fatal(err)
	}
	i := NewTemplateInspector(b, tmpl)

	if err := i.Inspect(testElement{"0.0.0.0"}, nil); err != nil {
		c.Fatal(err)
	}
	if err := i.Inspect(testElement{"1.1.1.1"}, nil); err != nil {
		c.Fatal(err)
	}

	if err := i.Flush(); err != nil {
		c.Fatal(err)
	}
	if b.String() != "0.0.0.0\n1.1.1.1\n" {
		c.Fatalf("Expected `0.0.0.0\\n1.1.1.1\\n`, got `%s`", b.String())
	}
}

func (s *DockerSuite) TestIndentedInspectorDefault(c *check.C) {
	b := new(bytes.Buffer)
	i := NewIndentedInspector(b)
	if err := i.Inspect(testElement{"0.0.0.0"}, nil); err != nil {
		c.Fatal(err)
	}

	if err := i.Flush(); err != nil {
		c.Fatal(err)
	}

	expected := `[
    {
        "Dns": "0.0.0.0"
    }
]
`
	if b.String() != expected {
		c.Fatalf("Expected `%s`, got `%s`", expected, b.String())
	}
}

func (s *DockerSuite) TestIndentedInspectorMultiple(c *check.C) {
	b := new(bytes.Buffer)
	i := NewIndentedInspector(b)
	if err := i.Inspect(testElement{"0.0.0.0"}, nil); err != nil {
		c.Fatal(err)
	}

	if err := i.Inspect(testElement{"1.1.1.1"}, nil); err != nil {
		c.Fatal(err)
	}

	if err := i.Flush(); err != nil {
		c.Fatal(err)
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
		c.Fatalf("Expected `%s`, got `%s`", expected, b.String())
	}
}

func (s *DockerSuite) TestIndentedInspectorEmpty(c *check.C) {
	b := new(bytes.Buffer)
	i := NewIndentedInspector(b)

	if err := i.Flush(); err != nil {
		c.Fatal(err)
	}

	expected := "[]\n"
	if b.String() != expected {
		c.Fatalf("Expected `%s`, got `%s`", expected, b.String())
	}
}

func (s *DockerSuite) TestIndentedInspectorRawElements(c *check.C) {
	b := new(bytes.Buffer)
	i := NewIndentedInspector(b)
	if err := i.Inspect(testElement{"0.0.0.0"}, []byte(`{"Dns": "0.0.0.0", "Node": "0"}`)); err != nil {
		c.Fatal(err)
	}

	if err := i.Inspect(testElement{"1.1.1.1"}, []byte(`{"Dns": "1.1.1.1", "Node": "1"}`)); err != nil {
		c.Fatal(err)
	}

	if err := i.Flush(); err != nil {
		c.Fatal(err)
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
		c.Fatalf("Expected `%s`, got `%s`", expected, b.String())
	}
}
