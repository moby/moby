package formatter

import (
	"strconv"
)

const (
	defaultStackTableFormat = "table {{.Name}}\t{{.Services}}"

	stackServicesHeader = "SERVICES"
)

// Stack contains deployed stack information.
type Stack struct {
	// Name is the name of the stack
	Name string
	// Services is the number of the services
	Services int
}

// NewStackFormat returns a format for use with a stack Context
func NewStackFormat(source string) Format {
	switch source {
	case TableFormatKey:
		return defaultStackTableFormat
	}
	return Format(source)
}

// StackWrite writes formatted stacks using the Context
func StackWrite(ctx Context, stacks []*Stack) error {
	render := func(format func(subContext subContext) error) error {
		for _, stack := range stacks {
			if err := format(&stackContext{s: stack}); err != nil {
				return err
			}
		}
		return nil
	}
	return ctx.Write(newStackContext(), render)
}

type stackContext struct {
	HeaderContext
	s *Stack
}

func newStackContext() *stackContext {
	stackCtx := stackContext{}
	stackCtx.header = map[string]string{
		"Name":     nameHeader,
		"Services": stackServicesHeader,
	}
	return &stackCtx
}

func (s *stackContext) MarshalJSON() ([]byte, error) {
	return marshalJSON(s)
}

func (s *stackContext) Name() string {
	return s.s.Name
}

func (s *stackContext) Services() string {
	return strconv.Itoa(s.s.Services)
}
