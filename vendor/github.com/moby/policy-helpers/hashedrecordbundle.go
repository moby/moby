package verifier

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"strconv"
	"strings"
	"time"

	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	v1common "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	v1 "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tlog"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

const (
	annotationCert = "dev.sigstore.cosign/certificate"
	// annotationChain     = "dev.sigstore.cosign/chain"
	annotationSignature = "dev.cosignproject.cosign/signature"
	annotationBundle    = "dev.sigstore.cosign/bundle"
)

// hashedRecordSignedEntity implements verify.SignedEntity using cosign oldbundle format.
type hashedRecordSignedEntity struct {
	mfst  *ocispecs.Manifest
	cert  verify.VerificationContent
	sig   *messageSignatureContent
	isDHI bool
}

var _ verify.SignedEntity = &hashedRecordSignedEntity{}
var _ verify.SignatureContent = &hashedRecordSignedEntity{}
var _ verify.VerificationContent = &hashedRecordSignedEntity{}

func newHashedRecordSignedEntity(mfst *ocispecs.Manifest, isDHI bool) (verify.SignedEntity, error) {
	if len(mfst.Layers) == 0 {
		return nil, errors.New("no layers in manifest")
	}
	desc := mfst.Layers[0]
	sigStr, ok := desc.Annotations[annotationSignature]
	if !ok {
		return nil, errors.New("no signature annotation found")
	}
	sig, err := base64.StdEncoding.DecodeString(sigStr)
	if err != nil {
		return nil, errors.Wrapf(err, "decode signature")
	}
	dgstBytest, err := hex.DecodeString(desc.Digest.Hex())
	if err != nil {
		return nil, errors.Wrapf(err, "decode digest")
	}

	hr := &hashedRecordSignedEntity{
		mfst: mfst,
		sig: &messageSignatureContent{
			digest:          dgstBytest,
			signature:       sig,
			digestAlgorithm: desc.Digest.Algorithm().String(),
		},
		isDHI: isDHI,
	}

	if !isDHI {
		certData := desc.Annotations[annotationCert]
		if certData == "" {
			return nil, errors.Errorf("no certificate annotation found")
		}
		block, _ := pem.Decode([]byte(certData))
		if block == nil {
			return nil, errors.New("no PEM certificate found in annotation")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		hr.cert = bundle.NewCertificate(cert)
	}

	return hr, nil
}

func (d *hashedRecordSignedEntity) HasInclusionPromise() bool {
	return true
}

func (d *hashedRecordSignedEntity) HasInclusionProof() bool {
	return true
}

func (d *hashedRecordSignedEntity) SignatureContent() (verify.SignatureContent, error) {
	return d, nil
}

func (d *hashedRecordSignedEntity) Timestamps() ([][]byte, error) {
	return nil, nil
}

func (d *hashedRecordSignedEntity) TlogEntries() ([]*tlog.Entry, error) {
	bundleBytes, ok := d.extractBundle()
	if !ok {
		return nil, nil
	}
	bundle, err := parseRekorBundle(bundleBytes)
	if err != nil {
		return nil, errors.Wrap(err, "parse rekor bundle")
	}
	logIDRaw, err := hex.DecodeString(bundle.LogID)
	if err != nil {
		return nil, errors.Wrap(err, "decode logID")
	}

	tl, err := tlog.NewTlogEntry(&v1.TransparencyLogEntry{
		LogIndex:          bundle.LogIndex,
		LogId:             &v1common.LogId{KeyId: logIDRaw},
		IntegratedTime:    bundle.IntegratedTime,
		CanonicalizedBody: bundle.Body,
		KindVersion: &v1.KindVersion{
			Kind:    "hashedrekord",
			Version: "0.0.1",
		},
		InclusionPromise: &v1.InclusionPromise{
			SignedEntryTimestamp: bundle.Signature,
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "create tlog entry")
	}
	return []*tlog.Entry{tl}, nil
}

func (d *hashedRecordSignedEntity) VerificationContent() (verify.VerificationContent, error) {
	return d, nil
}

func (d *hashedRecordSignedEntity) Version() (string, error) {
	return "v0.1", nil
}

func (d *hashedRecordSignedEntity) Signature() []byte {
	return d.sig.signature
}

func (d *hashedRecordSignedEntity) EnvelopeContent() verify.EnvelopeContent {
	return nil
}

func (d *hashedRecordSignedEntity) MessageSignatureContent() verify.MessageSignatureContent {
	return d.sig
}

type messageSignatureContent struct {
	digest          []byte
	digestAlgorithm string
	signature       []byte
}

func (m *messageSignatureContent) Digest() []byte {
	return m.digest
}

func (m *messageSignatureContent) DigestAlgorithm() string {
	return m.digestAlgorithm
}

func (m *messageSignatureContent) Signature() []byte {
	return m.signature
}

// CompareKey traces parameters and returns false.
func (d *hashedRecordSignedEntity) CompareKey(k any, tm root.TrustedMaterial) bool {
	if d.isDHI {
		return (&bundle.PublicKey{}).CompareKey(k, tm)
	}
	if _, ok := k.(*x509.Certificate); !ok {
		return false
	}
	return d.cert.CompareKey(k, tm)
}

func (d *hashedRecordSignedEntity) ValidAtTime(t time.Time, tm root.TrustedMaterial) bool {
	if d.isDHI {
		return (&bundle.PublicKey{}).ValidAtTime(t, tm)
	}
	return d.cert.ValidAtTime(t, tm)
}

func (d *hashedRecordSignedEntity) Certificate() *x509.Certificate {
	if d.isDHI {
		return nil
	}
	return d.cert.Certificate()
}

func (d *hashedRecordSignedEntity) PublicKey() verify.PublicKeyProvider {
	if d.isDHI {
		return bundle.PublicKey{}
	}
	return d.cert.PublicKey()
}

func (d *hashedRecordSignedEntity) extractBundle() ([]byte, bool) {
	if len(d.mfst.Layers) == 0 {
		return nil, false
	}
	desc := d.mfst.Layers[0]
	bundleStr := desc.Annotations[annotationBundle]
	if bundleStr == "" {
		return nil, false
	}
	return []byte(bundleStr), true
}

type rekorBundle struct {
	Body           []byte
	Signature      []byte
	LogID          string
	IntegratedTime int64
	LogIndex       int64
}

func parseRekorBundle(bundleBytes []byte) (*rekorBundle, error) {
	var nb struct {
		Content struct {
			VerificationMaterial struct {
				TlogEntries []struct {
					LogIndex any `json:"logIndex"`
					LogID    struct {
						KeyID string `json:"keyId"`
					} `json:"logId"`
					IntegratedTime   any `json:"integratedTime"`
					InclusionPromise struct {
						SignedEntryTimestamp []byte `json:"signedEntryTimestamp"`
					} `json:"inclusionPromise"`
					CanonicalizedBody []byte `json:"canonicalizedBody"`
				} `json:"tlogEntries"`
			} `json:"verificationMaterial"`
		} `json:"content"`
	}
	if json.Unmarshal(bundleBytes, &nb) == nil && len(nb.Content.VerificationMaterial.TlogEntries) > 0 {
		e := nb.Content.VerificationMaterial.TlogEntries[0]
		if len(e.CanonicalizedBody) != 0 && len(e.InclusionPromise.SignedEntryTimestamp) != 0 {
			b := &rekorBundle{
				Body:      e.CanonicalizedBody,
				Signature: e.InclusionPromise.SignedEntryTimestamp,
				LogID:     strings.ToLower(e.LogID.KeyID),
			}

			it, err1 := anyToInt64(e.IntegratedTime)
			if err1 == nil {
				b.IntegratedTime = it
			}
			li, err2 := anyToInt64(e.LogIndex)
			if err2 == nil {
				b.LogIndex = li
			}
			return b, nil
		}
	}

	// Fallback to older cosign bundle shape
	var bundle struct {
		SignedEntryTimestamp []byte `json:"SignedEntryTimestamp"`
		Payload              struct {
			Body           []byte `json:"body"`
			LogID          any    `json:"logID"`
			IntegratedTime any    `json:"integratedTime"`
			LogIndex       any    `json:"logIndex"`
		} `json:"Payload"`
		LogID          any `json:"logID"`
		IntegratedTime any `json:"integratedTime"`
		LogIndex       any `json:"logIndex"`
	}
	if err := json.Unmarshal(bundleBytes, &bundle); err != nil {
		return nil, errors.Wrap(err, "parse bundle json")
	}

	b := &rekorBundle{
		Body:      bundle.Payload.Body,
		Signature: bundle.SignedEntryTimestamp,
	}

	// Prefer top-level fields when present; otherwise fall back to nested under Payload
	// Handle string/number types
	if s, ok := bundle.LogID.(string); ok {
		b.LogID = s
	}
	if b.LogID == "" {
		if s, ok := bundle.Payload.LogID.(string); ok {
			b.LogID = s
		}
	}
	if v, err := anyToInt64(bundle.IntegratedTime); err == nil {
		b.IntegratedTime = v
	}
	if b.IntegratedTime == 0 {
		if v, err := anyToInt64(bundle.Payload.IntegratedTime); err == nil {
			b.IntegratedTime = v
		}
	}
	if v, err := anyToInt64(bundle.LogIndex); err == nil {
		b.LogIndex = v
	}
	if b.LogIndex == 0 {
		if v, err := anyToInt64(bundle.Payload.LogIndex); err == nil {
			b.LogIndex = v
		}
	}
	return b, nil
}

func anyToInt64(v any) (int64, error) {
	switch t := v.(type) {
	case nil:
		return 0, errors.New("nil")
	case float64:
		return int64(t), nil
	case json.Number:
		return t.Int64()
	case string:
		if t == "" {
			return 0, errors.New("empty")
		}
		return strconv.ParseInt(t, 10, 64)
	case int64:
		return t, nil
	case int:
		return int64(t), nil
	default:
		return 0, errors.Errorf("unsupported type %T", v)
	}
}
