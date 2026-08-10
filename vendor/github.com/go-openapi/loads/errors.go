// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package loads

type loaderError string

func (e loaderError) Error() string {
	return string(e)
}

const (
	// ErrLoads is an error returned by the loads package.
	ErrLoads loaderError = "loaderrs error"

	// ErrNoLoader indicates that no configured loader matched the input.
	ErrNoLoader loaderError = "no loader matched"

	// ErrForbiddenAddress is returned by [RestrictedHTTPClient] when a connection is attempted
	// to a non-public address (loopback, private, link-local, or unspecified).
	ErrForbiddenAddress loaderError = "blocked dial to a non-public address"
)
