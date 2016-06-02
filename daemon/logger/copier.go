package logger

import (
	"fmt"
	"io"
)

// Copier can copy logs from specified sources to Logger
type Copier interface {
	Run()
	Wait()
	Close()
}

// NewCopier creates a new Copier
func NewCopier(srcs map[string]io.Reader, dst Logger) (Copier, error) {
	if ml, ok := dst.(MessageLogger); ok {
		return NewMessageCopier(srcs, ml), nil
	} else if rl, ok := dst.(RawLogger); ok {
		return NewRawCopier(srcs, rl), nil
	}
	return nil, fmt.Errorf("strange logger %s", dst.Name())
}
