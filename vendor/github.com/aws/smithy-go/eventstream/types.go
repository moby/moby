package eventstream

import "github.com/aws/smithy-go"

// UnknownUnionMember is returned when a union member is returned over the
// wire, but has an unknown tag.
type UnknownUnionMember struct {
	Tag   string
	Value []byte
}

// Deserialize is a no-op. The raw bytes are already captured in Value.
func (*UnknownUnionMember) Deserialize(smithy.ShapeDeserializer) error {
	return nil
}

// UnknownMessageError provides an error when a message is received from the
// stream, but the reader is unable to determine what kind of message it is.
type UnknownMessageError struct {
	Type    string
	Message *Message
}

func (e *UnknownMessageError) Error() string {
	return "unknown event stream message type, " + e.Type
}
