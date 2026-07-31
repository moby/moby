package ptypes

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// MustTimestampProto converts time.Time to a google.protobuf.Timestamp proto.
// It panics if input timestamp is invalid.
func MustTimestampProto(t time.Time) *timestamppb.Timestamp {
	ts := timestamppb.New(t)
	if err := ts.CheckValid(); err != nil {
		panic(err.Error())
	}
	return ts
}
