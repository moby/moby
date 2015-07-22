package ps

import (
	"io"

	"github.com/docker/docker/api/types"
)

const (
	tableFormatKey = "table"
	rawFormatKey   = "raw"

	defaultTableFormat = "table {{.ID}}\t{{.Image}}\t{{.Command}}\t{{.RunningFor}} ago\t{{.Status}}\t{{.Ports}}\t{{.Names}}"
	defaultQuietFormat = "{{.ID}}"
)

type Context struct {
	Output io.Writer
	Format string
	Size   bool
	Quiet  bool
	Trunc  bool
}

func Format(ctx Context, containers []types.Container) {
	switch ctx.Format {
	case tableFormatKey:
		tableFormat(ctx, containers)
	case rawFormatKey:
		rawFormat(ctx, containers)
	default:
		customFormat(ctx, containers)
	}
}

func rawFormat(ctx Context, containers []types.Container) {
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

	customFormat(ctx, containers)
}

func tableFormat(ctx Context, containers []types.Container) {
	ctx.Format = defaultTableFormat
	if ctx.Quiet {
		ctx.Format = defaultQuietFormat
	}

	customFormat(ctx, containers)
}
