package formatter

import (
	"strings"

	"github.com/docker/docker/api/types/runtime"
)

const (
	defaultRuntimeTableFormat = "table {{.Name}}\t{{.Path}}\t{{.Args}}\t{{.Default}}"
	runtimeNameHeader         = "NAME"
	runtimePathHeader         = "PATH"
	runtimeArgsHeader         = "ARGS"
	runtimeDefaultHeader      = "DEFAULT"
)

// NewRuntimeFormat returns a Format for rendering
func NewRuntimeFormat(source string, quiet bool) Format {
	switch source {
	case TableFormatKey:
		if quiet {
			return defaultQuietFormat
		}
		return defaultRuntimeTableFormat
	}
	return Format(source)
}

// RuntimeWrite writes the context
func RuntimeWrite(ctx Context, runtimes []runtime.Info) error {
	render := func(format func(subContext subContext) error) error {
		for _, runtime := range runtimes {
			if err := format(&runtimeContext{r: runtime}); err != nil {
				return err
			}
		}
		return nil
	}
	return ctx.Write(newRuntimeContext(), render)
}

func newRuntimeContext() *runtimeContext {
	rCtx := &runtimeContext{}

	rCtx.header = map[string]string{
		"Name":    runtimeNameHeader,
		"Path":    runtimePathHeader,
		"Args":    runtimeArgsHeader,
		"Default": runtimeDefaultHeader,
	}
	return rCtx
}

type runtimeContext struct {
	HeaderContext
	r runtime.Info
}

func (c *runtimeContext) MarshalJSON() ([]byte, error) {
	return marshalJSON(c)
}

func (c *runtimeContext) Name() string {
	return c.r.Name
}

func (c *runtimeContext) Path() string {
	return c.r.Runtime.Path
}

func (c *runtimeContext) Args() string {
	return strings.Join(c.r.Runtime.Args, " ")
}

func (c *runtimeContext) Default() string {
	if c.r.DefaultRuntime {
		return "*"
	}

	return ""
}
