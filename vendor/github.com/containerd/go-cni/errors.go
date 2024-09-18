/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package cni

import (
	"errors"
)

var (
	ErrCNINotInitialized = errors.New("cni plugin not initialized")
	ErrInvalidConfig     = errors.New("invalid cni config")
	ErrNotFound          = errors.New("not found")
	ErrRead              = errors.New("failed to read config file")
	ErrInvalidResult     = errors.New("invalid result")
	ErrLoad              = errors.New("failed to load cni config")
)

// IsCNINotInitialized returns true if the error is due to cni config not being initialized
func IsCNINotInitialized(err error) bool {
	return errors.Is(err, ErrCNINotInitialized)
}

// IsInvalidConfig returns true if the error is invalid cni config
func IsInvalidConfig(err error) bool {
	return errors.Is(err, ErrInvalidConfig)
}

// IsNotFound returns true if the error is due to a missing config or result
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsReadFailure return true if the error is a config read failure
func IsReadFailure(err error) bool {
	return errors.Is(err, ErrRead)
}

// IsInvalidResult return true if the error is due to invalid cni result
func IsInvalidResult(err error) bool {
	return errors.Is(err, ErrInvalidResult)
}
