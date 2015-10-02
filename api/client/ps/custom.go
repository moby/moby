package ps

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/docker/docker/pkg/units"
)

const (
	tableKey = "table"

	idHeader         = "CONTAINER ID"
	imageHeader      = "IMAGE"
	namesHeader      = "NAMES"
	commandHeader    = "COMMAND"
	createdAtHeader  = "CREATED AT"
	runningForHeader = "CREATED"
	statusHeader     = "STATUS"
	portsHeader      = "PORTS"
	sizeHeader       = "SIZE"
	labelsHeader     = "LABELS"
)

type containerContext struct {
	trunc  bool
	header []string
	c      types.Container
}

func (c *containerContext) ID() string {
	c.addHeader(idHeader)
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
		if stringid.TruncateID(c.c.ImageID) == stringid.TruncateID(c.c.Image) {
			return stringutils.Truncate(c.c.Image, 12)
		}
	}
	return c.c.Image
}

func (c *containerContext) Command() string {
	c.addHeader(commandHeader)
	command := c.c.Command
	if c.trunc {
		command = stringutils.Truncate(command, 20)
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

func (c *containerContext) fullHeader() string {
	if c.header == nil {
		return ""
	}
	return strings.Join(c.header, "\t")
}

func (c *containerContext) addHeader(header string) {
	if c.header == nil {
		c.header = []string{}
	}
	c.header = append(c.header, strings.ToUpper(header))
}

func stripNamePrefix(ss []string) []string {
	for i, s := range ss {
		ss[i] = s[1:]
	}

	return ss
}
