// Copyright 2024 The Update Framework Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License
//
// SPDX-License-Identifier: Apache-2.0
//

package metadata

import (
	"fmt"
)

// Define TUF error types used inside the new modern implementation.
// The names chosen for TUF error types should start in 'Err' except where
// there is a good reason not to, and provide that reason in those cases.

// Repository errors

// ErrRepository - an error with a repository's state, such as a missing file.
// It covers all exceptions that come from the repository side when
// looking from the perspective of users of metadata API or client
type ErrRepository struct {
	Msg string
}

func (e *ErrRepository) Error() string {
	return fmt.Sprintf("repository error: %s", e.Msg)
}

func (e *ErrRepository) Is(target error) bool {
	_, ok := target.(*ErrRepository)
	return ok
}

// ErrUnsignedMetadata - An error about metadata object with insufficient threshold of signatures
type ErrUnsignedMetadata struct {
	Msg string
}

func (e *ErrUnsignedMetadata) Error() string {
	return fmt.Sprintf("unsigned metadata error: %s", e.Msg)
}

// ErrUnsignedMetadata is a subset of ErrRepository
func (e *ErrUnsignedMetadata) Is(target error) bool {
	if _, ok := target.(*ErrUnsignedMetadata); ok {
		return true
	}
	if _, ok := target.(*ErrRepository); ok {
		return true
	}
	return false
}

// ErrBadVersionNumber - An error for metadata that contains an invalid version number
type ErrBadVersionNumber struct {
	Msg string
}

func (e *ErrBadVersionNumber) Error() string {
	return fmt.Sprintf("bad version number error: %s", e.Msg)
}

// ErrBadVersionNumber is a subset of ErrRepository
func (e *ErrBadVersionNumber) Is(target error) bool {
	if _, ok := target.(*ErrBadVersionNumber); ok {
		return true
	}
	if _, ok := target.(*ErrRepository); ok {
		return true
	}
	return false
}

// ErrEqualVersionNumber - An error for metadata containing a previously verified version number
type ErrEqualVersionNumber struct {
	Msg string
}

func (e *ErrEqualVersionNumber) Error() string {
	return fmt.Sprintf("equal version number error: %s", e.Msg)
}

// ErrEqualVersionNumber is a subset of both ErrRepository and ErrBadVersionNumber
func (e *ErrEqualVersionNumber) Is(target error) bool {
	if _, ok := target.(*ErrEqualVersionNumber); ok {
		return true
	}
	if _, ok := target.(*ErrBadVersionNumber); ok {
		return true
	}
	if _, ok := target.(*ErrRepository); ok {
		return true
	}
	return false
}

// ErrExpiredMetadata - Indicate that a TUF Metadata file has expired
type ErrExpiredMetadata struct {
	Msg string
}

func (e *ErrExpiredMetadata) Error() string {
	return fmt.Sprintf("expired metadata error: %s", e.Msg)
}

// ErrExpiredMetadata is a subset of ErrRepository
func (e *ErrExpiredMetadata) Is(target error) bool {
	if _, ok := target.(*ErrExpiredMetadata); ok {
		return true
	}
	if _, ok := target.(*ErrRepository); ok {
		return true
	}
	return false
}

// ErrLengthOrHashMismatch - An error while checking the length and hash values of an object
type ErrLengthOrHashMismatch struct {
	Msg string
}

func (e *ErrLengthOrHashMismatch) Error() string {
	return fmt.Sprintf("length/hash verification error: %s", e.Msg)
}

// ErrLengthOrHashMismatch is a subset of ErrRepository
func (e *ErrLengthOrHashMismatch) Is(target error) bool {
	if _, ok := target.(*ErrLengthOrHashMismatch); ok {
		return true
	}
	if _, ok := target.(*ErrRepository); ok {
		return true
	}
	return false
}

// Download errors

// ErrDownload - An error occurred while attempting to download a file
type ErrDownload struct {
	Msg string
}

func (e *ErrDownload) Error() string {
	return fmt.Sprintf("download error: %s", e.Msg)
}

func (e *ErrDownload) Is(target error) bool {
	_, ok := target.(*ErrDownload)
	return ok
}

// ErrDownloadLengthMismatch - Indicate that a mismatch of lengths was seen while downloading a file
type ErrDownloadLengthMismatch struct {
	Msg string
}

func (e *ErrDownloadLengthMismatch) Error() string {
	return fmt.Sprintf("download length mismatch error: %s", e.Msg)
}

// ErrDownloadLengthMismatch is a subset of ErrDownload
func (e *ErrDownloadLengthMismatch) Is(target error) bool {
	if _, ok := target.(*ErrDownloadLengthMismatch); ok {
		return true
	}
	if _, ok := target.(*ErrDownload); ok {
		return true
	}
	return false
}

// ErrDownloadHTTP - Returned by Fetcher interface implementations for HTTP errors
type ErrDownloadHTTP struct {
	StatusCode int
	URL        string
}

func (e *ErrDownloadHTTP) Error() string {
	return fmt.Sprintf("failed to download %s, http status code: %d", e.URL, e.StatusCode)
}

// ErrDownloadHTTP is a subset of ErrDownload
func (e *ErrDownloadHTTP) Is(target error) bool {
	if _, ok := target.(*ErrDownloadHTTP); ok {
		return true
	}
	if _, ok := target.(*ErrDownload); ok {
		return true
	}
	return false
}

// ValueError
type ErrValue struct {
	Msg string
}

func (e *ErrValue) Error() string {
	return fmt.Sprintf("value error: %s", e.Msg)
}

func (e *ErrValue) Is(err error) bool {
	_, ok := err.(*ErrValue)
	return ok
}

// TypeError
type ErrType struct {
	Msg string
}

func (e *ErrType) Error() string {
	return fmt.Sprintf("type error: %s", e.Msg)
}

func (e *ErrType) Is(err error) bool {
	_, ok := err.(*ErrType)
	return ok
}

// RuntimeError
type ErrRuntime struct {
	Msg string
}

func (e *ErrRuntime) Error() string {
	return fmt.Sprintf("runtime error: %s", e.Msg)
}

func (e *ErrRuntime) Is(err error) bool {
	_, ok := err.(*ErrRuntime)
	return ok
}
