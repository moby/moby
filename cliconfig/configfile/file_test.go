package configfile

import (
	"testing"

	"github.com/docker/engine-api/types"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestEncodeAuth(c *check.C) {
	newAuthConfig := &types.AuthConfig{Username: "ken", Password: "test"}
	authStr := encodeAuth(newAuthConfig)
	decAuthConfig := &types.AuthConfig{}
	var err error
	decAuthConfig.Username, decAuthConfig.Password, err = decodeAuth(authStr)
	if err != nil {
		c.Fatal(err)
	}
	if newAuthConfig.Username != decAuthConfig.Username {
		c.Fatal("Encode Username doesn't match decoded Username")
	}
	if newAuthConfig.Password != decAuthConfig.Password {
		c.Fatal("Encode Password doesn't match decoded Password")
	}
	if authStr != "a2VuOnRlc3Q=" {
		c.Fatal("AuthString encoding isn't correct.")
	}
}
