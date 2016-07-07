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

func isDangling(image types.Image) bool {
	return len(image.RepoTags) == 1 && image.RepoTags[0] == "<none>:<none>" && len(image.RepoDigests) == 1 && image.RepoDigests[0] == "<none>@<none>"
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
		images := []*imageContext{}
		if isDangling(image) {
			images = append(images, &imageContext{
				trunc:  ctx.Trunc,
				i:      image,
				repo:   "<none>",
				tag:    "<none>",
				digest: "<none>",
			})
		} else {
			repoTags := map[string][]string{}
			repoDigests := map[string][]string{}

			for _, refString := range append(image.RepoTags) {
				ref, err := reference.ParseNamed(refString)
				if err != nil {
					continue
				}
				if nt, ok := ref.(reference.NamedTagged); ok {
					repoTags[ref.Name()] = append(repoTags[ref.Name()], nt.Tag())
				}
			}
			for _, refString := range append(image.RepoDigests) {
				ref, err := reference.ParseNamed(refString)
				if err != nil {
					continue
				}
				if c, ok := ref.(reference.Canonical); ok {
					repoDigests[ref.Name()] = append(repoDigests[ref.Name()], c.Digest().String())
				}
			}

			for repo, tags := range repoTags {
				digests := repoDigests[repo]

				// Do not display digests as their own row
				delete(repoDigests, repo)

				if !ctx.Digest {
					// Ignore digest references, just show tag once
					digests = nil
				}

				for _, tag := range tags {
					if len(digests) == 0 {
						images = append(images, &imageContext{
							trunc:  ctx.Trunc,
							i:      image,
							repo:   repo,
							tag:    tag,
							digest: "<none>",
						})
						continue
					}
					// Display the digests for each tag
					for _, dgst := range digests {
						images = append(images, &imageContext{
							trunc:  ctx.Trunc,
							i:      image,
							repo:   repo,
							tag:    tag,
							digest: dgst,
						})
					}

				}
			}

			// Show rows for remaining digest only references
			for repo, digests := range repoDigests {
				// If digests are displayed, show row per digest
				if ctx.Digest {
					for _, dgst := range digests {
						images = append(images, &imageContext{
							trunc:  ctx.Trunc,
							i:      image,
							repo:   repo,
							tag:    "<none>",
							digest: dgst,
						})
					}
				} else {
					images = append(images, &imageContext{
						trunc: ctx.Trunc,
						i:     image,
						repo:  repo,
						tag:   "<none>",
					})
				}
			}
		}
		for _, imageCtx := range images {
			err = ctx.contextFormat(tmpl, imageCtx)
			if err != nil {
				return
			}
		}
	}

	ctx.postformat(tmpl, &imageContext{})
}
