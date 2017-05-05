package formatter

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/archive"
)

const (
	defaultDiffTableFormat = "table {{.Type}}\t{{.Path}}"

	changeTypeHeader = "CHANGE TYPE"
	pathHeader       = "PATH"
)

// NewDiffFormat returns a format for use with a diff Context
func NewDiffFormat(source string) Format {
	switch source {
	case TableFormatKey:
		return defaultDiffTableFormat
	}
	return Format(source)
}

// DiffWrite writes formatted diff using the Context
func DiffWrite(ctx Context, changes []container.ContainerChangeResponseItem) error {

	render := func(format func(subContext subContext) error) error {
		for _, change := range changes {
			if err := format(&diffContext{c: change}); err != nil {
				return err
			}
		}
		return nil
	}
	return ctx.Write(newDiffContext(), render)
}

type diffContext struct {
	HeaderContext
	c container.ContainerChangeResponseItem
}

func newDiffContext() *diffContext {
	diffCtx := diffContext{}
	diffCtx.header = map[string]string{
		"Type": changeTypeHeader,
		"Path": pathHeader,
	}
	return &diffCtx
}

func (d *diffContext) MarshalJSON() ([]byte, error) {
	return marshalJSON(d)
}

func (d *diffContext) Type() string {
	var kind string
	switch d.c.Kind {
	case archive.ChangeModify:
		kind = "C"
	case archive.ChangeAdd:
		kind = "A"
	case archive.ChangeDelete:
		kind = "D"
	}
	return kind

}

func (d *diffContext) Path() string {
	return d.c.Path
}
