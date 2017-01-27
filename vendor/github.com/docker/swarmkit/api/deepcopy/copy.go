package deepcopy

import (
	"fmt"
	"time"

	"github.com/gogo/protobuf/types"
)

// CopierFrom can be implemented if an object knows how to copy another into itself.
type CopierFrom interface {
	// Copy takes the fields from src and copies them into the target object.
	//
	// Calling this method with a nil receiver or a nil src may panic.
	CopyFrom(src interface{})
}

// Copy copies src into dst. dst and src must have the same type.
//
// If the type has a copy function defined, it will be used.
//
// Default implementations for builtin types and well known protobuf types may
// be provided.
//
// If the copy cannot be performed, this function will panic. Make sure to test
// types that use this function.
func Copy(dst, src interface{}) {
	switch dst := dst.(type) {
	case *types.Duration:
		src := src.(*types.Duration)
		*dst = *src
	case *time.Duration:
		src := src.(*time.Duration)
		*dst = *src
	case *types.Timestamp:
		src := src.(*types.Timestamp)
		*dst = *src
	case CopierFrom:
		dst.CopyFrom(src)
	default:
		panic(fmt.Sprintf("Copy for %T not implemented", dst))
	}

}
