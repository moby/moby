package ps

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"text/template"

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

func customFormat(ctx Context, containers []types.Container) {
	var (
		table  bool
		header string
		format = ctx.Format
		buffer = bytes.NewBufferString("")
	)

	if strings.HasPrefix(ctx.Format, tableKey) {
		table = true
		format = format[len(tableKey):]
	}

	format = strings.Trim(format, " ")
	r := strings.NewReplacer(`\t`, "\t", `\n`, "\n")
	format = r.Replace(format)

	if table && ctx.Size {
		format += "\t{{.Size}}"
	}

	tmpl, err := template.New("").Parse(format)
	if err != nil {
		buffer.WriteString(fmt.Sprintf("Template parsing error: %v\n", err))
		buffer.WriteTo(ctx.Output)
		return
	}

	for _, container := range containers {
		containerCtx := &containerContext{
			trunc: ctx.Trunc,
			c:     container,
		}
		if err := tmpl.Execute(buffer, containerCtx); err != nil {
			buffer = bytes.NewBufferString(fmt.Sprintf("Template parsing error: %v\n", err))
			buffer.WriteTo(ctx.Output)
			return
		}
		if table && len(header) == 0 {
			header = containerCtx.fullHeader()
		}
		buffer.WriteString("\n")
	}

	if table {
		if len(header) == 0 {
			// if we still don't have a header, we didn't have any containers so we need to fake it to get the right headers from the template
			containerCtx := &containerContext{}
			tmpl.Execute(bytes.NewBufferString(""), containerCtx)
			header = containerCtx.fullHeader()
		}

		t := tabwriter.NewWriter(ctx.Output, 20, 1, 3, ' ', 0)
		t.Write([]byte(header))
		t.Write([]byte("\n"))
		buffer.WriteTo(t)
		t.Flush()
	} else {
		buffer.WriteTo(ctx.Output)
	}
}
