package stringutils

import (
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func testLengthHelper(generator func(int) string, c *check.C) {
	expectedLength := 20
	str := generator(expectedLength)
	if len(str) != expectedLength {
		c.Fatalf("Length of %s was %d but expected length %d", str, len(str), expectedLength)
	}
}

func testUniquenessHelper(generator func(int) string, c *check.C) {
	repeats := 25
	set := make(map[string]struct{}, repeats)
	for i := 0; i < repeats; i = i + 1 {
		str := generator(64)
		if len(str) != 64 {
			c.Fatalf("Id returned is incorrect: %s", str)
		}
		if _, ok := set[str]; ok {
			c.Fatalf("Random number is repeated")
		}
		set[str] = struct{}{}
	}
}

func isASCII(s string) bool {
	for _, c := range s {
		if c > 127 {
			return false
		}
	}
	return true
}

func (s *DockerSuite) TestGenerateRandomAlphaOnlyStringLength(c *check.C) {
	testLengthHelper(GenerateRandomAlphaOnlyString, c)
}

func (s *DockerSuite) TestGenerateRandomAlphaOnlyStringUniqueness(c *check.C) {
	testUniquenessHelper(GenerateRandomAlphaOnlyString, c)
}

func (s *DockerSuite) TestGenerateRandomAsciiStringLength(c *check.C) {
	testLengthHelper(GenerateRandomASCIIString, c)
}

func (s *DockerSuite) TestGenerateRandomAsciiStringUniqueness(c *check.C) {
	testUniquenessHelper(GenerateRandomASCIIString, c)
}

func (s *DockerSuite) TestGenerateRandomAsciiStringIsAscii(c *check.C) {
	str := GenerateRandomASCIIString(64)
	if !isASCII(str) {
		c.Fatalf("%s contained non-ascii characters", str)
	}
}

func (s *DockerSuite) TestEllipsis(c *check.C) {
	str := "t🐳ststring"
	newstr := Ellipsis(str, 3)
	if newstr != "t🐳s" {
		c.Fatalf("Expected t🐳s, got %s", newstr)
	}
	newstr = Ellipsis(str, 8)
	if newstr != "t🐳sts..." {
		c.Fatalf("Expected tests..., got %s", newstr)
	}
	newstr = Ellipsis(str, 20)
	if newstr != "t🐳ststring" {
		c.Fatalf("Expected t🐳ststring, got %s", newstr)
	}
}

func (s *DockerSuite) TestTruncate(c *check.C) {
	str := "t🐳ststring"
	newstr := Truncate(str, 4)
	if newstr != "t🐳st" {
		c.Fatalf("Expected t🐳st, got %s", newstr)
	}
	newstr = Truncate(str, 20)
	if newstr != "t🐳ststring" {
		c.Fatalf("Expected t🐳ststring, got %s", newstr)
	}
}

func (s *DockerSuite) TestInSlice(c *check.C) {
	slice := []string{"t🐳st", "in", "slice"}

	test := InSlice(slice, "t🐳st")
	if !test {
		c.Fatalf("Expected string t🐳st to be in slice")
	}
	test = InSlice(slice, "SLICE")
	if !test {
		c.Fatalf("Expected string SLICE to be in slice")
	}
	test = InSlice(slice, "notinslice")
	if test {
		c.Fatalf("Expected string notinslice not to be in slice")
	}
}

func (s *DockerSuite) TestShellQuoteArgumentsEmpty(c *check.C) {
	actual := ShellQuoteArguments([]string{})
	expected := ""
	if actual != expected {
		c.Fatalf("Expected an empty string")
	}
}

func (s *DockerSuite) TestShellQuoteArguments(c *check.C) {
	simpleString := "simpleString"
	complexString := "This is a 'more' complex $tring with some special char *"
	actual := ShellQuoteArguments([]string{simpleString, complexString})
	expected := "simpleString 'This is a '\\''more'\\'' complex $tring with some special char *'"
	if actual != expected {
		c.Fatalf("Expected \"%v\", got \"%v\"", expected, actual)
	}
}
