package signed

import (
	"fmt"
	"strings"
)

// ErrInsufficientSignatures - can not create enough signatures on a piece of
// metadata
type ErrInsufficientSignatures struct {
	FoundKeys     int
	NeededKeys    int
	MissingKeyIDs []string
}

func (e ErrInsufficientSignatures) Error() string {
	candidates := ""
	if len(e.MissingKeyIDs) > 0 {
		candidates = fmt.Sprintf(" (%s)", strings.Join(e.MissingKeyIDs, ", "))
	}

	if e.FoundKeys == 0 {
		return fmt.Sprintf("signing keys not available: need %d keys from %d possible keys%s",
			e.NeededKeys, len(e.MissingKeyIDs), candidates)
	}
	return fmt.Sprintf("not enough signing keys: found %d of %d needed keys - %d other possible keys%s",
		e.FoundKeys, e.NeededKeys, len(e.MissingKeyIDs), candidates)
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
type ErrRoleThreshold struct {
	Msg string
}

func (e ErrRoleThreshold) Error() string {
	if e.Msg == "" {
		return "valid signatures did not meet threshold"
	}
	return e.Msg
}

// ErrInvalidKeyType indicates the types for the key and signature it's associated with are
// mismatched. Probably a sign of malicious behaviour
type ErrInvalidKeyType struct{}

func (e ErrInvalidKeyType) Error() string {
	return "key type is not valid for signature"
}

// ErrInvalidKeyID indicates the specified key ID was incorrect for its associated data
type ErrInvalidKeyID struct{}

func (e ErrInvalidKeyID) Error() string {
	return "key ID is not valid for key content"
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
	KeyIDs []string
}

func (e ErrNoKeys) Error() string {
	return fmt.Sprintf("could not find necessary signing keys, at least one of these keys must be available: %s",
		strings.Join(e.KeyIDs, ", "))
}
