package formatter

import (
	"fmt"
	"strings"

	"github.com/docker/docker/cli/command/formatter"
	"github.com/docker/docker/components/volume/types"
)

const (
	defaultVolumeQuietFormat = "{{.Name}}"
	defaultVolumeTableFormat = "table {{.Driver}}\t{{.Name}}"

	nameHeader       = "NAME"
	driverHeader     = "DRIVER"
	mountpointHeader = "MOUNTPOINT"
	scopeHeader      = "SCOPE"
	labelsHeader     = "LABELS"
)

// NewVolumeFormat returns a format for use with a volume Context
func NewVolumeFormat(source string, quiet bool) formatter.Format {
	switch source {
	case formatter.TableFormatKey:
		if quiet {
			return defaultVolumeQuietFormat
		}
		return defaultVolumeTableFormat
	case formatter.RawFormatKey:
		if quiet {
			return `name: {{.Name}}`
		}
		return `name: {{.Name}}\ndriver: {{.Driver}}\n`
	}
	return formatter.Format(source)
}

// VolumeWrite writes formatted volumes using the Context
func VolumeWrite(ctx formatter.Context, volumes []*types.Volume) error {
	render := func(format func(subContext formatter.SubContext) error) error {
		for _, volume := range volumes {
			if err := format(&volumeContext{v: *volume}); err != nil {
				return err
			}
		}
		return nil
	}
	return ctx.Write(&volumeContext{}, render)
}

type volumeContext struct {
	formatter.HeaderContext
	v types.Volume
}

func (c *volumeContext) Name() string {
	c.AddHeader(nameHeader)
	return c.v.Name
}

func (c *volumeContext) Driver() string {
	c.AddHeader(driverHeader)
	return c.v.Driver
}

func (c *volumeContext) Scope() string {
	c.AddHeader(scopeHeader)
	return c.v.Scope
}

func (c *volumeContext) Mountpoint() string {
	c.AddHeader(mountpointHeader)
	return c.v.Mountpoint
}

func (c *volumeContext) Labels() string {
	c.AddHeader(labelsHeader)
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

	c.AddHeader(h)

	if c.v.Labels == nil {
		return ""
	}
	return c.v.Labels[name]
}
