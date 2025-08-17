package daemon

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/v2/dockerversion"
)

func TestFillLicense(t *testing.T) {
	v := &system.Info{}
	d := &Daemon{
		root: "/var/lib/docker/",
	}
	d.fillLicense(v)
	assert.Assert(t, v.ProductLicense == dockerversion.DefaultProductLicense)
}
