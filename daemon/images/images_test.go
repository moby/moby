package images

import (
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
)

func TestOnlyPlatformWithFallback(t *testing.T) {
	p := ocispec.Platform{
		OS:           "linux",
		Architecture: "arm",
		Variant:      "v8",
	}

	// Check no variant
	assert.Assert(t, OnlyPlatformWithFallback(p).Match(ocispec.Platform{
		OS:           p.OS,
		Architecture: p.Architecture,
	}))
	// check with variant
	assert.Assert(t, OnlyPlatformWithFallback(p).Match(ocispec.Platform{
		OS:           p.OS,
		Architecture: p.Architecture,
		Variant:      p.Variant,
	}))
	// Make sure non-matches are false.
	assert.Assert(t, !OnlyPlatformWithFallback(p).Match(ocispec.Platform{
		OS:           p.OS,
		Architecture: "amd64",
	}))
}
