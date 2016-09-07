package templates

import (
	"bytes"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestParseStringFunctions(c *check.C) {
	tm, err := Parse(`{{join (split . ":") "/"}}`)
	if err != nil {
		c.Fatal(err)
	}

	var b bytes.Buffer
	if err := tm.Execute(&b, "text:with:colon"); err != nil {
		c.Fatal(err)
	}
	want := "text/with/colon"
	if b.String() != want {
		c.Fatalf("expected %s, got %s", want, b.String())
	}
}

func (s *DockerSuite) TestNewParse(c *check.C) {
	tm, err := NewParse("foo", "this is a {{ . }}")
	if err != nil {
		c.Fatal(err)
	}

	var b bytes.Buffer
	if err := tm.Execute(&b, "string"); err != nil {
		c.Fatal(err)
	}
	want := "this is a string"
	if b.String() != want {
		c.Fatalf("expected %s, got %s", want, b.String())
	}
}
