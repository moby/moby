// +build experimental

package bundlefile

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestLoadFileV01Success(c *check.C) {
	reader := strings.NewReader(`{
		"Version": "0.1",
		"Services": {
			"redis": {
				"Image": "redis@sha256:4b24131101fa0117bcaa18ac37055fffd9176aa1a240392bb8ea85e0be50f2ce",
				"Networks": ["default"]
			},
			"web": {
				"Image": "dockercloud/hello-world@sha256:fe79a2cfbd17eefc344fb8419420808df95a1e22d93b7f621a7399fd1e9dca1d",
				"Networks": ["default"],
				"User": "web"
			}
		}
	}`)

	bundle, err := LoadFile(reader)
	c.Assert(err, check.IsNil)
	c.Assert(bundle.Version, check.Equals, "0.1")
	c.Assert(len(bundle.Services), check.Equals, 2)
}

func (s *DockerSuite) TestLoadFileSyntaxError(c *check.C) {
	reader := strings.NewReader(`{
		"Version": "0.1",
		"Services": unquoted string
	}`)

	_, err := LoadFile(reader)
	c.Assert(err, check.ErrorMatches, ".*syntax error at byte 37: invalid character 'u'.*")
}

func (s *DockerSuite) TestLoadFileTypeError(c *check.C) {
	reader := strings.NewReader(`{
		"Version": "0.1",
		"Services": {
			"web": {
				"Image": "redis",
				"Networks": "none"
			}
		}
	}`)

	_, err := LoadFile(reader)
	c.Assert(err, check.ErrorMatches, ".*Unexpected type at byte 94. Expected \\[\\]string but received string.*")
}

func (s *DockerSuite) TestPrint(c *check.C) {
	var buffer bytes.Buffer
	bundle := &Bundlefile{
		Version: "0.1",
		Services: map[string]Service{
			"web": {
				Image:   "image",
				Command: []string{"echo", "something"},
			},
		},
	}
	c.Assert(Print(&buffer, bundle), check.IsNil)
	output := buffer.String()
	c.Assert(output, check.Matches, "(?s).*\"Image\": \"image\".*")
	c.Assert(output, check.Matches,
		`(?s).*"Command": \[
                "echo",
                "something"
            \].*`)
}
