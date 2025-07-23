package specialimage

import (
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type SpecialImageFunc func(string) (*ocispec.Index, error)
