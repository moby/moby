package daemon // import "github.com/moby/moby/daemon"

import (
	"testing"

	"github.com/moby/moby/api/types"
	"github.com/moby/moby/dockerversion"
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
