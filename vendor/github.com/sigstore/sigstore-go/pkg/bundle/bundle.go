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
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	protodsse "github.com/sigstore/protobuf-specs/gen/pb-go/dsse"
	"golang.org/x/mod/semver"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/sigstore/sigstore-go/pkg/tlog"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

var ErrValidation = errors.New("validation error")
var ErrUnsupportedMediaType = fmt.Errorf("%w: unsupported media type", ErrValidation)
var ErrEmptyBundle = fmt.Errorf("%w: empty protobuf bundle", ErrValidation)
var ErrMissingVerificationMaterial = fmt.Errorf("%w: missing verification material", ErrValidation)
var ErrMissingBundleContent = fmt.Errorf("%w: missing bundle content", ErrValidation)
var ErrUnimplemented = errors.New("unimplemented")
var ErrInvalidAttestation = fmt.Errorf("%w: invalid attestation", ErrValidation)
var ErrMissingEnvelope = fmt.Errorf("%w: missing valid envelope", ErrInvalidAttestation)
var ErrDecodingJSON = fmt.Errorf("%w: decoding json", ErrInvalidAttestation)
var ErrDecodingB64 = fmt.Errorf("%w: decoding base64", ErrInvalidAttestation)

const mediaTypeBase = "application/vnd.dev.sigstore.bundle"

func ErrValidationError(err error) error {
	return fmt.Errorf("%w: %w", ErrValidation, err)
}

type Bundle struct {
	*protobundle.Bundle
	hasInclusionPromise bool
	hasInclusionProof   bool
}

func NewBundle(pbundle *protobundle.Bundle) (*Bundle, error) {
	bundle := &Bundle{
		Bundle:              pbundle,
		hasInclusionPromise: false,
		hasInclusionProof:   false,
	}

	err := bundle.validate()
	if err != nil {
		return nil, err
	}

	return bundle, nil
}

// Deprecated: use Bundle instead
type ProtobufBundle = Bundle

// Deprecated: use NewBundle instead
func NewProtobufBundle(b *protobundle.Bundle) (*ProtobufBundle, error) {
	return NewBundle(b)
}

func (b *Bundle) validate() error {
	bundleVersion, err := b.Version()
	if err != nil {
		return fmt.Errorf("error getting bundle version: %w", err)
	}

	// if bundle version is < 0.1, return error
	if semver.Compare(bundleVersion, "v0.1") < 0 {
		return fmt.Errorf("%w: bundle version %s is not supported", ErrUnsupportedMediaType, bundleVersion)
	}

	// fetch tlog entries, as next check needs to check them for inclusion proof/promise
	entries, err := b.TlogEntries()
	if err != nil {
		return err
	}

	// if bundle version == v0.1, require inclusion promise
	if semver.Compare(bundleVersion, "v0.1") == 0 {
		if len(entries) > 0 && !b.hasInclusionPromise {
			return errors.New("inclusion promises missing in bundle (required for bundle v0.1)")
		}
	} else {
		// if bundle version >= v0.2, require inclusion proof
		if len(entries) > 0 && !b.hasInclusionProof {
			return errors.New("inclusion proof missing in bundle (required for bundle v0.2)")
		}
	}

	// if bundle version >= v0.3, require verification material to not be X.509 certificate chain (only single certificate is allowed)
	if semver.Compare(bundleVersion, "v0.3") >= 0 {
		certs := b.VerificationMaterial.GetX509CertificateChain()

		if certs != nil {
			return errors.New("verification material cannot be X.509 certificate chain (for bundle v0.3)")
		}
	}

	// if bundle version is >= v0.4, return error as this version is not supported
	if semver.Compare(bundleVersion, "v0.4") >= 0 {
		return fmt.Errorf("%w: bundle version %s is not yet supported", ErrUnsupportedMediaType, bundleVersion)
	}

	err = validateBundle(b.Bundle)
	if err != nil {
		return fmt.Errorf("invalid bundle: %w", err)
	}
	return nil
}

// MediaTypeString returns a mediatype string for the specified bundle version.
// The function returns an error if the resulting string does validate.
func MediaTypeString(version string) (string, error) {
	if version == "" {
		return "", fmt.Errorf("unable to build media type string, no version defined")
	}

	var mtString string

	version = strings.TrimPrefix(version, "v")
	mtString = fmt.Sprintf("%s.v%s+json", mediaTypeBase, strings.TrimPrefix(version, "v"))

	if version == "0.1" || version == "0.2" {
		mtString = fmt.Sprintf("%s+json;version=%s", mediaTypeBase, strings.TrimPrefix(version, "v"))
	}

	if _, err := getBundleVersion(mtString); err != nil {
		return "", fmt.Errorf("unable to build mediatype: %w", err)
	}

	return mtString, nil
}

func (b *Bundle) Version() (string, error) {
	return getBundleVersion(b.MediaType)
}

func getBundleVersion(mediaType string) (string, error) {
	switch mediaType {
	case mediaTypeBase + "+json;version=0.1":
		return "v0.1", nil
	case mediaTypeBase + "+json;version=0.2":
		return "v0.2", nil
	case mediaTypeBase + "+json;version=0.3":
		return "v0.3", nil
	}
	if strings.HasPrefix(mediaType, mediaTypeBase+".v") && strings.HasSuffix(mediaType, "+json") {
		version := strings.TrimPrefix(mediaType, mediaTypeBase+".")
		version = strings.TrimSuffix(version, "+json")
		if semver.IsValid(version) {
			return version, nil
		}
		return "", fmt.Errorf("%w: invalid bundle version: %s", ErrUnsupportedMediaType, version)
	}
	return "", fmt.Errorf("%w: %s", ErrUnsupportedMediaType, mediaType)
}

func validateBundle(b *protobundle.Bundle) error {
	if b == nil {
		return ErrEmptyBundle
	}

	if b.Content == nil {
		return ErrMissingBundleContent
	}

	switch b.Content.(type) {
	case *protobundle.Bundle_DsseEnvelope, *protobundle.Bundle_MessageSignature:
	default:
		return fmt.Errorf("invalid bundle content: bundle content must be either a message signature or dsse envelope")
	}

	if b.VerificationMaterial == nil || b.VerificationMaterial.Content == nil {
		return ErrMissingVerificationMaterial
	}

	switch b.VerificationMaterial.Content.(type) {
	case *protobundle.VerificationMaterial_PublicKey, *protobundle.VerificationMaterial_Certificate, *protobundle.VerificationMaterial_X509CertificateChain:
	default:
		return fmt.Errorf("invalid verification material content: verification material must be one of public key, x509 certificate and x509 certificate chain")
	}

	return nil
}

func LoadJSONFromPath(path string) (*Bundle, error) {
	var bundle Bundle
	bundle.Bundle = new(protobundle.Bundle)

	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	err = bundle.UnmarshalJSON(contents)
	if err != nil {
		return nil, err
	}

	return &bundle, nil
}

func (b *Bundle) MarshalJSON() ([]byte, error) {
	return protojson.Marshal(b.Bundle)
}

func (b *Bundle) UnmarshalJSON(data []byte) error {
	b.Bundle = new(protobundle.Bundle)
	err := protojson.Unmarshal(data, b.Bundle)
	if err != nil {
		return err
	}

	err = b.validate()
	if err != nil {
		return err
	}

	return nil
}

func (b *Bundle) VerificationContent() (verify.VerificationContent, error) {
	if b.VerificationMaterial == nil {
		return nil, ErrMissingVerificationMaterial
	}

	switch content := b.VerificationMaterial.GetContent().(type) {
	case *protobundle.VerificationMaterial_X509CertificateChain:
		if content.X509CertificateChain == nil {
			return nil, ErrMissingVerificationMaterial
		}
		certs := content.X509CertificateChain.GetCertificates()
		if len(certs) == 0 || certs[0].RawBytes == nil {
			return nil, ErrMissingVerificationMaterial
		}
		parsedCert, err := x509.ParseCertificate(certs[0].RawBytes)
		if err != nil {
			return nil, ErrValidationError(err)
		}
		cert := &Certificate{
			certificate: parsedCert,
		}
		return cert, nil
	case *protobundle.VerificationMaterial_Certificate:
		if content.Certificate == nil || content.Certificate.RawBytes == nil {
			return nil, ErrMissingVerificationMaterial
		}
		parsedCert, err := x509.ParseCertificate(content.Certificate.RawBytes)
		if err != nil {
			return nil, ErrValidationError(err)
		}
		cert := &Certificate{
			certificate: parsedCert,
		}
		return cert, nil
	case *protobundle.VerificationMaterial_PublicKey:
		if content.PublicKey == nil {
			return nil, ErrMissingVerificationMaterial
		}
		pk := &PublicKey{
			hint: content.PublicKey.Hint,
		}
		return pk, nil

	default:
		return nil, ErrMissingVerificationMaterial
	}
}

func (b *Bundle) HasInclusionPromise() bool {
	return b.hasInclusionPromise
}

func (b *Bundle) HasInclusionProof() bool {
	return b.hasInclusionProof
}

func (b *Bundle) TlogEntries() ([]*tlog.Entry, error) {
	if b.VerificationMaterial == nil {
		return nil, nil
	}

	tlogEntries := make([]*tlog.Entry, len(b.VerificationMaterial.TlogEntries))
	var err error
	for i, entry := range b.VerificationMaterial.TlogEntries {
		tlogEntries[i], err = tlog.ParseTransparencyLogEntry(entry)
		if err != nil {
			return nil, ErrValidationError(err)
		}

		if tlogEntries[i].HasInclusionPromise() {
			b.hasInclusionPromise = true
		}
		if tlogEntries[i].HasInclusionProof() {
			b.hasInclusionProof = true
		}
	}

	return tlogEntries, nil
}

func (b *Bundle) SignatureContent() (verify.SignatureContent, error) {
	switch content := b.Content.(type) { //nolint:gocritic
	case *protobundle.Bundle_DsseEnvelope:
		envelope, err := parseEnvelope(content.DsseEnvelope)
		if err != nil {
			return nil, err
		}
		return envelope, nil
	case *protobundle.Bundle_MessageSignature:
		if content.MessageSignature == nil || content.MessageSignature.MessageDigest == nil {
			return nil, ErrMissingVerificationMaterial
		}
		return NewMessageSignature(
			content.MessageSignature.MessageDigest.Digest,
			protocommon.HashAlgorithm_name[int32(content.MessageSignature.MessageDigest.Algorithm)],
			content.MessageSignature.Signature,
		), nil
	}
	return nil, ErrMissingVerificationMaterial
}

func (b *Bundle) Envelope() (*Envelope, error) {
	switch content := b.Content.(type) { //nolint:gocritic
	case *protobundle.Bundle_DsseEnvelope:
		envelope, err := parseEnvelope(content.DsseEnvelope)
		if err != nil {
			return nil, err
		}
		return envelope, nil
	}
	return nil, ErrMissingVerificationMaterial
}

func (b *Bundle) Timestamps() ([][]byte, error) {
	if b.VerificationMaterial == nil {
		return nil, ErrMissingVerificationMaterial
	}

	signedTimestamps := make([][]byte, 0)

	if b.VerificationMaterial.TimestampVerificationData == nil {
		return signedTimestamps, nil
	}

	for _, timestamp := range b.VerificationMaterial.TimestampVerificationData.Rfc3161Timestamps {
		signedTimestamps = append(signedTimestamps, timestamp.SignedTimestamp)
	}

	return signedTimestamps, nil
}

// MinVersion returns true if the bundle version is greater than or equal to the expected version.
func (b *Bundle) MinVersion(expectVersion string) bool {
	version, err := b.Version()
	if err != nil {
		return false
	}

	if !strings.HasPrefix(expectVersion, "v") {
		expectVersion = "v" + expectVersion
	}

	return semver.Compare(version, expectVersion) >= 0
}

func parseEnvelope(input *protodsse.Envelope) (*Envelope, error) {
	if input == nil {
		return nil, ErrMissingEnvelope
	}
	output := &dsse.Envelope{}
	payload := input.GetPayload()
	if payload == nil {
		return nil, ErrMissingEnvelope
	}
	output.Payload = base64.StdEncoding.EncodeToString([]byte(payload))
	output.PayloadType = string(input.GetPayloadType())
	output.Signatures = make([]dsse.Signature, len(input.GetSignatures()))
	for i, sig := range input.GetSignatures() {
		if sig == nil {
			return nil, ErrMissingEnvelope
		}
		output.Signatures[i].KeyID = sig.GetKeyid()
		output.Signatures[i].Sig = base64.StdEncoding.EncodeToString(sig.GetSig())
	}
	return &Envelope{Envelope: output}, nil
}
