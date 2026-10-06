// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package loads

import (
	"errors"
	"fmt"
)

type loaderError string

func (e loaderError) Error() string {
	return string(e)
}

const (
	// ErrLoads is an error returned by the loads package.
	ErrLoads loaderError = "cannot load spec"

	// ErrNoLoader indicates that no configured loader matched the input.
	ErrNoLoader loaderError = "no loader matched"

	// ErrForbiddenAddress is returned by [RestrictedHTTPClient] when a connection is attempted
	// to a non-public address (loopback, private, link-local, or unspecified).
	ErrForbiddenAddress loaderError = "blocked dial to a non-public address"
)

// errLoads marks err as an error from this package, so callers may test it with
// [errors.Is] against [ErrLoads].
//
// The cause is reported on a single line, after the sentinel. An error that already
// carries the sentinel is returned unchanged: loaders are chained, and reporting
// "cannot load spec" once per link in the chain would tell the caller nothing.
func errLoads(err error) error {
	if err == nil || errors.Is(err, ErrLoads) {
		return err
	}

	return fmt.Errorf("%w: %w", ErrLoads, err)
}
