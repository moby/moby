package formatter

import (
	"bytes"
	"strings"
	"time"

	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/reference"
	"github.com/docker/engine-api/types"
	"github.com/docker/go-units"
)

const (
	defaultImageTableFormat           = "table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.CreatedSince}} ago\t{{.Size}}"
	defaultImageTableFormatWithDigest = "table {{.Repository}}\t{{.Tag}}\t{{.Digest}}\t{{.ID}}\t{{.CreatedSince}} ago\t{{.Size}}"

	imageIDHeader    = "IMAGE ID"
	repositoryHeader = "REPOSITORY"
	tagHeader        = "TAG"
	digestHeader     = "DIGEST"
)

// ImageContext contains image specific information required by the formater, encapsulate a Context struct.
type ImageContext struct {
	Context
	Digest bool
	// Images
	Images []types.Image
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
