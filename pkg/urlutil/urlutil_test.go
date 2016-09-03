package urlutil

import (
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

var (
	gitUrls = []string{
		"git://github.com/docker/docker",
		"git@github.com:docker/docker.git",
		"git@bitbucket.org:atlassianlabs/atlassian-docker.git",
		"https://github.com/docker/docker.git",
		"http://github.com/docker/docker.git",
		"http://github.com/docker/docker.git#branch",
		"http://github.com/docker/docker.git#:dir",
	}
	incompleteGitUrls = []string{
		"github.com/docker/docker",
	}
	invalidGitUrls = []string{
		"http://github.com/docker/docker.git:#branch",
	}
	transportUrls = []string{
		"tcp://example.com",
		"tcp+tls://example.com",
		"udp://example.com",
		"unix:///example",
		"unixgram:///example",
	}
)

func (s *DockerSuite) TestValidGitTransport(c *check.C) {
	for _, url := range gitUrls {
		if IsGitTransport(url) == false {
			c.Fatalf("%q should be detected as valid Git prefix", url)
		}
	}

	for _, url := range incompleteGitUrls {
		if IsGitTransport(url) == true {
			c.Fatalf("%q should not be detected as valid Git prefix", url)
		}
	}
}

func (s *DockerSuite) TestIsGIT(c *check.C) {
	for _, url := range gitUrls {
		if IsGitURL(url) == false {
			c.Fatalf("%q should be detected as valid Git url", url)
		}
	}

	for _, url := range incompleteGitUrls {
		if IsGitURL(url) == false {
			c.Fatalf("%q should be detected as valid Git url", url)
		}
	}

	for _, url := range invalidGitUrls {
		if IsGitURL(url) == true {
			c.Fatalf("%q should not be detected as valid Git prefix", url)
		}
	}
}

func (s *DockerSuite) TestIsTransport(c *check.C) {
	for _, url := range transportUrls {
		if IsTransportURL(url) == false {
			c.Fatalf("%q should be detected as valid Transport url", url)
		}
	}
}
