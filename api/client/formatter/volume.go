package formatter

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/docker/engine-api/types"
)

const (
	defaultVolumeQuietFormat = "{{.Name}}"
	defaultVolumeTableFormat = "table {{.Driver}}\t{{.Name}}"

	mountpointHeader = "MOUNTPOINT"
	// Status header ?
)

// VolumeContext contains volume specific information required by the formatter,
// encapsulate a Context struct.
type VolumeContext struct {
	Context
	// Volumes
	Volumes []*types.Volume
}

func (ctx VolumeContext) Write() {
	switch ctx.Format {
	case tableFormatKey:
		if ctx.Quiet {
			ctx.Format = defaultVolumeQuietFormat
		} else {
			ctx.Format = defaultVolumeTableFormat
		}
	case rawFormatKey:
		if ctx.Quiet {
			ctx.Format = `name: {{.Name}}`
		} else {
			ctx.Format = `name: {{.Name}}\ndriver: {{.Driver}}\n`
		}
	}

	ctx.buffer = bytes.NewBufferString("")
	ctx.preformat()

	tmpl, err := ctx.parseFormat()
	if err != nil {
		return
	}

	for _, volume := range ctx.Volumes {
		volumeCtx := &volumeContext{
			v: volume,
		}
		err = ctx.contextFormat(tmpl, volumeCtx)
		if err != nil {
			return
		}
	}

	ctx.postformat(tmpl, &networkContext{})
}

type volumeContext struct {
	baseSubContext
	v *types.Volume
}

func (c *volumeContext) Name() string {
	c.addHeader(nameHeader)
	return c.v.Name
}

func (c *volumeContext) Driver() string {
	c.addHeader(driverHeader)
	return c.v.Driver
}

func (c *volumeContext) Scope() string {
	c.addHeader(scopeHeader)
	return c.v.Scope
}

func (c *volumeContext) Mountpoint() string {
	c.addHeader(mountpointHeader)
	return c.v.Mountpoint
}

func (c *volumeContext) Labels() string {
	c.addHeader(labelsHeader)
	if c.v.Labels == nil {
		return ""
	}

	var joinLabels []string
	for k, v := range c.v.Labels {
		joinLabels = append(joinLabels, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(joinLabels, ",")
}

func (c *volumeContext) Label(name string) string {

	n := strings.Split(name, ".")
	r := strings.NewReplacer("-", " ", "_", " ")
	h := r.Replace(n[len(n)-1])

	c.addHeader(h)

	if c.v.Labels == nil {
		return ""
	}
	return c.v.Labels[name]
}
