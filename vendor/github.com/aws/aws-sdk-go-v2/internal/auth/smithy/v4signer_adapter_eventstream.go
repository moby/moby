package smithy

import (
	"context"
	"fmt"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	smithygo "github.com/aws/smithy-go"
	"github.com/aws/smithy-go/auth"
	"github.com/aws/smithy-go/eventstream"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

var _ smithyhttp.EventStreamSigner = (*V4SignerAdapter)(nil)

// NewMessageSigner implements [smithyhttp.EventStreamSigner].
func (v *V4SignerAdapter) NewMessageSigner(ctx context.Context, r *smithyhttp.Request, identity auth.Identity, props smithygo.Properties) (eventstream.MessageSigner, error) {
	ca, ok := identity.(*CredentialsAdapter)
	if !ok {
		return nil, fmt.Errorf("unexpected identity type: %T", identity)
	}

	name, ok := smithyhttp.GetSigV4SigningName(&props)
	if !ok {
		return nil, fmt.Errorf("sigv4 signing name is required")
	}

	region, ok := smithyhttp.GetSigV4SigningRegion(&props)
	if !ok {
		return nil, fmt.Errorf("sigv4 signing region is required")
	}

	seed, err := v4.GetSignedRequestSignature(r.Request)
	if err != nil {
		return nil, fmt.Errorf("get seed signature: %w", err)
	}

	return &streamSignerAdapter{
		signer: v4.NewStreamSigner(ca.Credentials, name, region, seed),
	}, nil
}

// streamSignerAdapter adapts v4.StreamSigner to eventstream.MessageSigner.
type streamSignerAdapter struct {
	signer *v4.StreamSigner
}

func (s *streamSignerAdapter) SignMessage(headers, payload []byte, signingTime time.Time) ([]byte, error) {
	return s.signer.GetSignature(context.Background(), headers, payload, signingTime)
}
