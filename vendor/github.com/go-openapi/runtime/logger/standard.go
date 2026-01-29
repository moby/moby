// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package logger

import (
	"fmt"
	"os"
)

var _ Logger = StandardLogger{}

type StandardLogger struct{}

func (StandardLogger) Printf(format string, args ...any) {
	if len(format) == 0 || format[len(format)-1] != '\n' {
		format += "\n"
	}
	fmt.Fprintf(os.Stderr, format, args...)
}

func (StandardLogger) Debugf(format string, args ...any) {
	if len(format) == 0 || format[len(format)-1] != '\n' {
		format += "\n"
	}
	fmt.Fprintf(os.Stderr, format, args...)
}
