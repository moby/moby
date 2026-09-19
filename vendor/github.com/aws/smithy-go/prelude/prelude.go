// Package prelude defines schemas for the Smithy prelude shapes.
package prelude

import "github.com/aws/smithy-go"

// All shapes in the prelude.
var (
	String    = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "String"}, smithy.ShapeTypeString, 0)
	Blob      = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "Blob"}, smithy.ShapeTypeBlob, 0)
	Boolean   = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "Boolean"}, smithy.ShapeTypeBoolean, 0)
	Byte      = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "Byte"}, smithy.ShapeTypeByte, 0)
	Short     = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "Short"}, smithy.ShapeTypeShort, 0)
	Integer   = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "Integer"}, smithy.ShapeTypeInteger, 0)
	Long      = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "Long"}, smithy.ShapeTypeLong, 0)
	Float     = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "Float"}, smithy.ShapeTypeFloat, 0)
	Double    = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "Double"}, smithy.ShapeTypeDouble, 0)
	Timestamp = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "Timestamp"}, smithy.ShapeTypeTimestamp, 0)
	Document  = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "Document"}, smithy.ShapeTypeDocument, 0)
	Unit      = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "Unit"}, smithy.ShapeTypeStructure, 0)

	PrimitiveBoolean = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "PrimitiveBoolean"}, smithy.ShapeTypeBoolean, 0)
	PrimitiveByte    = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "PrimitiveByte"}, smithy.ShapeTypeByte, 0)
	PrimitiveShort   = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "PrimitiveShort"}, smithy.ShapeTypeShort, 0)
	PrimitiveInteger = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "PrimitiveInteger"}, smithy.ShapeTypeInteger, 0)
	PrimitiveLong    = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "PrimitiveLong"}, smithy.ShapeTypeLong, 0)
	PrimitiveFloat   = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "PrimitiveFloat"}, smithy.ShapeTypeFloat, 0)
	PrimitiveDouble  = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "PrimitiveDouble"}, smithy.ShapeTypeDouble, 0)

	BigInteger = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "BigInteger"}, smithy.ShapeTypeBigInteger, 0)
	BigDecimal = smithy.NewSchema(smithy.ShapeID{Namespace: "smithy.api", Name: "BigDecimal"}, smithy.ShapeTypeBigDecimal, 0)
)
