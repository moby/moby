package in_toto

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/secure-systems-lab/go-securesystemslib/cjson"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
)

// PayloadType is the payload type used for links and layouts.
const PayloadType = "application/vnd.in-toto+json"

// ErrInvalidPayloadType indicates that the envelope used an unknown payload type
var ErrInvalidPayloadType = errors.New("unknown payload type")

type Envelope struct {
	envelope *dsse.Envelope
	payload  any
}

func loadEnvelope(env *dsse.Envelope) (*Envelope, error) {
	e := &Envelope{envelope: env}

	contentBytes, err := env.DecodeB64Payload()
	if err != nil {
		return nil, err
	}

	payload, err := loadPayload(contentBytes)
	if err != nil {
		return nil, err
	}
	e.payload = payload

	return e, nil
}

func (e *Envelope) SetPayload(payload any) error {
	encodedBytes, err := cjson.EncodeCanonical(payload)
	if err != nil {
		return err
	}

	e.payload = payload
	e.envelope = &dsse.Envelope{
		Payload:     base64.StdEncoding.EncodeToString(encodedBytes),
		PayloadType: PayloadType,
	}

	return nil
}

func (e *Envelope) GetPayload() any {
	return e.payload
}

func (e *Envelope) VerifySignature(key Key) error {
	verifier, err := getSignerVerifierFromKey(key)
	if err != nil {
		return err
	}

	ev, err := dsse.NewEnvelopeVerifier(verifier)
	if err != nil {
		return err
	}

	_, err = ev.Verify(context.Background(), e.envelope)
	return err
}

func (e *Envelope) Sign(key Key) error {
	signer, err := getSignerVerifierFromKey(key)
	if err != nil {
		return err
	}

	es, err := dsse.NewEnvelopeSigner(signer)
	if err != nil {
		return err
	}

	payload, err := e.envelope.DecodeB64Payload()
	if err != nil {
		return err
	}

	env, err := es.SignPayload(context.Background(), e.envelope.PayloadType, payload)
	if err != nil {
		return err
	}

	e.envelope = env
	return nil
}

func (e *Envelope) Sigs() []Signature {
	sigs := []Signature{}
	for _, s := range e.envelope.Signatures {
		sigs = append(sigs, Signature{
			KeyID: s.KeyID,
			Sig:   s.Sig,
		})
	}
	return sigs
}

func (e *Envelope) GetSignatureForKeyID(keyID string) (Signature, error) {
	for _, s := range e.Sigs() {
		if s.KeyID == keyID {
			return s, nil
		}
	}

	return Signature{}, fmt.Errorf("no signature found for key '%s'", keyID)
}

func (e *Envelope) Dump(path string) error {
	jsonBytes, err := json.MarshalIndent(e.envelope, "", "  ")
	if err != nil {
		return err
	}

	// Write JSON bytes to the passed path with permissions (-rw-r--r--)
	err = os.WriteFile(path, jsonBytes, 0644)
	if err != nil {
		return err
	}

	return nil
}

func getSignerVerifierFromKey(key Key) (dsse.SignerVerifier, error) {
	sslibKey := getSSLibKeyFromKey(key)

	switch sslibKey.KeyType {
	case signerverifier.RSAKeyType:
		return signerverifier.NewRSAPSSSignerVerifierFromSSLibKey(&sslibKey)
	case signerverifier.ED25519KeyType:
		return signerverifier.NewED25519SignerVerifierFromSSLibKey(&sslibKey)
	case signerverifier.ECDSAKeyType:
		return signerverifier.NewECDSASignerVerifierFromSSLibKey(&sslibKey)
	}

	return nil, ErrUnsupportedKeyType
}

func getSSLibKeyFromKey(key Key) signerverifier.SSLibKey {
	return signerverifier.SSLibKey{
		KeyType:             key.KeyType,
		KeyIDHashAlgorithms: key.KeyIDHashAlgorithms,
		KeyID:               key.KeyID,
		Scheme:              key.Scheme,
		KeyVal: signerverifier.KeyVal{
			Public:      key.KeyVal.Public,
			Private:     key.KeyVal.Private,
			Certificate: key.KeyVal.Certificate,
		},
	}
}
