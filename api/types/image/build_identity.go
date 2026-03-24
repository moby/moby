package image

import (
	"time"
)

// BuildIdentity contains build reference information if image was created via build.
type BuildIdentity struct {
	// Ref is the identifier for the build request. This reference can be used to
	// look up the build details in BuildKit history API.
	Ref string `json:"Ref,omitempty"`

	// CreatedAt is the time when the build ran.
	CreatedAt time.Time `json:"CreatedAt,omitempty"`
}
