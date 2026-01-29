// Copyright 2023 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bundle

import (
	"encoding/base64"

	in_toto "github.com/in-toto/attestation/go/v1"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"google.golang.org/protobuf/encoding/protojson"
)

const IntotoMediaType = "application/vnd.in-toto+json"

type MessageSignature struct {
	digest          []byte
	digestAlgorithm string
	signature       []byte
}

func (m *MessageSignature) Digest() []byte {
	return m.digest
}

func (m *MessageSignature) DigestAlgorithm() string {
	return m.digestAlgorithm
}

func NewMessageSignature(digest []byte, digestAlgorithm string, signature []byte) *MessageSignature {
	return &MessageSignature{
		digest:          digest,
		digestAlgorithm: digestAlgorithm,
		signature:       signature,
	}
}

type Envelope struct {
	*dsse.Envelope
}

func (e *Envelope) Statement() (*in_toto.Statement, error) {
	if e.PayloadType != IntotoMediaType {
		return nil, ErrUnsupportedMediaType
	}

	var statement in_toto.Statement
	raw, err := e.DecodeB64Payload()
	if err != nil {
		return nil, ErrDecodingB64
	}
	err = protojson.Unmarshal(raw, &statement)
	if err != nil {
		return nil, ErrDecodingJSON
	}
	return &statement, nil
}

func (e *Envelope) EnvelopeContent() verify.EnvelopeContent {
	return e
}

func (e *Envelope) RawEnvelope() *dsse.Envelope {
	return e.Envelope
}

func (m *MessageSignature) EnvelopeContent() verify.EnvelopeContent {
	return nil
}

func (e *Envelope) MessageSignatureContent() verify.MessageSignatureContent {
	return nil
}

func (m *MessageSignature) MessageSignatureContent() verify.MessageSignatureContent {
	return m
}

func (m *MessageSignature) Signature() []byte {
	return m.signature
}

func (e *Envelope) Signature() []byte {
	if len(e.Signatures) == 0 {
		return []byte{}
	}

	sigBytes, err := base64.StdEncoding.DecodeString(e.Signatures[0].Sig)
	if err != nil {
		return []byte{}
	}

	return sigBytes
}
