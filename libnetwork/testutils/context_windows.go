package testutils

import "testing"

type OSContext struct{}

func SetupTestOSContextEx(*testing.T) *OSContext {
	return nil
}

func (*OSContext) Cleanup(t *testing.T) {}

func (*OSContext) Set() (func(Logger), error) {
	return func(Logger) {}, nil
}
