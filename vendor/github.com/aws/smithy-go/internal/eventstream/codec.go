package eventstream

import (
	"bytes"
	"fmt"
	"io"

	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/eventstream"
)

// Codec orchestrates event stream message serde for protocols that use the
// standard event stream binary framing.
type Codec struct {
	Serializer   func() smithy.ShapeSerializer
	Deserializer func([]byte) smithy.ShapeDeserializer
	ContentType  string

	// Protocol-specific hook to retrieve error information from some payload.
	ErrorInfo func(payload []byte) (code, message string, err error)

	encoder    *eventstream.Encoder
	decoder    *eventstream.Decoder
	payloadBuf []byte
}

func (c *Codec) enc() *eventstream.Encoder {
	if c.encoder == nil {
		c.encoder = eventstream.NewEncoder()
	}
	return c.encoder
}

func (c *Codec) dec() *eventstream.Decoder {
	if c.decoder == nil {
		c.decoder = eventstream.NewDecoder()
	}
	return c.decoder
}

// SerializeEventMessage serializes an event to the input stream.
func (c *Codec) SerializeEventMessage(schema, variant *smithy.Schema, v smithy.Serializable, w io.Writer) error {
	var msg eventstream.Message

	inner := c.Serializer()
	ss := eventstream.NewShapeSerializer(&msg, inner)

	v.Serialize(ss)

	// header order matches the legacy per-operation serializers: the event type
	// is written before the message type
	msg.Headers.Set(eventstream.EventTypeHeader, eventstream.StringValue(variant.MemberName()))
	msg.Headers.Set(eventstream.MessageTypeHeader, eventstream.StringValue(eventstream.EventMessageType))

	if ct := ss.ContentType(); ct != "" {
		msg.Headers.Set(eventstream.ContentTypeHeader, eventstream.StringValue(ct))
	} else if payload := inner.Bytes(); len(payload) > 0 {
		msg.Payload = payload
		msg.Headers.Set(eventstream.ContentTypeHeader, eventstream.StringValue(c.ContentType))
	}

	return c.enc().Encode(w, msg)
}

// DeserializeEventMessage reads an event from the output stream.
func (c *Codec) DeserializeEventMessage(schema *smithy.Schema, types *smithy.TypeRegistry, r io.Reader) (smithy.Deserializable, error) {
	for {
		c.payloadBuf = c.payloadBuf[0:0]
		msg, err := c.dec().Decode(r, c.payloadBuf)
		if err != nil {
			if isEOF(err) {
				return nil, io.EOF
			}
			return nil, fmt.Errorf("decode event: %w", err)
		}

		msgType := msg.Headers.Get(eventstream.MessageTypeHeader)
		if msgType == nil {
			return nil, fmt.Errorf("missing %s header", eventstream.MessageTypeHeader)
		}

		switch msgType.String() {
		case eventstream.EventMessageType:
			event, err := c.deserializeEvent(schema, types, &msg)
			if err != nil {
				return nil, err
			}
			return event, nil
		case eventstream.ExceptionMessageType:
			return nil, c.deserializeException(schema, types, &msg)
		case eventstream.ErrorMessageType:
			return nil, deserializeError(&msg)
		default:
			mc := msg.Clone()
			return nil, &eventstream.UnknownMessageError{
				Type:    msgType.String(),
				Message: &mc,
			}
		}
	}
}

func (c *Codec) deserializeEvent(schema *smithy.Schema, types *smithy.TypeRegistry, msg *eventstream.Message) (smithy.Deserializable, error) {
	eventType := msg.Headers.Get(eventstream.EventTypeHeader)
	if eventType == nil {
		return nil, fmt.Errorf("missing %s header", eventstream.EventTypeHeader)
	}

	member := schema.Member(eventType.String())
	if member == nil {
		return c.unknownEvent(eventType.String(), msg)
	}

	entry, ok := types.LookupEntry(member.TargetID().String())
	if !ok {
		return c.unknownEvent(eventType.String(), msg)
	}

	instance, ok := entry.New().(smithy.Deserializable)
	if !ok {
		return nil, fmt.Errorf("event type %s is not deserializable", eventType.String())
	}

	inner := c.Deserializer(msg.Payload)
	ed := eventstream.NewShapeDeserializer(msg, inner)
	if err := instance.Deserialize(ed); err != nil {
		return nil, fmt.Errorf("deserialize event %s: %w", eventType.String(), err)
	}

	return instance, nil
}

func (c *Codec) unknownEvent(tag string, msg *eventstream.Message) (*eventstream.UnknownUnionMember, error) {
	var buf bytes.Buffer
	c.enc().Encode(&buf, *msg)
	return &eventstream.UnknownUnionMember{Tag: tag, Value: buf.Bytes()}, nil
}

func (c *Codec) deserializeException(schema *smithy.Schema, types *smithy.TypeRegistry, msg *eventstream.Message) error {
	exType := msg.Headers.Get(eventstream.ExceptionTypeHeader)
	if exType == nil {
		return fmt.Errorf("missing %s header", eventstream.ExceptionTypeHeader)
	}

	var id string
	if member := schema.Member(exType.String()); member != nil {
		id = member.TargetID().String()
	} else {
		id = exType.String()
	}

	perr, ok := types.DeserializableError(id)
	if !ok {
		code := exType.String()
		message := "UnknownError"
		if c.ErrorInfo != nil {
			bodyCode, bodyMessage, err := c.ErrorInfo(msg.Payload)
			if err != nil {
				return &smithy.DeserializationError{
					Err:      fmt.Errorf("get error info: %w", err),
					Snapshot: msg.Payload,
				}
			}
			if len(code) == 0 && len(bodyCode) != 0 {
				code = bodyCode
			}
			if len(bodyMessage) != 0 {
				message = bodyMessage
			}
		}
		return &smithy.GenericAPIError{
			Code:    code,
			Message: message,
		}
	}

	inner := c.Deserializer(msg.Payload)
	ed := eventstream.NewShapeDeserializer(msg, inner)
	if err := perr.Deserialize(ed); err != nil {
		return fmt.Errorf("deserialize exception %s: %w", exType.String(), err)
	}

	return perr
}

// SerializeInitialRequest serializes the operation input as the first event
// stream message with :event-type "initial-request".
func (c *Codec) SerializeInitialRequest(schema *smithy.Schema, v smithy.Serializable, w io.Writer) error {
	ss := c.Serializer()
	v.Serialize(ss)

	var msg eventstream.Message
	msg.Headers.Set(eventstream.MessageTypeHeader, eventstream.StringValue(eventstream.EventMessageType))
	msg.Headers.Set(eventstream.EventTypeHeader, eventstream.StringValue("initial-request"))
	if payload := ss.Bytes(); len(payload) > 0 {
		msg.Payload = payload
		msg.Headers.Set(eventstream.ContentTypeHeader, eventstream.StringValue(c.ContentType))
	}

	return c.enc().Encode(w, msg)
}

// DeserializeInitialResponse reads the first event stream message and
// deserializes it as the operation output.
func (c *Codec) DeserializeInitialResponse(schema *smithy.Schema, r io.Reader, out smithy.Deserializable) error {
	c.payloadBuf = c.payloadBuf[0:0]
	msg, err := c.dec().Decode(r, c.payloadBuf)
	if err != nil {
		return fmt.Errorf("decode initial response: %w", err)
	}

	eventType := msg.Headers.Get(eventstream.EventTypeHeader)
	if eventType == nil || eventType.String() != "initial-response" {
		return fmt.Errorf("expected initial-response, got %v", eventType)
	}

	if len(msg.Payload) > 0 {
		sd := c.Deserializer(msg.Payload)
		if err := out.Deserialize(sd); err != nil {
			return fmt.Errorf("deserialize initial response: %w", err)
		}
	}

	return nil
}

func deserializeError(msg *eventstream.Message) error {
	code := "UnknownError"
	message := code
	if v := msg.Headers.Get(eventstream.ErrorCodeHeader); v != nil {
		code = v.String()
	}
	if v := msg.Headers.Get(eventstream.ErrorMessageHeader); v != nil {
		message = v.String()
	}
	return &smithy.GenericAPIError{
		Code:    code,
		Message: message,
	}
}

func isEOF(err error) bool {
	return err == io.EOF || err == io.ErrUnexpectedEOF
}
