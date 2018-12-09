package daemon // import "github.com/docker/docker/daemon"

import (
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/dockerversion"
	"gotest.tools/assert"
)

func TestFillLicense(t *testing.T) {
	v := &types.Info{}
	d := &Daemon{
		root: "/var/lib/docker/",
	}
	d.fillLicense(v)
	assert.Assert(t, v.ProductLicense == dockerversion.DefaultProductLicense)
}
