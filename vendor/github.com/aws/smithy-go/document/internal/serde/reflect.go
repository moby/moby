package serde

import (
	"github.com/aws/smithy-go/document"
	"math/big"
	"reflect"
	"time"
)

// ReflectTypeOf is a structure containing various reflect.Type members that are useful
// to document Marshaler or Unmarshaler implementations.
var ReflectTypeOf = struct {
	BigFloat             reflect.Type
	BigInt               reflect.Type
	DocumentNumber       reflect.Type
	MapStringToInterface reflect.Type
	Time                 reflect.Type
}{
	BigFloat:             reflect.TypeOf((*big.Float)(nil)).Elem(),
	BigInt:               reflect.TypeOf((*big.Int)(nil)).Elem(),
	DocumentNumber:       reflect.TypeOf((*document.Number)(nil)).Elem(),
	MapStringToInterface: reflect.TypeOf((map[string]interface{})(nil)),
	Time:                 reflect.TypeOf((*time.Time)(nil)).Elem(),
}
