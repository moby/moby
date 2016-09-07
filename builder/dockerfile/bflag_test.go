package dockerfile

import (
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestBuilderFlags(c *check.C) {
	var expected string
	var err error

	// ---

	bf := NewBFlags()
	bf.Args = []string{}
	if err := bf.Parse(); err != nil {
		c.Fatalf("Test1 of %q was supposed to work: %s", bf.Args, err)
	}

	// ---

	bf = NewBFlags()
	bf.Args = []string{"--"}
	if err := bf.Parse(); err != nil {
		c.Fatalf("Test2 of %q was supposed to work: %s", bf.Args, err)
	}

	// ---

	bf = NewBFlags()
	flStr1 := bf.AddString("str1", "")
	flBool1 := bf.AddBool("bool1", false)
	bf.Args = []string{}
	if err = bf.Parse(); err != nil {
		c.Fatalf("Test3 of %q was supposed to work: %s", bf.Args, err)
	}

	if flStr1.IsUsed() == true {
		c.Fatalf("Test3 - str1 was not used!")
	}
	if flBool1.IsUsed() == true {
		c.Fatalf("Test3 - bool1 was not used!")
	}

	// ---

	bf = NewBFlags()
	flStr1 = bf.AddString("str1", "HI")
	flBool1 = bf.AddBool("bool1", false)
	bf.Args = []string{}

	if err = bf.Parse(); err != nil {
		c.Fatalf("Test4 of %q was supposed to work: %s", bf.Args, err)
	}

	if flStr1.Value != "HI" {
		c.Fatalf("Str1 was supposed to default to: HI")
	}
	if flBool1.IsTrue() {
		c.Fatalf("Bool1 was supposed to default to: false")
	}
	if flStr1.IsUsed() == true {
		c.Fatalf("Str1 was not used!")
	}
	if flBool1.IsUsed() == true {
		c.Fatalf("Bool1 was not used!")
	}

	// ---

	bf = NewBFlags()
	flStr1 = bf.AddString("str1", "HI")
	bf.Args = []string{"--str1"}

	if err = bf.Parse(); err == nil {
		c.Fatalf("Test %q was supposed to fail", bf.Args)
	}

	// ---

	bf = NewBFlags()
	flStr1 = bf.AddString("str1", "HI")
	bf.Args = []string{"--str1="}

	if err = bf.Parse(); err != nil {
		c.Fatalf("Test %q was supposed to work: %s", bf.Args, err)
	}

	expected = ""
	if flStr1.Value != expected {
		c.Fatalf("Str1 (%q) should be: %q", flStr1.Value, expected)
	}

	// ---

	bf = NewBFlags()
	flStr1 = bf.AddString("str1", "HI")
	bf.Args = []string{"--str1=BYE"}

	if err = bf.Parse(); err != nil {
		c.Fatalf("Test %q was supposed to work: %s", bf.Args, err)
	}

	expected = "BYE"
	if flStr1.Value != expected {
		c.Fatalf("Str1 (%q) should be: %q", flStr1.Value, expected)
	}

	// ---

	bf = NewBFlags()
	flBool1 = bf.AddBool("bool1", false)
	bf.Args = []string{"--bool1"}

	if err = bf.Parse(); err != nil {
		c.Fatalf("Test %q was supposed to work: %s", bf.Args, err)
	}

	if !flBool1.IsTrue() {
		c.Fatalf("Test-b1 Bool1 was supposed to be true")
	}

	// ---

	bf = NewBFlags()
	flBool1 = bf.AddBool("bool1", false)
	bf.Args = []string{"--bool1=true"}

	if err = bf.Parse(); err != nil {
		c.Fatalf("Test %q was supposed to work: %s", bf.Args, err)
	}

	if !flBool1.IsTrue() {
		c.Fatalf("Test-b2 Bool1 was supposed to be true")
	}

	// ---

	bf = NewBFlags()
	flBool1 = bf.AddBool("bool1", false)
	bf.Args = []string{"--bool1=false"}

	if err = bf.Parse(); err != nil {
		c.Fatalf("Test %q was supposed to work: %s", bf.Args, err)
	}

	if flBool1.IsTrue() {
		c.Fatalf("Test-b3 Bool1 was supposed to be false")
	}

	// ---

	bf = NewBFlags()
	flBool1 = bf.AddBool("bool1", false)
	bf.Args = []string{"--bool1=false1"}

	if err = bf.Parse(); err == nil {
		c.Fatalf("Test %q was supposed to fail", bf.Args)
	}

	// ---

	bf = NewBFlags()
	flBool1 = bf.AddBool("bool1", false)
	bf.Args = []string{"--bool2"}

	if err = bf.Parse(); err == nil {
		c.Fatalf("Test %q was supposed to fail", bf.Args)
	}

	// ---

	bf = NewBFlags()
	flStr1 = bf.AddString("str1", "HI")
	flBool1 = bf.AddBool("bool1", false)
	bf.Args = []string{"--bool1", "--str1=BYE"}

	if err = bf.Parse(); err != nil {
		c.Fatalf("Test %q was supposed to work: %s", bf.Args, err)
	}

	if flStr1.Value != "BYE" {
		c.Fatalf("Teset %s, str1 should be BYE", bf.Args)
	}
	if !flBool1.IsTrue() {
		c.Fatalf("Teset %s, bool1 should be true", bf.Args)
	}
}
