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

// Context contains information required by the formatter to print the output as desired.
type Context struct {
	// Output is the output stream to which the formatted string is written.
	Output io.Writer
	// Format is used to choose raw, table or custom format for the output.
	Format string
	// Size when set to true will display the size of the output.
	Size bool
	// Quiet when set to true will simply print minimal information.
	Quiet bool
	// Trunc when set to true will truncate the output of certain fields such as Container ID.
	Trunc bool
}

// Format helps to format the output using the parameters set in the Context.
// Currently Format allow to display in raw, table or custom format the output.
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
