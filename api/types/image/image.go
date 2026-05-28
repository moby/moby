package image

import (
	"time"
)

// Metadata contains engine-local data about the image.
type Metadata struct {
	// LastTagTime is the date and time at which the image was last tagged.
	LastTagTime time.Time `json:",omitempty"`
}

// PruneReport contains the response for Engine API:
// POST "/images/prune"
type PruneReport struct {
	ImagesDeleted  []DeleteResponse
	SpaceReclaimed uint64
}
