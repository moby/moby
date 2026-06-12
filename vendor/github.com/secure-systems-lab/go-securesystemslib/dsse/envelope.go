package dsse

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strconv"
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
	// Pre-size to avoid reallocation. Previously fmt.Sprintf copied payload
	// into a string and []byte(...) copied it again.
	const prefix = "DSSEv1 "
	const sep = " "
	// Max decimal digits for a non-negative int (len() result) on any
	// platform: len("9223372036854775807") == 19. Grow is a hint, so a
	// slight overestimate is harmless.
	const maxDecimalLen = 19
	var b bytes.Buffer
	b.Grow(len(prefix) +
		maxDecimalLen + len(sep) + len(payloadType) + len(sep) +
		maxDecimalLen + len(sep) + len(payload))
	b.WriteString(prefix)
	b.WriteString(strconv.Itoa(len(payloadType)))
	b.WriteByte(' ')
	b.WriteString(payloadType)
	b.WriteByte(' ')
	b.WriteString(strconv.Itoa(len(payload)))
	b.WriteByte(' ')
	b.Write(payload)
	return b.Bytes()
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
