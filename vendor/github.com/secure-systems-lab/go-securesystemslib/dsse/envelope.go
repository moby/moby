package dsse

import (
	"encoding/base64"
	"fmt"
)

/*
Envelope captures an envelope as described by the DSSE specification. See here:
https://github.com/secure-systems-lab/dsse/blob/master/envelope.md
*/
type Envelope struct {
	PayloadType string      `json:"payloadType"`
	Payload     string      `json:"payload"`
	Signatures  []Signature `json:"signatures"`
}

/*
DecodeB64Payload returns the serialized body, decoded from the envelope's
payload field. A flexible decoder is used, first trying standard base64, then
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
PAE implements the DSSE Pre-Authentic Encoding
https://github.com/secure-systems-lab/dsse/blob/master/protocol.md#signature-definition
*/
func PAE(payloadType string, payload []byte) []byte {
	return []byte(fmt.Sprintf("DSSEv1 %d %s %d %s",
		len(payloadType), payloadType,
		len(payload), payload))
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
			return nil, fmt.Errorf("unable to base64 decode payload (is payload in the right format?)")
		}
	}

	return b, nil
}
