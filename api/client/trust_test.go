package client

import (
	"os"
	"testing"

	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/registry"
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
	c.Assert(os.Setenv("DOCKER_CONTENT_TRUST_SERVER", "https://notary-test.com:5000"), check.IsNil)
	output, err := trustServer(indexInfo)
	expectedStr := "https://notary-test.com:5000"
	c.Assert(err, check.IsNil)
	c.Assert(output, check.Equals, expectedStr)
}

func (s *DockerSuite) TestHTTPENVTrustServer(c *check.C) {
	defer unsetENV()
	indexInfo := &registrytypes.IndexInfo{Name: "testserver"}
	c.Assert(os.Setenv("DOCKER_CONTENT_TRUST_SERVER", "http://notary-test.com:5000"), check.IsNil)
	_, err := trustServer(indexInfo)
	c.Assert(err, check.NotNil)
}

func (s *DockerSuite) TestOfficialTrustServer(c *check.C) {
	indexInfo := &registrytypes.IndexInfo{Name: "testserver", Official: true}
	output, err := trustServer(indexInfo)
	c.Assert(err, check.IsNil)
	c.Assert(output, check.Equals, registry.NotaryServer)
}

func (s *DockerSuite) TestNonOfficialTrustServer(c *check.C) {
	indexInfo := &registrytypes.IndexInfo{Name: "testserver", Official: false}
	output, err := trustServer(indexInfo)
	expectedStr := "https://" + indexInfo.Name
	c.Assert(err, check.IsNil)
	c.Assert(output, check.Equals, expectedStr)
}
