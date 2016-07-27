package formatter

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/docker/engine-api/types"
	"github.com/docker/go-units"
	"github.com/fatih/color"
)

const (
	tableKey = "table"

	containerIDHeader  = "CONTAINER ID"
	imageHeader        = "IMAGE"
	namesHeader        = "NAMES"
	commandHeader      = "COMMAND"
	createdSinceHeader = "CREATED"
	createdAtHeader    = "CREATED AT"
	runningForHeader   = "CREATED"
	statusHeader       = "STATUS"
	portsHeader        = "PORTS"
	sizeHeader         = "SIZE"
	labelsHeader       = "LABELS"
	imageIDHeader      = "IMAGE ID"
	repositoryHeader   = "REPOSITORY"
	tagHeader          = "TAG"
	digestHeader       = "DIGEST"
	mountsHeader       = "MOUNTS"
)

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
			name = stringutils.Truncate(name, 15)
		}
		mounts = append(mounts, name)
	}
	return strings.Join(mounts, ",")
}

type imageContext struct {
	baseSubContext
	trunc  bool
	i      types.Image
	repo   string
	tag    string
	digest string
}

func (c *imageContext) ID() string {
	c.addHeader(imageIDHeader)
	if c.trunc {
		return stringid.TruncateID(c.i.ID)
	}
	return c.i.ID
}

func (c *imageContext) Repository() string {
	c.addHeader(repositoryHeader)
	return c.repo
}

func (c *imageContext) Tag() string {
	c.addHeader(tagHeader)
	return c.tag
}

func (c *imageContext) Digest() string {
	c.addHeader(digestHeader)
	return c.digest
}

func (c *imageContext) CreatedSince() string {
	c.addHeader(createdSinceHeader)
	createdAt := time.Unix(int64(c.i.Created), 0)
	return units.HumanDuration(time.Now().UTC().Sub(createdAt))
}

func (c *imageContext) CreatedAt() string {
	c.addHeader(createdAtHeader)
	return time.Unix(int64(c.i.Created), 0).String()
}

func (c *imageContext) Size() string {
	c.addHeader(sizeHeader)
	return units.HumanSize(float64(c.i.Size))
}

type subContext interface {
	fullHeader() string
	addHeader(header string)
}

type baseSubContext struct {
	header []string
}

func (c *baseSubContext) fullHeader() string {
	if c.header == nil {
		return ""
	}
	return strings.Join(c.header, "\t")
}

func (c *baseSubContext) addHeader(header string) {
	if c.header == nil {
		c.header = []string{}
	}
	c.header = append(c.header, strings.ToUpper(header))
}

func stripNamePrefix(ss []string) []string {
	sss := make([]string, len(ss))
	for i, s := range ss {
		sss[i] = s[1:]
	}

	return sss
}

// colors

// Base

func (c *containerContext) Reset(s string) string {
	return color.New(color.Reset).SprintFunc()(s)
}

func (c *containerContext) Bold(s string) string {
	return color.New(color.Bold).SprintFunc()(s)
}

func (c *containerContext) Faint(s string) string {
	return color.New(color.Faint).SprintFunc()(s)
}

func (c *containerContext) Italic(s string) string {
	return color.New(color.Italic).SprintFunc()(s)
}

func (c *containerContext) Underline(s string) string {
	return color.New(color.Underline).SprintFunc()(s)
}

func (c *containerContext) BlinkSlow(s string) string {
	return color.New(color.BlinkSlow).SprintFunc()(s)
}

func (c *containerContext) BlinkRapide(s string) string {
	return color.New(color.BlinkRapid).SprintFunc()(s)
}

func (c *containerContext) ReverseVideo(s string) string {
	return color.New(color.ReverseVideo).SprintFunc()(s)
}

func (c *containerContext) Concealed(s string) string {
	return color.New(color.Concealed).SprintFunc()(s)
}

func (c *containerContext) CrossedOut(s string) string {
	return color.New(color.CrossedOut).SprintFunc()(s)
}

// Foreground

func (c *containerContext) Black(s string) string {
	return color.BlackString(s)
}

func (c *containerContext) Red(s string) string {
	return color.RedString(s)
}

func (c *containerContext) Green(s string) string {
	return color.GreenString(s)
}

func (c *containerContext) Yellow(s string) string {
	return color.YellowString(s)
}

func (c *containerContext) Blue(s string) string {
	return color.BlueString(s)
}

func (c *containerContext) Magenta(s string) string {
	return color.MagentaString(s)
}

func (c *containerContext) Cyan(s string) string {
	return color.CyanString(s)
}

func (c *containerContext) White(s string) string {
	return color.CyanString(s)
}

// Foreground Hi-Intensity text colors

func (c *containerContext) HiBlack(s string) string {
	return color.New(color.FgHiBlack).SprintFunc()(s)
}

func (c *containerContext) HiRed(s string) string {
	return color.New(color.FgHiRed).SprintFunc()(s)
}

func (c *containerContext) HiGreen(s string) string {
	return color.New(color.FgHiGreen).SprintFunc()(s)
}

func (c *containerContext) HiYellow(s string) string {
	return color.New(color.FgHiYellow).SprintFunc()(s)
}

func (c *containerContext) HiBlue(s string) string {
	return color.New(color.FgHiBlue).SprintFunc()(s)
}

func (c *containerContext) HiMagenta(s string) string {
	return color.New(color.FgHiMagenta).SprintFunc()(s)
}

func (c *containerContext) HiCyan(s string) string {
	return color.New(color.FgHiCyan).SprintFunc()(s)
}

func (c *containerContext) HiWhite(s string) string {
	return color.New(color.FgHiWhite).SprintFunc()(s)
}

// Background BgBlack colors

func (c *containerContext) BgBlack(s string) string {
	return color.New(color.BgBlack).SprintFunc()(s)
}

func (c *containerContext) BgRed(s string) string {
	return color.New(color.BgRed).SprintFunc()(s)
}

func (c *containerContext) BgGreen(s string) string {
	return color.New(color.BgGreen).SprintFunc()(s)
}

func (c *containerContext) BgYellow(s string) string {
	return color.New(color.BgYellow).SprintFunc()(s)
}

func (c *containerContext) BgBlue(s string) string {
	return color.New(color.BgBlue).SprintFunc()(s)
}

func (c *containerContext) BgMagenta(s string) string {
	return color.New(color.BgMagenta).SprintFunc()(s)
}

func (c *containerContext) BgCyan(s string) string {
	return color.New(color.BgCyan).SprintFunc()(s)
}

func (c *containerContext) BgWhite(s string) string {
	return color.New(color.BgWhite).SprintFunc()(s)
}

// Background Hi-Intensity text colors

func (c *containerContext) BgHiBlack(s string) string {
	return color.New(color.BgHiBlack).SprintFunc()(s)
}

func (c *containerContext) BgHiRed(s string) string {
	return color.New(color.BgHiRed).SprintFunc()(s)
}

func (c *containerContext) BgHiGreen(s string) string {
	return color.New(color.BgHiGreen).SprintFunc()(s)
}

func (c *containerContext) BgHiYellow(s string) string {
	return color.New(color.BgHiYellow).SprintFunc()(s)
}

func (c *containerContext) BgHiBlue(s string) string {
	return color.New(color.BgHiBlue).SprintFunc()(s)
}

func (c *containerContext) BgHiMagenta(s string) string {
	return color.New(color.BgHiMagenta).SprintFunc()(s)
}

func (c *containerContext) BgHiCyan(s string) string {
	return color.New(color.BgHiCyan).SprintFunc()(s)
}

func (c *containerContext) BgHiWhite(s string) string {
	return color.New(color.BgHiWhite).SprintFunc()(s)
}
