// +build linux

package journald

import (
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestSanitizeKeyMod(c *check.C) {
	entries := map[string]string{
		"io.kubernetes.pod.name":      "IO_KUBERNETES_POD_NAME",
		"io?.kubernetes.pod.name":     "IO__KUBERNETES_POD_NAME",
		"?io.kubernetes.pod.name":     "IO_KUBERNETES_POD_NAME",
		"io123.kubernetes.pod.name":   "IO123_KUBERNETES_POD_NAME",
		"_io123.kubernetes.pod.name":  "IO123_KUBERNETES_POD_NAME",
		"__io123_kubernetes.pod.name": "IO123_KUBERNETES_POD_NAME",
	}
	for k, v := range entries {
		if sanitizeKeyMod(k) != v {
			c.Fatalf("Failed to sanitize %s, got %s, expected %s", k, sanitizeKeyMod(k), v)
		}
	}
}
