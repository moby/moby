package errdefs

import (
	fmt "fmt"

	"github.com/containerd/typeurl/v2"
	"github.com/moby/buildkit/util/grpcerrors"
)

func init() {
	typeurl.Register((*CompatibilityFeature)(nil), "github.com/moby/buildkit", "errdefs.CompatibilityFeature+json")
}

type UnsupportedCompatibilityFeatureError struct {
	*CompatibilityFeature
	error
}

func (e *UnsupportedCompatibilityFeatureError) Error() string {
	msg := fmt.Sprintf("unsupported compatibility-version %d feature %s", e.Version, e.Feature)
	if e.error != nil {
		msg += ": " + e.error.Error()
	}
	return msg
}

func (e *UnsupportedCompatibilityFeatureError) Unwrap() error {
	return e.error
}

func (e *UnsupportedCompatibilityFeatureError) ToProto() grpcerrors.TypedErrorProto {
	return e.CompatibilityFeature
}

func NewUnsupportedCompatibilityFeatureError(version int, feature string) error {
	return &UnsupportedCompatibilityFeatureError{
		CompatibilityFeature: &CompatibilityFeature{Version: int64(version), Feature: feature},
	}
}

func (v *CompatibilityFeature) WrapError(err error) error {
	return &UnsupportedCompatibilityFeatureError{error: err, CompatibilityFeature: v}
}
