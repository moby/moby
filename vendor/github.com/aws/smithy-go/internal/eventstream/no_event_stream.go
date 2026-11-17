package eventstream

import (
	"fmt"
	"io"

	"github.com/aws/smithy-go"
)

// NoEventStream implements the event stream methods of
// [github.com/aws/smithy-go/transport/http.ClientProtocol] for protocols
// that do not support event streams.
type NoEventStream struct{}

// HasInitialEventMessage returns false.
func (NoEventStream) HasInitialEventMessage() bool { return false }

// SerializeEventMessage returns an error.
func (NoEventStream) SerializeEventMessage(_, _ *smithy.Schema, _ smithy.Serializable, _ io.Writer) error {
	return fmt.Errorf("event streams are not supported by this protocol")
}

// DeserializeEventMessage returns an error.
func (NoEventStream) DeserializeEventMessage(_ *smithy.Schema, _ *smithy.TypeRegistry, _ io.Reader) (smithy.Deserializable, error) {
	return nil, fmt.Errorf("event streams are not supported by this protocol")
}

// SerializeInitialRequest returns an error.
func (NoEventStream) SerializeInitialRequest(_ *smithy.Schema, _ smithy.Serializable, _ io.Writer) error {
	return fmt.Errorf("event streams are not supported by this protocol")
}

// DeserializeInitialResponse returns an error.
func (NoEventStream) DeserializeInitialResponse(_ *smithy.Schema, _ io.Reader, _ smithy.Deserializable) error {
	return fmt.Errorf("event streams are not supported by this protocol")
}
