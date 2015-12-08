package signed

import (
	"fmt"
	"strings"
)

// ErrInsufficientSignatures - do not have enough signatures on a piece of
// metadata
type ErrInsufficientSignatures struct {
	Name string
}

func (e ErrInsufficientSignatures) Error() string {
	return fmt.Sprintf("tuf: insufficient signatures: %s", e.Name)
}

// ErrExpired indicates a piece of metadata has expired
type ErrExpired struct {
	Role    string
	Expired string
}

func (e ErrExpired) Error() string {
	return fmt.Sprintf("%s expired at %v", e.Role, e.Expired)
}

// ErrLowVersion indicates the piece of metadata has a version number lower than
// a version number we're already seen for this role
type ErrLowVersion struct {
	Actual  int
	Current int
}

func (e ErrLowVersion) Error() string {
	return fmt.Sprintf("version %d is lower than current version %d", e.Actual, e.Current)
}

// ErrRoleThreshold indicates we did not validate enough signatures to meet the threshold
type ErrRoleThreshold struct{}

func (e ErrRoleThreshold) Error() string {
	return "valid signatures did not meet threshold"
}

// ErrInvalidKeyType indicates the types for the key and signature it's associated with are
// mismatched. Probably a sign of malicious behaviour
type ErrInvalidKeyType struct{}

func (e ErrInvalidKeyType) Error() string {
	return "key type is not valid for signature"
}

// ErrInvalidKeyLength indicates that while we may support the cipher, the provided
// key length is not specifically supported, i.e. we support RSA, but not 1024 bit keys
type ErrInvalidKeyLength struct {
	msg string
}

func (e ErrInvalidKeyLength) Error() string {
	return fmt.Sprintf("key length is not supported: %s", e.msg)
}

// ErrNoKeys indicates no signing keys were found when trying to sign
type ErrNoKeys struct {
	keyIDs []string
}

func (e ErrNoKeys) Error() string {
	return fmt.Sprintf("could not find necessary signing keys, at least one of these keys must be available: %s",
		strings.Join(e.keyIDs, ", "))
}
