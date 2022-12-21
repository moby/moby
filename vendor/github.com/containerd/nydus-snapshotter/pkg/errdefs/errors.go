/*
 * Copyright (c) 2020. Ant Group. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package errdefs

import (
	stderrors "errors"
	"net"
	"strings"
	"syscall"

	"github.com/pkg/errors"
)

const signalKilled = "signal: killed"

var (
	ErrAlreadyExists = errors.New("already exists")
	ErrNotFound      = errors.New("not found")
)

// IsAlreadyExists returns true if the error is due to already exists
func IsAlreadyExists(err error) bool {
	return errors.Is(err, ErrAlreadyExists)
}

// IsNotFound returns true if the error is due to a missing object
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsSignalKilled returns true if the error is signal killed
func IsSignalKilled(err error) bool {
	return strings.Contains(err.Error(), signalKilled)
}

// IsConnectionClosed returns true if error is due to connection closed
// this is used when snapshotter closed by sig term
func IsConnectionClosed(err error) bool {
	switch err := err.(type) {
	case *net.OpError:
		return err.Err.Error() == "use of closed network connection"
	default:
		return false
	}
}

func IsErofsMounted(err error) bool {
	return stderrors.Is(err, syscall.EBUSY)
}
