package formatter

import (
	"fmt"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stringid"
	units "github.com/docker/go-units"
)

const (
	defaultImageTableFormat           = "table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.CreatedSince}} ago\t{{.Size}}"
	defaultImageTableFormatWithDigest = "table {{.Repository}}\t{{.Tag}}\t{{.Digest}}\t{{.ID}}\t{{.CreatedSince}} ago\t{{.Size}}"

	imageIDHeader    = "IMAGE ID"
	repositoryHeader = "REPOSITORY"
	tagHeader        = "TAG"
	digestHeader     = "DIGEST"
)

// ImageContext contains image specific information required by the formatter, encapsulate a Context struct.
type ImageContext struct {
	Context
	Digest bool
}

func isDangling(image types.ImageSummary) bool {
	return len(image.RepoTags) == 1 && image.RepoTags[0] == "<none>:<none>" && len(image.RepoDigests) == 1 && image.RepoDigests[0] == "<none>@<none>"
}

// NewImageFormat returns a format for rendering an ImageContext
func NewImageFormat(source string, quiet bool, digest bool) Format {
	switch source {
	case TableFormatKey:
		switch {
		case quiet:
			return defaultQuietFormat
		case digest:
			return defaultImageTableFormatWithDigest
		default:
			return defaultImageTableFormat
		}
	case RawFormatKey:
		switch {
		case quiet:
			return `image_id: {{.ID}}`
		case digest:
			return `repository: {{ .Repository }}
tag: {{.Tag}}
digest: {{.Digest}}
image_id: {{.ID}}
created_at: {{.CreatedAt}}
virtual_size: {{.Size}}
`
		default:
			return `repository: {{ .Repository }}
tag: {{.Tag}}
image_id: {{.ID}}
created_at: {{.CreatedAt}}
virtual_size: {{.Size}}
`
		}
	}

	format := Format(source)
	if format.IsTable() && digest && !format.Contains("{{.Digest}}") {
		format += "\t{{.Digest}}"
	}
	return format
}

// ImageWrite writes the formatter images using the ImageContext
func ImageWrite(ctx ImageContext, images []types.ImageSummary) error {
	render := func(format func(subContext subContext) error) error {
		return imageFormat(ctx, images, format)
	}
	return ctx.Write(&imageContext{}, render)
}

func imageFormat(ctx ImageContext, images []types.ImageSummary, format func(subContext subContext) error) error {
	for _, image := range images {
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

			for _, refString := range image.RepoTags {
				ref, err := reference.ParseNormalizedNamed(refString)
				if err != nil {
					continue
				}
				if nt, ok := ref.(reference.NamedTagged); ok {
					familiarRef := reference.FamiliarName(ref)
					repoTags[familiarRef] = append(repoTags[familiarRef], nt.Tag())
				}
			}
			for _, refString := range image.RepoDigests {
				ref, err := reference.ParseNormalizedNamed(refString)
				if err != nil {
					continue
				}
				if c, ok := ref.(reference.Canonical); ok {
					familiarRef := reference.FamiliarName(ref)
					repoDigests[familiarRef] = append(repoDigests[familiarRef], c.Digest().String())
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
			if err := format(imageCtx); err != nil {
				return err
			}
		}
	}
	return nil
}

type imageContext struct {
	HeaderContext
	trunc  bool
	i      types.ImageSummary
	repo   string
	tag    string
	digest string
}

func (c *imageContext) MarshalJSON() ([]byte, error) {
	return marshalJSON(c)
}

func (c *imageContext) ID() string {
	c.AddHeader(imageIDHeader)
	if c.trunc {
		return stringid.TruncateID(c.i.ID)
	}
	return c.i.ID
}

func (c *imageContext) Repository() string {
	c.AddHeader(repositoryHeader)
	return c.repo
}

func (c *imageContext) Tag() string {
	c.AddHeader(tagHeader)
	return c.tag
}

func (c *imageContext) Digest() string {
	c.AddHeader(digestHeader)
	return c.digest
}

func (c *imageContext) CreatedSince() string {
	c.AddHeader(createdSinceHeader)
	createdAt := time.Unix(int64(c.i.Created), 0)
	return units.HumanDuration(time.Now().UTC().Sub(createdAt))
}

func (c *imageContext) CreatedAt() string {
	c.AddHeader(createdAtHeader)
	return time.Unix(int64(c.i.Created), 0).String()
}

func (c *imageContext) Size() string {
	c.AddHeader(sizeHeader)
	return units.HumanSizeWithPrecision(float64(c.i.Size), 3)
}

func (c *imageContext) Containers() string {
	c.AddHeader(containersHeader)
	if c.i.Containers == -1 {
		return "N/A"
	}
	return fmt.Sprintf("%d", c.i.Containers)
}

func (c *imageContext) VirtualSize() string {
	c.AddHeader(sizeHeader)
	return units.HumanSize(float64(c.i.VirtualSize))
}

func (c *imageContext) SharedSize() string {
	c.AddHeader(sharedSizeHeader)
	if c.i.SharedSize == -1 {
		return "N/A"
	}
	return units.HumanSize(float64(c.i.SharedSize))
}

func (c *imageContext) UniqueSize() string {
	c.AddHeader(uniqueSizeHeader)
	if c.i.VirtualSize == -1 || c.i.SharedSize == -1 {
		return "N/A"
	}
	return units.HumanSize(float64(c.i.VirtualSize - c.i.SharedSize))
}
