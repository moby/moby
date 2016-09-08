package inspect

import (
	"bytes"
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
	c.Assert(err, check.IsNil)

	i := NewTemplateInspector(b, tmpl)
	c.Assert(i.Inspect(testElement{"0.0.0.0"}, nil), check.IsNil)

	c.Assert(i.Flush(), check.IsNil)
	c.Assert(b.String(), check.Equals, "0.0.0.0\n")
}

func (s *DockerSuite) TestTemplateInspectorEmpty(c *check.C) {
	b := new(bytes.Buffer)
	tmpl, err := templates.Parse("{{.DNS}}")
	c.Assert(err, check.IsNil)

	i := NewTemplateInspector(b, tmpl)
	c.Assert(i.Flush(), check.IsNil)
	c.Assert(b.String(), check.Equals, "\n")
}

func (s *DockerSuite) TestTemplateInspectorTemplateError(c *check.C) {
	b := new(bytes.Buffer)
	tmpl, err := templates.Parse("{{.Foo}}")
	c.Assert(err, check.IsNil)

	i := NewTemplateInspector(b, tmpl)
	err = i.Inspect(testElement{"0.0.0.0"}, nil)
	c.Assert(err, check.NotNil)

	c.Assert(err, check.ErrorMatches, ".*Template parsing error.*")
}

func (s *DockerSuite) TestTemplateInspectorRawFallback(c *check.C) {
	b := new(bytes.Buffer)
	tmpl, err := templates.Parse("{{.Dns}}")
	c.Assert(err, check.IsNil)

	i := NewTemplateInspector(b, tmpl)
	c.Assert(i.Inspect(testElement{"0.0.0.0"}, []byte(`{"Dns": "0.0.0.0"}`)), check.IsNil)

	c.Assert(i.Flush(), check.IsNil)
	c.Assert(b.String(), check.Equals, "0.0.0.0\n")
}

func (s *DockerSuite) TestTemplateInspectorRawFallbackError(c *check.C) {
	b := new(bytes.Buffer)
	tmpl, err := templates.Parse("{{.Dns}}")
	c.Assert(err, check.IsNil)

	i := NewTemplateInspector(b, tmpl)
	err = i.Inspect(testElement{"0.0.0.0"}, []byte(`{"Foo": "0.0.0.0"}`))
	c.Assert(err, check.NotNil)
	c.Assert(err, check.ErrorMatches, ".*Template parsing error.*")
}

func (s *DockerSuite) TestTemplateInspectorMultiple(c *check.C) {
	b := new(bytes.Buffer)
	tmpl, err := templates.Parse("{{.DNS}}")
	c.Assert(err, check.IsNil)

	i := NewTemplateInspector(b, tmpl)
	c.Assert(i.Inspect(testElement{"0.0.0.0"}, nil), check.IsNil)
	c.Assert(i.Inspect(testElement{"1.1.1.1"}, nil), check.IsNil)

	c.Assert(i.Flush(), check.IsNil)
	c.Assert(b.String(), check.Equals, "0.0.0.0\n1.1.1.1\n")
}

func (s *DockerSuite) TestIndentedInspectorDefault(c *check.C) {
	b := new(bytes.Buffer)
	i := NewIndentedInspector(b)
	c.Assert(i.Inspect(testElement{"0.0.0.0"}, nil), check.IsNil)

	c.Assert(i.Flush(), check.IsNil)
	expected := `[
    {
        "Dns": "0.0.0.0"
    }
]
`
	c.Assert(expected, check.Equals, b.String())
}

func (s *DockerSuite) TestIndentedInspectorMultiple(c *check.C) {
	b := new(bytes.Buffer)
	i := NewIndentedInspector(b)
	c.Assert(i.Inspect(testElement{"0.0.0.0"}, nil), check.IsNil)
	c.Assert(i.Inspect(testElement{"1.1.1.1"}, nil), check.IsNil)

	c.Assert(i.Flush(), check.IsNil)
	expected := `[
    {
        "Dns": "0.0.0.0"
    },
    {
        "Dns": "1.1.1.1"
    }
]
`
	c.Assert(expected, check.Equals, b.String())
}

func (s *DockerSuite) TestIndentedInspectorEmpty(c *check.C) {
	b := new(bytes.Buffer)
	i := NewIndentedInspector(b)

	c.Assert(i.Flush(), check.IsNil)
	expected := "[]\n"
	c.Assert(expected, check.Equals, b.String())
}

func (s *DockerSuite) TestIndentedInspectorRawElements(c *check.C) {
	b := new(bytes.Buffer)
	i := NewIndentedInspector(b)
	c.Assert(i.Inspect(testElement{"0.0.0.0"}, []byte(`{"Dns": "0.0.0.0", "Node": "0"}`)), check.IsNil)
	c.Assert(i.Inspect(testElement{"1.1.1.1"}, []byte(`{"Dns": "1.1.1.1", "Node": "1"}`)), check.IsNil)

	c.Assert(i.Flush(), check.IsNil)
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
	c.Assert(expected, check.Equals, b.String())
}
