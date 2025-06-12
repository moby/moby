/*
Package dsse implements the Dead Simple Signing Envelope (DSSE)
https://github.com/secure-systems-lab/dsse
*/
package dsse

import (
	"context"
	"encoding/base64"
	"errors"
)

// ErrNoSigners indicates that no signer was provided.
var ErrNoSigners = errors.New("no signers provided")

// EnvelopeSigner creates signed Envelopes.
type EnvelopeSigner struct {
	providers []SignerVerifier
}

/*
NewEnvelopeSigner creates an EnvelopeSigner that uses 1+ Signer algorithms to
sign the data. Creates a verifier with threshold=1, at least one of the
providers must validate signatures successfully.
*/
func NewEnvelopeSigner(p ...SignerVerifier) (*EnvelopeSigner, error) {
	return NewMultiEnvelopeSigner(1, p...)
}

/*
NewMultiEnvelopeSigner creates an EnvelopeSigner that uses 1+ Signer
algorithms to sign the data. Creates a verifier with threshold. Threshold
indicates the amount of providers that must validate the envelope.
*/
func NewMultiEnvelopeSigner(threshold int, p ...SignerVerifier) (*EnvelopeSigner, error) {
	var providers []SignerVerifier

	for _, sv := range p {
		if sv != nil {
			providers = append(providers, sv)
		}
	}

	if len(providers) == 0 {
		return nil, ErrNoSigners
	}

	return &EnvelopeSigner{
		providers: providers,
	}, nil
}

/*
SignPayload signs a payload and payload type according to DSSE.
Returned is an envelope as defined here:
https://github.com/secure-systems-lab/dsse/blob/master/envelope.md
One signature will be added for each Signer in the EnvelopeSigner.
*/
func (es *EnvelopeSigner) SignPayload(ctx context.Context, payloadType string, body []byte) (*Envelope, error) {
	var e = Envelope{
		Payload:     base64.StdEncoding.EncodeToString(body),
		PayloadType: payloadType,
	}

	paeEnc := PAE(payloadType, body)

	for _, signer := range es.providers {
		sig, err := signer.Sign(ctx, paeEnc)
		if err != nil {
			return nil, err
		}
		keyID, err := signer.KeyID()
		if err != nil {
			keyID = ""
		}

		e.Signatures = append(e.Signatures, Signature{
			KeyID: keyID,
			Sig:   base64.StdEncoding.EncodeToString(sig),
		})
	}

	return &e, nil
}
