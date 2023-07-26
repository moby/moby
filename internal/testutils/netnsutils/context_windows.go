package netnsutils

import (
	"testing"

	"github.com/docker/docker/internal/testutils"
)

type OSContext struct{}

func SetupTestOSContextEx(*testing.T) *OSContext {
	return nil
}

func (*OSContext) Cleanup(t *testing.T) {}

func (*OSContext) Set() (func(testutils.Logger), error) {
	return func(testutils.Logger) {}, nil
}
