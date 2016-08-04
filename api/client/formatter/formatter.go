package formatter

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/docker/docker/utils/templates"
)

const (
	tableFormatKey = "table"
	rawFormatKey   = "raw"

	defaultQuietFormat = "{{.ID}}"
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
