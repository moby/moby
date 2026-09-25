package smithy

import (
	"fmt"
	"io"
	"math/big"
	"time"

	"github.com/aws/smithy-go/document"
)

// ShapeSerializer implements the marshaling of an in-code representation of a
// shape to an unspecified data format, which is determined by the
// implementation.
//
// A ShapeSerializer is consumed by the **code-generated** Serialize() method
// of a modeled structure. For example:
//
//	func (v *PutItemInput) Serialize(s smithy.ShapeSerializer) {
//		s.WriteStruct(schemas.PutItemInput)
//		v.SerializeMembers(s)
//		s.CloseStruct()
//	}
//
//	func (v *PutItemInput) SerializeMembers(s smithy.ShapeSerializer) {
//		if v.TableName != nil {
//			s.WriteString(schemas.PutItemInput_TableName, *v.TableName)
//		}
//		if v.Item != nil {
//			serializeAttributeMap(s, schemas.PutItemInput_Item, v.Item)
//		}
//		// ...
//	}
type ShapeSerializer interface {
	Bytes() []byte

	WriteInt8(*Schema, int8)
	WriteInt16(*Schema, int16)
	WriteInt32(*Schema, int32)
	WriteInt64(*Schema, int64)
	WriteFloat32(*Schema, float32)
	WriteFloat64(*Schema, float64)
	WriteBool(*Schema, bool)
	WriteString(*Schema, string)
	WriteBigInt(*Schema, *big.Int)
	WriteBigFloat(*Schema, *big.Float)
	WriteBlob(*Schema, []byte)
	WriteTime(*Schema, time.Time)

	WriteUnion(schema, variant *Schema)
	CloseUnion()
	WriteDocument(*Schema, document.Value)
	WriteNil(*Schema)

	WriteStruct(*Schema)
	CloseStruct()

	WriteList(*Schema)
	CloseList()

	WriteMap(*Schema)
	WriteKey(*Schema, string)
	CloseMap()
}

// ShapeDeserializer implements the unmarshaling from some unspecified data
// format to an in-code representation of a shape, which is determined by the
// implementation.
type ShapeDeserializer interface {
	ReadInt8(*Schema, *int8) error
	ReadInt16(*Schema, *int16) error
	ReadInt32(*Schema, *int32) error
	ReadInt64(*Schema, *int64) error
	ReadFloat32(*Schema, *float32) error
	ReadFloat64(*Schema, *float64) error
	ReadBool(*Schema, *bool) error
	ReadString(*Schema, *string) error
	ReadBlob(*Schema, *[]byte) error
	ReadTime(*Schema, *time.Time) error
	ReadBigInt(*Schema, *big.Int) error
	ReadBigFloat(*Schema, *big.Float) error
	ReadNil(*Schema) (bool, error)

	ReadStruct(*Schema) error
	ReadStructMember() (*Schema, error)

	ReadUnion(*Schema) (*Schema, error)
	ReadDocument(*Schema, *document.Value) error

	ReadList(*Schema) error
	ReadListItem(*Schema) (hasMoreElements bool, err error)

	ReadMap(*Schema) error
	ReadMapKey(*Schema) (key string, hasMoreElements bool, err error)
}

// Serializable is an entity that can describe itself to a ShapeSerializer to
// be encoded to some format.
//
// Unlike the standard library marshaler interfaces, which idiomatically encode
// to []byte, the output format and data type here is not specified at all.
// This is because Smithy shapes need to encode to a variety of formats or data
// carriers. For example, HTTP-binding JSON protocols need to serialize some
// members to bytes (the HTTP request body) and others directly to fields on
// the HTTP request itself (e.g. headers).
type Serializable interface {
	Serialize(ShapeSerializer)
}

// StreamingInput is implemented by input types that have a streaming blob
// payload (an io.Reader member with @httpPayload + @streaming).
type StreamingInput interface {
	GetPayloadStream() io.Reader
}

// StreamingOutput is implemented by output types that have a streaming blob
// payload (an io.ReadCloser member with @httpPayload + @streaming).
type StreamingOutput interface {
	SetPayloadStream(io.ReadCloser)
}

// Deserializable is an entity that can unmarshal itself from a
// ShapeDeserializer.
type Deserializable interface {
	Deserialize(ShapeDeserializer) error
}

// DeserializableError is implemented by modeled error types for a service.
type DeserializableError interface {
	Deserializable
	error
}

// ReadUnion is a utility API for generated clients.
func ReadUnion(d ShapeDeserializer, schema *Schema, memberFn func(*Schema) error) error {
	ms, err := d.ReadUnion(schema)
	if ms == nil || err != nil {
		return err
	}

	if err := memberFn(ms); err != nil {
		return err
	}

	for {
		ms, err = d.ReadUnion(schema)
		if err != nil {
			return err
		}
		if ms == nil {
			return nil
		}
		return fmt.Errorf("union has more than one non-nil member: %s", ms.MemberName())
	}
}

// ReadStruct is a utility API for generated clients.
func ReadStruct(d ShapeDeserializer, schema *Schema, memberFn func(*Schema) error) error {
	if err := d.ReadStruct(schema); err != nil {
		return err
	}

	for {
		ms, err := d.ReadStructMember()
		if err != nil {
			return err
		}

		if ms == nil {
			return nil
		}

		if err := memberFn(ms); err != nil {
			return err
		}
	}
}

// ReadList is a utility API for generated clients.
func ReadList(d ShapeDeserializer, schema *Schema, memberFn func() error) error {
	if err := d.ReadList(schema); err != nil {
		return err
	}

	var memberSchema *Schema
	if schema != nil {
		memberSchema = schema.ListMember()
	}

	for {
		ok, err := d.ReadListItem(memberSchema)
		if !ok {
			return nil
		}
		if err != nil {
			return err
		}

		if err := memberFn(); err != nil {
			return err
		}
	}
}

// ReadMap is a utility API for generated clients.
func ReadMap(d ShapeDeserializer, schema *Schema, memberFn func(string) error) error {
	if err := d.ReadMap(schema); err != nil {
		return err
	}

	var keySchema *Schema
	if schema != nil {
		keySchema = schema.MapKey()
	}

	for {
		k, ok, err := d.ReadMapKey(keySchema)
		if !ok {
			return nil
		}
		if err != nil {
			return err
		}

		if err := memberFn(k); err != nil {
			return err
		}
	}
}
