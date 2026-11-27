package awsquery

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/smithy-go"
	internales "github.com/aws/smithy-go/internal/eventstream"
	internalsync "github.com/aws/smithy-go/internal/sync"
	"github.com/aws/smithy-go/traits"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	internalquery "github.com/aws/smithy-go/transport/http/protocol/internal/query"
	internalxml "github.com/aws/smithy-go/transport/http/protocol/internal/xml"
)

// ProtocolOptions configures aws.protocols#awsQuery.
type ProtocolOptions struct{}

// Protocol implements aws.protocols#awsQuery.
type Protocol struct {
	eventstream internales.NoEventStream

	version string

	bufs *internalsync.BufferPool
}

var _ smithyhttp.ClientProtocol = (*Protocol)(nil)

// New returns an instance of the awsQuery protocol. The service version is
// pulled from the ServiceVersion trait on the service schema.
func New(service *smithy.ServiceSchema, opts ...func(*ProtocolOptions)) *Protocol {
	var o ProtocolOptions
	for _, fn := range opts {
		fn(&o)
	}
	return &Protocol{version: service.Version, bufs: internalsync.NewBufferPool()}
}

// ID identifies the protocol.
func (*Protocol) ID() smithy.ShapeID {
	return smithy.ShapeID{Namespace: "aws.protocols", Name: "awsQuery"}
}

// HasInitialEventMessage implements [smithyhttp.ClientProtocol].
func (p *Protocol) HasInitialEventMessage() bool {
	return p.eventstream.HasInitialEventMessage()
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

// SerializeRequest serializes a request for awsQuery.
func (p *Protocol) SerializeRequest(
	ctx context.Context,
	schema *smithy.OperationSchema,
	in smithy.Serializable,
	req *smithyhttp.Request,
) error {
	req.Method = http.MethodPost
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if len(req.URL.Path) == 0 {
		req.URL.Path = "/"
	}

	ss := internalquery.NewShapeSerializer(schema.ID().Name, p.version)
	if schema.Input != nil {
		in.Serialize(ss)
	}

	sreq, err := req.SetStream(bytes.NewReader(ss.Bytes()))
	if err != nil {
		return fmt.Errorf("set stream: %w", err)
	}

	*req = *sreq
	return nil
}

// DeserializeResponse deserializes a response for awsQuery.
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

	// NOTE: payload aliases the pooled buffer, which is reused for the next
	// response, so nothing may retain a reference into it past this call.
	buf, err := p.bufs.Get(resp.Body)
	if err != nil {
		return &smithy.DeserializationError{Err: err}
	}
	defer p.bufs.Put(buf)

	payload := buf.Bytes()

	if len(payload) == 0 {
		return nil
	}

	sd, err := internalxml.NewShapeDeserializerAtElement(payload, schema.ID().Name+"Result")
	if err != nil {
		if schema.Output == nil {
			return nil
		}
		return &smithy.DeserializationError{Err: err}
	}

	return out.Deserialize(sd)
}

func (p *Protocol) deserializeError(types *smithy.TypeRegistry, resp *smithyhttp.Response) error {
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return &smithy.DeserializationError{Err: err}
	}

	errorCode, errorMessage, errorBody, err := internalxml.GetProtocolErrorInfo(payload)
	if err != nil {
		return &smithy.DeserializationError{Err: err}
	}

	// resolveError checks both direct shape name and @awsQueryError trait.
	perr, ok := resolveError(types, errorCode)
	if !ok {
		return &smithy.GenericAPIError{
			Code:    errorCode,
			Message: errorMessage,
		}
	}

	if len(errorBody) > 0 {
		sd := internalxml.NewShapeDeserializer(errorBody)
		if err := perr.Deserialize(sd); err != nil {
			return &smithy.DeserializationError{Err: err}
		}
	}

	return perr
}

func resolveError(types *smithy.TypeRegistry, code string) (smithy.DeserializableError, bool) {
	if perr, ok := types.DeserializableError(code); ok {
		return perr, true
	}

	for _, entry := range types.Entries {
		if entry.Schema == nil {
			continue
		}

		if t, ok := smithy.SchemaTrait[*traits.AWSQueryError](entry.Schema); ok {
			if t.ErrorCode == code {
				v := entry.New()
				if perr, ok := v.(smithy.DeserializableError); ok {
					return perr, true
				}
			}
		}
	}

	return nil, false
}
