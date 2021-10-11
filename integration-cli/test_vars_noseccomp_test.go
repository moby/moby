//go:build !seccomp
// +build !seccomp

package main

const (
	// indicates docker daemon built with seccomp support
	supportsSeccomp = false
)
