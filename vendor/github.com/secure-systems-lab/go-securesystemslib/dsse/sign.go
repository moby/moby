/*
Package dsse implements the Dead Simple Signing Envelope (DSSE)
https://github.com/secure-systems-lab/dsse
*/
package dsse

import (
	"encoding/base64"
	"errors"
	"fmt"
)

// ErrUnknownKey indicates that the implementation does not recognize the
// key.
var ErrUnknownKey = errors.New("unknown key")

// ErrNoSignature indicates that an envelope did not contain any signatures.
var ErrNoSignature = errors.New("no signature found")

// ErrNoSigners indicates that no signer was provided.
var ErrNoSigners = errors.New("no signers provided")

/*
Envelope captures an envelope as described by the Secure Systems Lab
Signing Specification. See here:
https://github.com/secure-systems-lab/signing-spec/blob/master/envelope.md
*/
type Envelope struct {
	PayloadType string      `json:"payloadType"`
	Payload     string      `json:"payload"`
	Signatures  []Signature `json:"signatures"`
}

/*
DecodeB64Payload returns the serialized body, decoded
from the envelope's payload field. A flexible
decoder is used, first trying standard base64, then
URL-encoded base64.
*/
func (e *Envelope) DecodeB64Payload() ([]byte, error) {
	return b64Decode(e.Payload)
}

/*
Signature represents a generic in-toto signature that contains the identifier
of the key which was used to create the signature.
The used signature scheme has to be agreed upon by the signer and verifer
out of band.
The signature is a base64 encoding of the raw bytes from the signature
algorithm.
*/
type Signature struct {
	KeyID string `json:"keyid"`
	Sig   string `json:"sig"`
}

/*
PAE implementes the DSSE Pre-Authentic Encoding
https://github.com/secure-systems-lab/dsse/blob/master/protocol.md#signature-definition
*/
func PAE(payloadType string, payload []byte) []byte {
	return []byte(fmt.Sprintf("DSSEv1 %d %s %d %s",
		len(payloadType), payloadType,
		len(payload), payload))
}

/*
Signer defines the interface for an abstract signing algorithm.
The Signer interface is used to inject signature algorithm implementations
into the EnevelopeSigner. This decoupling allows for any signing algorithm
and key management system can be used.
The full message is provided as the parameter. If the signature algorithm
depends on hashing of the message prior to signature calculation, the
implementor of this interface must perform such hashing.
The function must return raw bytes representing the calculated signature
using the current algorithm, and the key used (if applicable).
For an example see EcdsaSigner in sign_test.go.
*/
type Signer interface {
	Sign(data []byte) ([]byte, error)
	KeyID() (string, error)
}

// SignVerifer provides both the signing and verification interface.
type SignVerifier interface {
	Signer
	Verifier
}

// EnvelopeSigner creates signed Envelopes.
type EnvelopeSigner struct {
	providers []SignVerifier
	ev        *EnvelopeVerifier
}

/*
NewEnvelopeSigner creates an EnvelopeSigner that uses 1+ Signer
algorithms to sign the data.
Creates a verifier with threshold=1, at least one of the providers must validate signitures successfully.
*/
func NewEnvelopeSigner(p ...SignVerifier) (*EnvelopeSigner, error) {
	return NewMultiEnvelopeSigner(1, p...)
}

/*
NewMultiEnvelopeSigner creates an EnvelopeSigner that uses 1+ Signer
algorithms to sign the data.
Creates a verifier with threshold.
threashold indicates the amount of providers that must validate the envelope.
*/
func NewMultiEnvelopeSigner(threshold int, p ...SignVerifier) (*EnvelopeSigner, error) {
	var providers []SignVerifier

	for _, sv := range p {
		if sv != nil {
			providers = append(providers, sv)
		}
	}

	if len(providers) == 0 {
		return nil, ErrNoSigners
	}

	evps := []Verifier{}
	for _, p := range providers {
		evps = append(evps, p.(Verifier))
	}

	ev, err := NewMultiEnvelopeVerifier(threshold, evps...)
	if err != nil {
		return nil, err
	}

	return &EnvelopeSigner{
		providers: providers,
		ev:        ev,
	}, nil
}

/*
SignPayload signs a payload and payload type according to DSSE.
Returned is an envelope as defined here:
https://github.com/secure-systems-lab/dsse/blob/master/envelope.md
One signature will be added for each Signer in the EnvelopeSigner.
*/
func (es *EnvelopeSigner) SignPayload(payloadType string, body []byte) (*Envelope, error) {
	var e = Envelope{
		Payload:     base64.StdEncoding.EncodeToString(body),
		PayloadType: payloadType,
	}

	paeEnc := PAE(payloadType, body)

	for _, signer := range es.providers {
		sig, err := signer.Sign(paeEnc)
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

/*
Verify decodes the payload and verifies the signature.
Any domain specific validation such as parsing the decoded body and
validating the payload type is left out to the caller.
Verify returns a list of accepted keys each including a keyid, public and signiture of the accepted provider keys.
*/
func (es *EnvelopeSigner) Verify(e *Envelope) ([]AcceptedKey, error) {
	return es.ev.Verify(e)
}

/*
Both standard and url encoding are allowed:
https://github.com/secure-systems-lab/dsse/blob/master/envelope.md
*/
func b64Decode(s string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		b, err = base64.URLEncoding.DecodeString(s)
		if err != nil {
			return nil, err
		}
	}

	return b, nil
}
