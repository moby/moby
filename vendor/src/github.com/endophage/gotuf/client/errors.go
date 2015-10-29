package client

import (
	"errors"
	"fmt"
)

var (
	ErrNoRootKeys       = errors.New("tuf: no root keys found in local meta store")
	ErrInsufficientKeys = errors.New("tuf: insufficient keys to meet threshold")
)

type ErrChecksumMismatch struct {
	role string
}

func (e ErrChecksumMismatch) Error() string {
	return fmt.Sprintf("tuf: checksum for %s did not match", e.role)
}

type ErrMissingMeta struct {
	role string
}

func (e ErrMissingMeta) Error() string {
	return fmt.Sprintf("tuf: sha256 checksum required for %s", e.role)
}

type ErrMissingRemoteMetadata struct {
	Name string
}

func (e ErrMissingRemoteMetadata) Error() string {
	return fmt.Sprintf("tuf: missing remote metadata %s", e.Name)
}

type ErrDownloadFailed struct {
	File string
	Err  error
}

func (e ErrDownloadFailed) Error() string {
	return fmt.Sprintf("tuf: failed to download %s: %s", e.File, e.Err)
}

type ErrDecodeFailed struct {
	File string
	Err  error
}

func (e ErrDecodeFailed) Error() string {
	return fmt.Sprintf("tuf: failed to decode %s: %s", e.File, e.Err)
}

func isDecodeFailedWithErr(err, expected error) bool {
	e, ok := err.(ErrDecodeFailed)
	if !ok {
		return false
	}
	return e.Err == expected
}

type ErrNotFound struct {
	File string
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("tuf: file not found: %s", e.File)
}

func IsNotFound(err error) bool {
	_, ok := err.(ErrNotFound)
	return ok
}

type ErrWrongSize struct {
	File     string
	Actual   int64
	Expected int64
}

func (e ErrWrongSize) Error() string {
	return fmt.Sprintf("tuf: unexpected file size: %s (expected %d bytes, got %d bytes)", e.File, e.Expected, e.Actual)
}

type ErrLatestSnapshot struct {
	Version int
}

func (e ErrLatestSnapshot) Error() string {
	return fmt.Sprintf("tuf: the local snapshot version (%d) is the latest", e.Version)
}

func IsLatestSnapshot(err error) bool {
	_, ok := err.(ErrLatestSnapshot)
	return ok
}

type ErrUnknownTarget struct {
	Name string
}

func (e ErrUnknownTarget) Error() string {
	return fmt.Sprintf("tuf: unknown target file: %s", e.Name)
}

type ErrMetaTooLarge struct {
	Name string
	Size int64
}

func (e ErrMetaTooLarge) Error() string {
	return fmt.Sprintf("tuf: %s size %d bytes greater than maximum", e.Name, e.Size)
}

type ErrInvalidURL struct {
	URL string
}

func (e ErrInvalidURL) Error() string {
	return fmt.Sprintf("tuf: invalid repository URL %s", e.URL)
}

type ErrCorruptedCache struct {
	file string
}

func (e ErrCorruptedCache) Error() string {
	return fmt.Sprintf("cache is corrupted: %s", e.file)
}
