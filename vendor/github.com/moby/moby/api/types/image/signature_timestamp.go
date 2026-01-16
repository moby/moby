package image

import (
	"time"
)

// SignatureTimestamp contains information about a verified signed timestamp for an image signature.
type SignatureTimestamp struct {
	Type      SignatureTimestampType `json:"Type"`
	URI       string                 `json:"URI"`
	Timestamp time.Time              `json:"Timestamp"`
}
