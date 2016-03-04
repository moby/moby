package formatter

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/docker/docker/reference"
	"github.com/docker/docker/utils/templates"
	"github.com/docker/engine-api/types"
)

const (
	tableFormatKey = "table"
	rawFormatKey   = "raw"

	defaultContainerTableFormat       = "table {{.ID}}\t{{.Image}}\t{{.Command}}\t{{.RunningFor}} ago\t{{.Status}}\t{{.Ports}}\t{{.Names}}"
	defaultImageTableFormat           = "table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.CreatedSince}} ago\t{{.Size}}"
	defaultImageTableFormatWithDigest = "table {{.Repository}}\t{{.Tag}}\t{{.Digest}}\t{{.ID}}\t{{.CreatedSince}} ago\t{{.Size}}"
	defaultQuietFormat                = "{{.ID}}"
)

// Context contains information required by the formatter to print the output as desired.
type Context struct {
	// Output is the output stream to which the formatted string is written.
	Output io.Writer
	// Format is used to choose raw, table or custom format for the output.
	Format string
	// Quiet when set to true will simply print minimal information.
	Quiet bool
	// Trunc when set to true will truncate the output of certain fields such as Container ID.
	Trunc bool

	// internal element
	table       bool
	finalFormat string
	header      string
	buffer      *bytes.Buffer
}

func (c *Context) preformat() {
	c.finalFormat = c.Format

	if strings.HasPrefix(c.Format, tableKey) {
		c.table = true
		c.finalFormat = c.finalFormat[len(tableKey):]
	}

	c.finalFormat = strings.Trim(c.finalFormat, " ")
	r := strings.NewReplacer(`\t`, "\t", `\n`, "\n")
	c.finalFormat = r.Replace(c.finalFormat)
}

func (c *Context) parseFormat() (*template.Template, error) {
	tmpl, err := templates.Parse(c.finalFormat)
	if err != nil {
		c.buffer.WriteString(fmt.Sprintf("Template parsing error: %v\n", err))
		c.buffer.WriteTo(c.Output)
	}
	return tmpl, err
}

func (c *Context) postformat(tmpl *template.Template, subContext subContext) {
	if c.table {
		if len(c.header) == 0 {
			// if we still don't have a header, we didn't have any containers so we need to fake it to get the right headers from the template
			tmpl.Execute(bytes.NewBufferString(""), subContext)
			c.header = subContext.fullHeader()
		}

		t := tabwriter.NewWriter(c.Output, 20, 1, 3, ' ', 0)
		t.Write([]byte(c.header))
		t.Write([]byte("\n"))
		c.buffer.WriteTo(t)
		t.Flush()
	} else {
		c.buffer.WriteTo(c.Output)
	}
}

func (c *Context) contextFormat(tmpl *template.Template, subContext subContext) error {
	if err := tmpl.Execute(c.buffer, subContext); err != nil {
		c.buffer = bytes.NewBufferString(fmt.Sprintf("Template parsing error: %v\n", err))
		c.buffer.WriteTo(c.Output)
		return err
	}
	if c.table && len(c.header) == 0 {
		c.header = subContext.fullHeader()
	}
	c.buffer.WriteString("\n")
	return nil
}

// ContainerContext contains container specific information required by the formater, encapsulate a Context struct.
type ContainerContext struct {
	Context
	// Size when set to true will display the size of the output.
	Size bool
	// Containers
	Containers []types.Container
}

// ImageContext contains image specific information required by the formater, encapsulate a Context struct.
type ImageContext struct {
	Context
	Digest bool
	// Images
	Images []types.Image
}

func (ctx ContainerContext) Write() {
	switch ctx.Format {
	case tableFormatKey:
		ctx.Format = defaultContainerTableFormat
		if ctx.Quiet {
			ctx.Format = defaultQuietFormat
		}
	case rawFormatKey:
		if ctx.Quiet {
			ctx.Format = `container_id: {{.ID}}`
		} else {
			ctx.Format = `container_id: {{.ID}}
image: {{.Image}}
command: {{.Command}}
created_at: {{.CreatedAt}}
status: {{.Status}}
names: {{.Names}}
labels: {{.Labels}}
ports: {{.Ports}}
`
			if ctx.Size {
				ctx.Format += `size: {{.Size}}
`
			}
		}
	}

	ctx.buffer = bytes.NewBufferString("")
	ctx.preformat()
	if ctx.table && ctx.Size {
		ctx.finalFormat += "\t{{.Size}}"
	}

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

func (ctx ImageContext) Write() {
	switch ctx.Format {
	case tableFormatKey:
		ctx.Format = defaultImageTableFormat
		if ctx.Digest {
			ctx.Format = defaultImageTableFormatWithDigest
		}
		if ctx.Quiet {
			ctx.Format = defaultQuietFormat
		}
	case rawFormatKey:
		if ctx.Quiet {
			ctx.Format = `image_id: {{.ID}}`
		} else {
			if ctx.Digest {
				ctx.Format = `repository: {{ .Repository }}
tag: {{.Tag}}
digest: {{.Digest}}
image_id: {{.ID}}
created_at: {{.CreatedAt}}
virtual_size: {{.Size}}
`
			} else {
				ctx.Format = `repository: {{ .Repository }}
tag: {{.Tag}}
image_id: {{.ID}}
created_at: {{.CreatedAt}}
virtual_size: {{.Size}}
`
			}
		}
	}

	ctx.buffer = bytes.NewBufferString("")
	ctx.preformat()
	if ctx.table && ctx.Digest && !strings.Contains(ctx.Format, "{{.Digest}}") {
		ctx.finalFormat += "\t{{.Digest}}"
	}

	tmpl, err := ctx.parseFormat()
	if err != nil {
		return
	}

	for _, image := range ctx.Images {

		repoTags := image.RepoTags
		repoDigests := image.RepoDigests

		if len(repoTags) == 1 && repoTags[0] == "<none>:<none>" && len(repoDigests) == 1 && repoDigests[0] == "<none>@<none>" {
			// dangling image - clear out either repoTags or repoDigests so we only show it once below
			repoDigests = []string{}
		}
		// combine the tags and digests lists
		tagsAndDigests := append(repoTags, repoDigests...)
		for _, repoAndRef := range tagsAndDigests {
			repo := "<none>"
			tag := "<none>"
			digest := "<none>"

			if !strings.HasPrefix(repoAndRef, "<none>") {
				ref, err := reference.ParseNamed(repoAndRef)
				if err != nil {
					continue
				}
				repo = ref.Name()

				switch x := ref.(type) {
				case reference.Canonical:
					digest = x.Digest().String()
				case reference.NamedTagged:
					tag = x.Tag()
				}
			}
			imageCtx := &imageContext{
				trunc:  ctx.Trunc,
				i:      image,
				repo:   repo,
				tag:    tag,
				digest: digest,
			}
			err = ctx.contextFormat(tmpl, imageCtx)
			if err != nil {
				return
			}
		}
	}

	ctx.postformat(tmpl, &imageContext{})
}
