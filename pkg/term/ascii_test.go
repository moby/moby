package term

import (
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestToBytes(c *check.C) {
	codes, err := ToBytes("ctrl-a,a")
	if err != nil {
		c.Fatal(err)
	}
	if len(codes) != 2 {
		c.Fatalf("Expected 2 codes, got %d", len(codes))
	}
	if codes[0] != 1 || codes[1] != 97 {
		c.Fatalf("Expected '1' '97', got '%d' '%d'", codes[0], codes[1])
	}

	codes, err = ToBytes("shift-z")
	if err == nil {
		c.Fatalf("Expected error, got none")
	}

	codes, err = ToBytes("ctrl-@,ctrl-[,~,ctrl-o")
	if err != nil {
		c.Fatal(err)
	}
	if len(codes) != 4 {
		c.Fatalf("Expected 4 codes, got %d", len(codes))
	}
	if codes[0] != 0 || codes[1] != 27 || codes[2] != 126 || codes[3] != 15 {
		c.Fatalf("Expected '0' '27' '126', '15', got '%d' '%d' '%d' '%d'", codes[0], codes[1], codes[2], codes[3])
	}

	codes, err = ToBytes("DEL,+")
	if err != nil {
		c.Fatal(err)
	}
	if len(codes) != 2 {
		c.Fatalf("Expected 2 codes, got %d", len(codes))
	}
	if codes[0] != 127 || codes[1] != 43 {
		c.Fatalf("Expected '127 '43'', got '%d' '%d'", codes[0], codes[1])
	}
}
