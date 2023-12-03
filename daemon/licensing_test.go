package daemon // import "github.com/docker/docker/daemon"

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/dockerversion"
)

func TestFillLicense(t *testing.T) {
	v := &system.Info{}
	d := &Daemon{
		root: "/var/lib/docker/",
	}
	d.fillLicense(v)
	assert.Assert(t, v.ProductLicense == dockerversion.DefaultProductLicense)
}
