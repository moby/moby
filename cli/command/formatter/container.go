package formatter

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/stringutils"
	units "github.com/docker/go-units"
)

const (
	defaultContainerTableFormat = "table {{.ID}}\t{{.Image}}\t{{.Command}}\t{{.RunningFor}} ago\t{{.Status}}\t{{.Ports}}\t{{.Names}}"

	containerIDHeader = "CONTAINER ID"
	namesHeader       = "NAMES"
	commandHeader     = "COMMAND"
	runningForHeader  = "CREATED"
	statusHeader      = "STATUS"
	portsHeader       = "PORTS"
	mountsHeader      = "MOUNTS"
	localVolumes      = "LOCAL VOLUMES"
)

// NewContainerFormat returns a Format for rendering using a Context
func NewContainerFormat(source string, quiet bool, size bool) Format {
	switch source {
	case TableFormatKey:
		if quiet {
			return defaultQuietFormat
		}
		format := defaultContainerTableFormat
		if size {
			format += `\t{{.Size}}`
		}
		return Format(format)
	case RawFormatKey:
		if quiet {
			return `container_id: {{.ID}}`
		}
		format := `container_id: {{.ID}}
image: {{.Image}}
command: {{.Command}}
created_at: {{.CreatedAt}}
status: {{- pad .Status 1 0}}
names: {{.Names}}
labels: {{- pad .Labels 1 0}}
ports: {{- pad .Ports 1 0}}
`
		if size {
			format += `size: {{.Size}}\n`
		}
		return Format(format)
	}
	return Format(source)
}

// ContainerWrite renders the context for a list of containers
func ContainerWrite(ctx Context, containers []types.Container) error {
	render := func(format func(subContext subContext) error) error {
		for _, container := range containers {
			err := format(&containerContext{trunc: ctx.Trunc, c: container})
			if err != nil {
				return err
			}
		}
		return nil
	}
	return ctx.Write(&containerContext{}, render)
}

type containerContext struct {
	HeaderContext
	trunc bool
	c     types.Container
}

func (c *containerContext) ID() string {
	c.AddHeader(containerIDHeader)
	if c.trunc {
		return stringid.TruncateID(c.c.ID)
	}
	return c.c.ID
}

func (c *containerContext) Names() string {
	c.AddHeader(namesHeader)
	names := stripNamePrefix(c.c.Names)
	if c.trunc {
		for _, name := range names {
			if len(strings.Split(name, "/")) == 1 {
				names = []string{name}
				break
			}
		}
	}
	return strings.Join(names, ",")
}

func (c *containerContext) Image() string {
	c.AddHeader(imageHeader)
	if c.c.Image == "" {
		return "<no image>"
	}
	if c.trunc {
		if trunc := stringid.TruncateID(c.c.ImageID); trunc == stringid.TruncateID(c.c.Image) {
			return trunc
		}
	}
	return c.c.Image
}

func (c *containerContext) Command() string {
	c.AddHeader(commandHeader)
	command := c.c.Command
	if c.trunc {
		command = stringutils.Ellipsis(command, 20)
	}
	return strconv.Quote(command)
}

func (c *containerContext) CreatedAt() string {
	c.AddHeader(createdAtHeader)
	return time.Unix(int64(c.c.Created), 0).String()
}

func (c *containerContext) RunningFor() string {
	c.AddHeader(runningForHeader)
	createdAt := time.Unix(int64(c.c.Created), 0)
	return units.HumanDuration(time.Now().UTC().Sub(createdAt))
}

func (c *containerContext) Ports() string {
	c.AddHeader(portsHeader)
	return api.DisplayablePorts(c.c.Ports)
}

func (c *containerContext) Status() string {
	c.AddHeader(statusHeader)
	return c.c.Status
}

func (c *containerContext) Size() string {
	c.AddHeader(sizeHeader)
	srw := units.HumanSizeWithPrecision(float64(c.c.SizeRw), 3)
	sv := units.HumanSizeWithPrecision(float64(c.c.SizeRootFs), 3)

	sf := srw
	if c.c.SizeRootFs > 0 {
		sf = fmt.Sprintf("%s (virtual %s)", srw, sv)
	}
	return sf
}

func (c *containerContext) Labels() string {
	c.AddHeader(labelsHeader)
	if c.c.Labels == nil {
		return ""
	}

	var joinLabels []string
	for k, v := range c.c.Labels {
		joinLabels = append(joinLabels, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(joinLabels, ",")
}

func (c *containerContext) Label(name string) string {
	n := strings.Split(name, ".")
	r := strings.NewReplacer("-", " ", "_", " ")
	h := r.Replace(n[len(n)-1])

	c.AddHeader(h)

	if c.c.Labels == nil {
		return ""
	}
	return c.c.Labels[name]
}

func (c *containerContext) Mounts() string {
	c.AddHeader(mountsHeader)

	var name string
	var mounts []string
	for _, m := range c.c.Mounts {
		if m.Name == "" {
			name = m.Source
		} else {
			name = m.Name
		}
		if c.trunc {
			name = stringutils.Ellipsis(name, 15)
		}
		mounts = append(mounts, name)
	}
	return strings.Join(mounts, ",")
}

func (c *containerContext) LocalVolumes() string {
	c.AddHeader(localVolumes)

	count := 0
	for _, m := range c.c.Mounts {
		if m.Driver == "local" {
			count++
		}
	}

	return fmt.Sprintf("%d", count)
}
