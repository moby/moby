package filters

import (
	"errors"
	"fmt"
	"io"
)

var DefaultFilterProcs = FilterProcSet{}

func Register(name string, fp FilterProc) error {
	return DefaultFilterProcs.Register(name, fp)
}

var ErrorFilterExists = errors.New("filter already exists and ")
var ErrorFilterExistsConflict = errors.New("filter already exists and FilterProc are different")

type FilterProcSet map[string]FilterProc

func (fs FilterProcSet) Process(context string) {
}

func (fs FilterProcSet) Register(name string, fp FilterProc) error {
	if v, ok := fs[name]; ok {
		if v == fp {
			return ErrorFilterExists
		} else {
			return ErrorFilterExistsConflict
		}
	}
	fs[name] = fp
	return nil
}

type FilterProc interface {
	Process(context, key, value string, output io.Writer) error
}

type UnknownFilterProc struct{}

func (ufp UnknownFilterProc) Process(context, key, value string, output io.Writer) error {
	if output != nil {
		fmt.Fprintf(output, "do not know how to process [%s : %s]", key, value)
	}
	return nil
}

type Filter interface {
	Scope() string
	Target() string
	Expressions() []string
	Match(interface{}) bool
}
