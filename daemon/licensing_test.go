package daemon

import (
	"testing"

	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/v2/dockerversion"
	"gotest.tools/v3/assert"
)

func TestFillLicense(t *testing.T) {
	v := &system.Info{}
	d := &Daemon{
		root: "/var/lib/docker/",
	}
	d.fillLicense(v)
	assert.Assert(t, v.ProductLicense == dockerversion.DefaultProductLicense)
}
