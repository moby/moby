package client

import (
	"os"
	"testing"

	"github.com/docker/docker/registry"
	registrytypes "github.com/docker/engine-api/types/registry"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func unsetENV() {
	os.Unsetenv("DOCKER_CONTENT_TRUST")
	os.Unsetenv("DOCKER_CONTENT_TRUST_SERVER")
}

func (s *DockerSuite) TestENVTrustServer(c *check.C) {
	defer unsetENV()
	indexInfo := &registrytypes.IndexInfo{Name: "testserver"}
	if err := os.Setenv("DOCKER_CONTENT_TRUST_SERVER", "https://notary-test.com:5000"); err != nil {
		c.Fatal("Failed to set ENV variable")
	}
	output, err := trustServer(indexInfo)
	expectedStr := "https://notary-test.com:5000"
	if err != nil || output != expectedStr {
		c.Fatalf("Expected server to be %s, got %s", expectedStr, output)
	}
}

func (s *DockerSuite) TestHTTPENVTrustServer(c *check.C) {
	defer unsetENV()
	indexInfo := &registrytypes.IndexInfo{Name: "testserver"}
	if err := os.Setenv("DOCKER_CONTENT_TRUST_SERVER", "http://notary-test.com:5000"); err != nil {
		c.Fatal("Failed to set ENV variable")
	}
	_, err := trustServer(indexInfo)
	if err == nil {
		c.Fatal("Expected error with invalid scheme")
	}
}

func (s *DockerSuite) TestOfficialTrustServer(c *check.C) {
	indexInfo := &registrytypes.IndexInfo{Name: "testserver", Official: true}
	output, err := trustServer(indexInfo)
	if err != nil || output != registry.NotaryServer {
		c.Fatalf("Expected server to be %s, got %s", registry.NotaryServer, output)
	}
}

func (s *DockerSuite) TestNonOfficialTrustServer(c *check.C) {
	indexInfo := &registrytypes.IndexInfo{Name: "testserver", Official: false}
	output, err := trustServer(indexInfo)
	expectedStr := "https://" + indexInfo.Name
	if err != nil || output != expectedStr {
		c.Fatalf("Expected server to be %s, got %s", expectedStr, output)
	}
}
