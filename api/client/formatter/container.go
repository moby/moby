package formatter

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/docker/engine-api/types"
	"github.com/docker/go-units"
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
)

// ContainerContext contains container specific information required by the formater, encapsulate a Context struct.
type ContainerContext struct {
	Context
	// Size when set to true will display the size of the output.
	Size bool
	// Containers
	Containers []types.Container
}

func (ctx ContainerContext) Write() {
	switch ctx.Format {
	case tableFormatKey:
		if ctx.Quiet {
			ctx.Format = defaultQuietFormat
		} else {
			ctx.Format = defaultContainerTableFormat
			if ctx.Size {
				ctx.Format += `\t{{.Size}}`
			}
		}
	case rawFormatKey:
		if ctx.Quiet {
			ctx.Format = `container_id: {{.ID}}`
		} else {
			ctx.Format = `container_id: {{.ID}}\nimage: {{.Image}}\ncommand: {{.Command}}\ncreated_at: {{.CreatedAt}}\nstatus: {{.Status}}\nnames: {{.Names}}\nlabels: {{.Labels}}\nports: {{.Ports}}\n`
			if ctx.Size {
				ctx.Format += `size: {{.Size}}\n`
			}
		}
	}

	ctx.buffer = bytes.NewBufferString("")
	ctx.preformat()

	tmpl, err := ctx.parseFormat()
	if err != nil {
		return
	}

	for _, container := range ctx.Containers {
		containerCtx := &containerContext{
			trunc: ctx.Trunc,
			c:     container,
		}
		err = ctx.contextFormat(tmpl, containerCtx)
		if err != nil {
			return
		}
	}

	ctx.postformat(tmpl, &containerContext{})
}

type containerContext struct {
	baseSubContext
	trunc bool
	c     types.Container
}

func (c *containerContext) ID() string {
	c.addHeader(containerIDHeader)
	if c.trunc {
		return stringid.TruncateID(c.c.ID)
	}
	return c.c.ID
}

func (c *containerContext) Names() string {
	c.addHeader(namesHeader)
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
	c.addHeader(imageHeader)
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
	c.addHeader(commandHeader)
	command := c.c.Command
	if c.trunc {
		command = stringutils.Ellipsis(command, 20)
	}
	return strconv.Quote(command)
}

func (c *containerContext) CreatedAt() string {
	c.addHeader(createdAtHeader)
	return time.Unix(int64(c.c.Created), 0).String()
}

func (c *containerContext) RunningFor() string {
	c.addHeader(runningForHeader)
	createdAt := time.Unix(int64(c.c.Created), 0)
	return units.HumanDuration(time.Now().UTC().Sub(createdAt))
}

func (c *containerContext) Ports() string {
	c.addHeader(portsHeader)
	return api.DisplayablePorts(c.c.Ports)
}

func (c *containerContext) Status() string {
	c.addHeader(statusHeader)
	return c.c.Status
}

func (c *containerContext) Size() string {
	c.addHeader(sizeHeader)
	srw := units.HumanSize(float64(c.c.SizeRw))
	sv := units.HumanSize(float64(c.c.SizeRootFs))

	sf := srw
	if c.c.SizeRootFs > 0 {
		sf = fmt.Sprintf("%s (virtual %s)", srw, sv)
	}
	return sf
}

func (c *containerContext) Labels() string {
	c.addHeader(labelsHeader)
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

	c.addHeader(h)

	if c.c.Labels == nil {
		return ""
	}
	return c.c.Labels[name]
}

func (c *containerContext) Mounts() string {
	c.addHeader(mountsHeader)

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
