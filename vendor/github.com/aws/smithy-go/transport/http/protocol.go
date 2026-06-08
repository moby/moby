package http

import (
	"context"
	"io"

	"github.com/aws/smithy-go"
)

// ClientProtocol defines the interface through which client-side operation
// request/responses are (de)serialized across the wire.
//
// While a caller CAN define their own protocol, it is almost never necessary
// to do so. In practice, a generated client will utilize one of the predefined
// protocols implemented as part of the Smithy client runtime.
type ClientProtocol interface {
	ID() smithy.ShapeID
	SerializeRequest(context.Context, *smithy.OperationSchema, smithy.Serializable, *Request) error
	DeserializeResponse(ctx context.Context, schema *smithy.OperationSchema, types *smithy.TypeRegistry, resp *Response, out smithy.Deserializable) error

	// event stream APIs
	HasInitialEventMessage() bool
	SerializeEventMessage(schema, variant *smithy.Schema, v smithy.Serializable, w io.Writer) error
	DeserializeEventMessage(schema *smithy.Schema, types *smithy.TypeRegistry, r io.Reader) (smithy.Deserializable, error)
	SerializeInitialRequest(schema *smithy.Schema, v smithy.Serializable, w io.Writer) error
	DeserializeInitialResponse(schema *smithy.Schema, r io.Reader, out smithy.Deserializable) error
}
