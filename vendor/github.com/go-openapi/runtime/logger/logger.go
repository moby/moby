// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package logger

import "os"

type Logger interface {
	Printf(format string, args ...any)
	Debugf(format string, args ...any)
}

func DebugEnabled() bool {
	d := os.Getenv("SWAGGER_DEBUG")
	if d != "" && d != "false" && d != "0" {
		return true
	}
	d = os.Getenv("DEBUG")
	if d != "" && d != "false" && d != "0" {
		return true
	}
	return false
}
