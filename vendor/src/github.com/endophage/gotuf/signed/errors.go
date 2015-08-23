package signed

import (
	"fmt"
)

type ErrExpired struct {
	Role    string
	Expired string
}

func (e ErrExpired) Error() string {
	return fmt.Sprintf("%s expired at %v", e.Role, e.Expired)
}

type ErrLowVersion struct {
	Actual  int
	Current int
}

func (e ErrLowVersion) Error() string {
	return fmt.Sprintf("version %d is lower than current version %d", e.Actual, e.Current)
}

type ErrRoleThreshold struct{}

func (e ErrRoleThreshold) Error() string {
	return "valid signatures did not meet threshold"
}

type ErrInvalidKeyType struct{}

func (e ErrInvalidKeyType) Error() string {
	return "key type is not valid for signature"
}

type ErrInvalidKeyLength struct {
	msg string
}

func (e ErrInvalidKeyLength) Error() string {
	return fmt.Sprintf("key length is not supported: %s", e.msg)
}
