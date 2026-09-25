// Copyright IBM Corp. 2013, 2025
// SPDX-License-Identifier: MIT

//go:build windows

package metrics

import (
	"syscall"
)

const (
	// DefaultSignal is used with DefaultInmemSignal
	// Windows has no SIGUSR1, use SIGBREAK
	DefaultSignal = syscall.Signal(21)
)
