package errors

import (
	"errors"
	"fmt"
	"time"
)

var ErrInitNotAllowed = errors.New("tuf: repository already initialized")

type ErrMissingMetadata struct {
	Name string
}

func (e ErrMissingMetadata) Error() string {
	return fmt.Sprintf("tuf: missing metadata %s", e.Name)
}

type ErrFileNotFound struct {
	Path string
}

func (e ErrFileNotFound) Error() string {
	return fmt.Sprintf("tuf: file not found %s", e.Path)
}

type ErrInsufficientKeys struct {
	Name string
}

func (e ErrInsufficientKeys) Error() string {
	return fmt.Sprintf("tuf: insufficient keys to sign %s", e.Name)
}

type ErrInsufficientSignatures struct {
	Name string
	Err  error
}

func (e ErrInsufficientSignatures) Error() string {
	return fmt.Sprintf("tuf: insufficient signatures for %s: %s", e.Name, e.Err)
}

type ErrInvalidRole struct {
	Role string
}

func (e ErrInvalidRole) Error() string {
	return fmt.Sprintf("tuf: invalid role %s", e.Role)
}

type ErrInvalidExpires struct {
	Expires time.Time
}

func (e ErrInvalidExpires) Error() string {
	return fmt.Sprintf("tuf: invalid expires: %s", e.Expires)
}

type ErrKeyNotFound struct {
	Role  string
	KeyID string
}

func (e ErrKeyNotFound) Error() string {
	return fmt.Sprintf(`tuf: no key with id "%s" exists for the %s role`, e.KeyID, e.Role)
}

type ErrNotEnoughKeys struct {
	Role      string
	Keys      int
	Threshold int
}

func (e ErrNotEnoughKeys) Error() string {
	return fmt.Sprintf("tuf: %s role has insufficient keys for threshold (has %d keys, threshold is %d)", e.Role, e.Keys, e.Threshold)
}

type ErrPassphraseRequired struct {
	Role string
}

func (e ErrPassphraseRequired) Error() string {
	return fmt.Sprintf("tuf: a passphrase is required to access the encrypted %s keys file", e.Role)
}
