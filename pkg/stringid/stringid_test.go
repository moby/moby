package stringid

import (
	"strings"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestGenerateRandomID(c *check.C) {
	id := GenerateRandomID()

	if len(id) != 64 {
		c.Fatalf("Id returned is incorrect: %s", id)
	}
}

func (s *DockerSuite) TestGenerateNonCryptoID(c *check.C) {
	id := GenerateNonCryptoID()

	if len(id) != 64 {
		c.Fatalf("Id returned is incorrect: %s", id)
	}
}

func (s *DockerSuite) TestShortenId(c *check.C) {
	id := "90435eec5c4e124e741ef731e118be2fc799a68aba0466ec17717f24ce2ae6a2"
	truncID := TruncateID(id)
	if truncID != "90435eec5c4e" {
		c.Fatalf("Id returned is incorrect: truncate on %s returned %s", id, truncID)
	}
}

func (s *DockerSuite) TestShortenSha256Id(c *check.C) {
	id := "sha256:4e38e38c8ce0b8d9041a9c4fefe786631d1416225e13b0bfe8cfa2321aec4bba"
	truncID := TruncateID(id)
	if truncID != "4e38e38c8ce0" {
		c.Fatalf("Id returned is incorrect: truncate on %s returned %s", id, truncID)
	}
}

func (s *DockerSuite) TestShortenIdEmpty(c *check.C) {
	id := ""
	truncID := TruncateID(id)
	if len(truncID) > len(id) {
		c.Fatalf("Id returned is incorrect: truncate on %s returned %s", id, truncID)
	}
}

func (s *DockerSuite) TestShortenIdInvalid(c *check.C) {
	id := "1234"
	truncID := TruncateID(id)
	if len(truncID) != len(id) {
		c.Fatalf("Id returned is incorrect: truncate on %s returned %s", id, truncID)
	}
}

func (s *DockerSuite) TestIsShortIDNonHex(c *check.C) {
	id := "some non-hex value"
	if IsShortID(id) {
		c.Fatalf("%s is not a short ID", id)
	}
}

func (s *DockerSuite) TestIsShortIDNotCorrectSize(c *check.C) {
	id := strings.Repeat("a", shortLen+1)
	if IsShortID(id) {
		c.Fatalf("%s is not a short ID", id)
	}
	id = strings.Repeat("a", shortLen-1)
	if IsShortID(id) {
		c.Fatalf("%s is not a short ID", id)
	}
}
