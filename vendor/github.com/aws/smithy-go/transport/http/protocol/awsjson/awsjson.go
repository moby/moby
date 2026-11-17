package awsjson

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/smithy-go"
	internalerrors "github.com/aws/smithy-go/internal/errors"
	internales "github.com/aws/smithy-go/internal/eventstream"
	internalsync "github.com/aws/smithy-go/internal/sync"
	smithyio "github.com/aws/smithy-go/io"
	"github.com/aws/smithy-go/traits"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	internaljson "github.com/aws/smithy-go/transport/http/protocol/internal/json"
)

// ProtocolOptions configures aws.protocols#awsJson1_0.
type ProtocolOptions struct{}

// New10 returns an instance of the awsJson 1.0 protocol.
func New10(service *smithy.ServiceSchema, opts ...func(*ProtocolOptions)) *Protocol {
	return new("1.0", service, opts...)
}

// New11 returns an instance of the awsJson 1.1 protocol.
func New11(service *smithy.ServiceSchema, opts ...func(*ProtocolOptions)) *Protocol {
	return new("1.1", service, opts...)
}

func new(version string, service *smithy.ServiceSchema, opts ...func(*ProtocolOptions)) *Protocol {
	var o ProtocolOptions
	for _, fn := range opts {
		fn(&o)
	}

	_, qc := smithy.SchemaTrait[*traits.AWSQueryCompatible](service.Schema)
	return &Protocol{
		version:         version,
		queryCompatible: qc,
		serviceName:     service.Schema.ID().Name,
		bufs:            internalsync.NewBufferPool(),
		eventstream: &internales.Codec{
			Serializer:   func() smithy.ShapeSerializer { return internaljson.NewShapeSerializer() },
			Deserializer: func(p []byte) smithy.ShapeDeserializer { return internaljson.NewShapeDeserializer(p) },
			ContentType:  "application/json",
			ErrorInfo:    internaljson.EventStreamErrorInfo,
		},
	}
}

// Protocol implements aws.protocols#awsJson1_0 and aws.protocols#awsJson1_1.
type Protocol struct {
	version         string
	queryCompatible bool
	serviceName     string

	bufs        *internalsync.BufferPool
	eventstream *internales.Codec
}

var _ smithyhttp.ClientProtocol = (*Protocol)(nil)

// ID identifies the protocol.
func (p *Protocol) ID() smithy.ShapeID {
	if p.version == "1.1" {
		return smithy.ShapeID{Namespace: "aws.protocols", Name: "awsJson1_1"}
	}
	return smithy.ShapeID{Namespace: "aws.protocols", Name: "awsJson1_0"}
}

// SerializeRequest serializes a request for AWS Json 1.0.
func (p *Protocol) SerializeRequest(
	ctx context.Context,
	schema *smithy.OperationSchema,
	in smithy.Serializable,
	req *smithyhttp.Request,
) error {
	req.Method = http.MethodPost
	if len(req.URL.Path) == 0 {
		req.URL.Path = "/"
	}
	req.Header.Set("X-Amz-Target", fmt.Sprintf("%s.%s", p.serviceName, schema.ID().Name))
	if schema.IsInputEventStream() {
		req.Header.Set("Content-Type", "application/vnd.amazon.eventstream")
		return nil
	}

	req.Header.Set("Content-Type", "application/x-amz-json-"+p.version)
	if p.queryCompatible {
		req.Header.Set("X-Amzn-Query-Mode", "true")
	}

	if schema.Input == nil {
		sreq, err := req.SetStream(strings.NewReader("{}"))
		if err != nil {
			return fmt.Errorf("set stream: %w", err)
		}
		*req = *sreq
		return nil
	}

	ss := internaljson.NewShapeSerializer()
	in.Serialize(ss)

	sreq, err := req.SetStream(bytes.NewReader(ss.Bytes()))
	if err != nil {
		return fmt.Errorf("set stream: %w", err)
	}

	*req = *sreq
	return nil
}

// DeserializeResponse deserializes a response for AWS Json 1.0.
func (p *Protocol) DeserializeResponse(
	ctx context.Context,
	schema *smithy.OperationSchema,
	types *smithy.TypeRegistry,
	resp *smithyhttp.Response,
	out smithy.Deserializable,
) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return p.deserializeError(types, resp)
	}

	if schema.IsOutputEventStream() {
		if schema.IsInputEventStream() {
			if err := p.eventstream.RequireBidiHTTP2(resp.Proto, resp.ProtoMajor); err != nil {
				return err
			}
		}
		return nil
	}

	if schema.Output == nil {
		return nil
	}

	buf, err := p.bufs.Get(resp.Body)
	if err != nil {
		return &smithy.DeserializationError{Err: err}
	}
	defer p.bufs.Put(buf)

	payload := buf.Bytes()
	if len(payload) == 0 {
		return nil
	}

	sd := internaljson.NewShapeDeserializer(payload)
	if err := out.Deserialize(sd); err != nil {
		return &smithy.DeserializationError{Err: err}
	}

	return nil
}

// HasInitialEventMessage is true because this is an RPC protocol.
func (*Protocol) HasInitialEventMessage() bool {
	return true
}

// SerializeEventMessage implements [smithyhttp.ClientProtocol].
func (p *Protocol) SerializeEventMessage(schema, variant *smithy.Schema, v smithy.Serializable, w io.Writer) error {
	return p.eventstream.SerializeEventMessage(schema, variant, v, w)
}

// DeserializeEventMessage implements [smithyhttp.ClientProtocol].
func (p *Protocol) DeserializeEventMessage(schema *smithy.Schema, types *smithy.TypeRegistry, r io.Reader) (smithy.Deserializable, error) {
	return p.eventstream.DeserializeEventMessage(schema, types, r)
}

// SerializeInitialRequest implements [smithyhttp.ClientProtocol].
func (p *Protocol) SerializeInitialRequest(schema *smithy.Schema, v smithy.Serializable, w io.Writer) error {
	return p.eventstream.SerializeInitialRequest(schema, v, w)
}

// DeserializeInitialResponse implements [smithyhttp.ClientProtocol].
func (p *Protocol) DeserializeInitialResponse(schema *smithy.Schema, r io.Reader, out smithy.Deserializable) error {
	return p.eventstream.DeserializeInitialResponse(schema, r, out)
}

// this handles both awsJson 1.0 and 1.1 - the only thing that 1.1 adds is
// error shape renaming (basically not having the namespace) but both versions
// of the protocol are supposed to support this anyway so it doesn't matter
func (p *Protocol) deserializeError(types *smithy.TypeRegistry, response *smithyhttp.Response) error {
	var errorBuffer bytes.Buffer
	if _, err := io.Copy(&errorBuffer, response.Body); err != nil {
		return &smithy.DeserializationError{Err: fmt.Errorf("failed to copy error response body, %w", err)}
	}
	errorBody := bytes.NewReader(errorBuffer.Bytes())

	errorCode := "UnknownError"
	errorMessage := errorCode

	var headerCode string
	headerCode = response.Header.Get("X-Amzn-ErrorType")

	var buff [1024]byte
	ringBuffer := smithyio.NewRingBuffer(buff[:])

	body := io.TeeReader(errorBody, ringBuffer)
	decoder := json.NewDecoder(body)
	decoder.UseNumber()
	bodyInfo, err := internaljson.GetProtocolErrorInfo(decoder)
	if err != nil {
		var snapshot bytes.Buffer
		io.Copy(&snapshot, ringBuffer)
		err = &smithy.DeserializationError{
			Err:      fmt.Errorf("failed to decode response body, %w", err),
			Snapshot: snapshot.Bytes(),
		}
		return err
	}

	errorBody.Seek(0, io.SeekStart)
	if typ, ok := internaljson.ResolveProtocolErrorType(headerCode, bodyInfo); ok {
		errorCode = typ
	}
	if len(bodyInfo.Message) != 0 {
		errorMessage = bodyInfo.Message
	}

	errorCode = internaljson.SanitizeErrorCode(errorCode)

	var queryCode string
	var queryFault smithy.ErrorFault
	if p.queryCompatible {
		queryHeader := response.Header.Get("X-Amzn-Query-Error")
		queryCode, queryFault = internalerrors.ParseQueryError(queryHeader)
	}

	perr, ok := types.DeserializableError(errorCode)
	if !ok {
		code := errorCode
		if queryCode != "" {
			code = queryCode
		}
		return &smithy.GenericAPIError{
			Code:    code,
			Message: errorMessage,
			Fault:   queryFault,
		}
	}

	errorBody.Seek(0, io.SeekStart)
	errorBytes, _ := io.ReadAll(errorBody)
	if len(errorBytes) > 0 {
		deser := internaljson.NewShapeDeserializer(errorBytes)
		if err := perr.Deserialize(deser); err != nil {
			return &smithy.DeserializationError{Err: err}
		}
	}

	if queryCode != "" {
		internalerrors.SetErrorCodeOverride(perr, queryCode)
	}

	return perr
}
