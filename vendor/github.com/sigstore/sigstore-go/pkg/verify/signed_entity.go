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

package verify

import (
	"crypto/x509"
	"encoding/asn1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	in_toto "github.com/in-toto/attestation/go/v1"
	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	VerificationResultMediaType01 = "application/vnd.dev.sigstore.verificationresult+json;version=0.1"
)

type Verifier struct {
	trustedMaterial root.TrustedMaterial
	config          VerifierConfig
}

type VerifierConfig struct { // nolint: revive
	// requireSignedTimestamps requires RFC3161 timestamps to verify
	// short-lived certificates
	requireSignedTimestamps bool
	// signedTimestampThreshold is the minimum number of verified
	// RFC3161 timestamps in a bundle
	signedTimestampThreshold int
	// requireIntegratedTimestamps requires log entry integrated timestamps to
	// verify short-lived certificates
	requireIntegratedTimestamps bool
	// integratedTimeThreshold is the minimum number of log entry
	// integrated timestamps in a bundle
	integratedTimeThreshold int
	// requireObserverTimestamps requires RFC3161 timestamps and/or log
	// integrated timestamps to verify short-lived certificates
	requireObserverTimestamps bool
	// observerTimestampThreshold is the minimum number of verified
	// RFC3161 timestamps and/or log integrated timestamps in a bundle
	observerTimestampThreshold int
	// requireTlogEntries requires log inclusion proofs in a bundle
	requireTlogEntries bool
	// tlogEntriesThreshold is the minimum number of verified inclusion
	// proofs in a bundle
	tlogEntriesThreshold int
	// requireSCTs requires SCTs in Fulcio certificates
	requireSCTs bool
	// ctlogEntriesThreshold is the minimum number of verified SCTs in
	// a Fulcio certificate
	ctlogEntriesThreshold int
	// useCurrentTime uses the current time rather than a provided signed
	// or log timestamp. Most workflows will not use this option
	useCurrentTime bool
	// allowNoTimestamp can be used to skip timestamp checks when a key
	// is used rather than a certificate.
	allowNoTimestamp bool
}

type VerifierOption func(*VerifierConfig) error

// NewVerifier creates a new Verifier. It takes a
// root.TrustedMaterial, which contains a set of trusted public keys and
// certificates, and a set of VerifierConfigurators, which set the config
// that determines the behaviour of the Verify function.
//
// VerifierConfig's set of options should match the properties of a given
// Sigstore deployment, i.e. whether to expect SCTs, Tlog entries, or signed
// timestamps.
func NewVerifier(trustedMaterial root.TrustedMaterial, options ...VerifierOption) (*Verifier, error) {
	var err error
	c := VerifierConfig{}

	for _, opt := range options {
		err = opt(&c)
		if err != nil {
			return nil, fmt.Errorf("failed to configure verifier: %w", err)
		}
	}

	err = c.Validate()
	if err != nil {
		return nil, err
	}

	v := &Verifier{
		trustedMaterial: trustedMaterial,
		config:          c,
	}

	return v, nil
}

// TODO: Remove the following deprecated functions in a future release before sigstore-go 2.0.

// Deprecated: Use Verifier instead
type SignedEntityVerifier = Verifier

// Deprecated: Use NewVerifier instead
func NewSignedEntityVerifier(trustedMaterial root.TrustedMaterial, options ...VerifierOption) (*Verifier, error) {
	return NewVerifier(trustedMaterial, options...)
}

// WithSignedTimestamps configures the Verifier to expect RFC 3161
// timestamps from a Timestamp Authority, verify them using the TrustedMaterial's
// TimestampingAuthorities(), and, if it exists, use the resulting timestamp(s)
// to verify the Fulcio certificate.
func WithSignedTimestamps(threshold int) VerifierOption {
	return func(c *VerifierConfig) error {
		if threshold < 1 {
			return errors.New("signed timestamp threshold must be at least 1")
		}
		c.requireSignedTimestamps = true
		c.signedTimestampThreshold = threshold
		return nil
	}
}

// WithObserverTimestamps configures the Verifier to expect
// timestamps from either an RFC3161 timestamp authority or a log's
// SignedEntryTimestamp. These are verified using the TrustedMaterial's
// TimestampingAuthorities() or RekorLogs(), and used to verify
// the Fulcio certificate.
func WithObserverTimestamps(threshold int) VerifierOption {
	return func(c *VerifierConfig) error {
		if threshold < 1 {
			return errors.New("observer timestamp threshold must be at least 1")
		}
		c.requireObserverTimestamps = true
		c.observerTimestampThreshold = threshold
		return nil
	}
}

// WithTransparencyLog configures the Verifier to expect
// Transparency Log inclusion proofs or SignedEntryTimestamps, verifying them
// using the TrustedMaterial's RekorLogs().
func WithTransparencyLog(threshold int) VerifierOption {
	return func(c *VerifierConfig) error {
		if threshold < 1 {
			return errors.New("transparency log entry threshold must be at least 1")
		}
		c.requireTlogEntries = true
		c.tlogEntriesThreshold = threshold
		return nil
	}
}

// WithIntegratedTimestamps configures the Verifier to
// expect log entry integrated timestamps from either SignedEntryTimestamps
// or live log lookups.
func WithIntegratedTimestamps(threshold int) VerifierOption {
	return func(c *VerifierConfig) error {
		c.requireIntegratedTimestamps = true
		c.integratedTimeThreshold = threshold
		return nil
	}
}

// WithSignedCertificateTimestamps configures the Verifier to
// expect the Fulcio certificate to have a SignedCertificateTimestamp, and
// verify it using the TrustedMaterial's CTLogAuthorities().
func WithSignedCertificateTimestamps(threshold int) VerifierOption {
	return func(c *VerifierConfig) error {
		if threshold < 1 {
			return errors.New("ctlog entry threshold must be at least 1")
		}
		c.requireSCTs = true
		c.ctlogEntriesThreshold = threshold
		return nil
	}
}

// WithCurrentTime configures the Verifier to not expect
// any timestamps from either a Timestamp Authority or a Transparency Log.
// This option should not be enabled when verifying short-lived certificates,
// as an observer timestamp is needed. This option is useful primarily for
// private deployments with long-lived code signing certificates.
func WithCurrentTime() VerifierOption {
	return func(c *VerifierConfig) error {
		c.useCurrentTime = true
		return nil
	}
}

// WithNoObserverTimestamps configures the Verifier to not expect
// any timestamps from either a Timestamp Authority or a Transparency Log
// and to not use the current time to verify a certificate. This may only
// be used when verifying with keys rather than certificates.
func WithNoObserverTimestamps() VerifierOption {
	return func(c *VerifierConfig) error {
		c.allowNoTimestamp = true
		return nil
	}
}

func (c *VerifierConfig) Validate() error {
	if c.allowNoTimestamp && (c.requireObserverTimestamps || c.requireSignedTimestamps || c.requireIntegratedTimestamps || c.useCurrentTime) {
		return errors.New("specify WithNoObserverTimestamps() without any other verifier options")
	}
	if !c.requireObserverTimestamps && !c.requireSignedTimestamps && !c.requireIntegratedTimestamps && !c.useCurrentTime && !c.allowNoTimestamp {
		return errors.New("when initializing a new Verifier, you must specify at least one of " +
			"WithObserverTimestamps(), WithSignedTimestamps(), WithIntegratedTimestamps() or WithCurrentTime(), " +
			"or exclusively specify WithNoObserverTimestamps()")
	}

	return nil
}

type VerificationResult struct {
	MediaType          string                        `json:"mediaType"`
	Statement          *in_toto.Statement            `json:"statement,omitempty"`
	Signature          *SignatureVerificationResult  `json:"signature,omitempty"`
	VerifiedTimestamps []TimestampVerificationResult `json:"verifiedTimestamps"`
	VerifiedIdentity   *CertificateIdentity          `json:"verifiedIdentity,omitempty"`
}

type SignatureVerificationResult struct {
	PublicKeyID *[]byte              `json:"publicKeyId,omitempty"`
	Certificate *certificate.Summary `json:"certificate,omitempty"`
}

type TimestampVerificationResult struct {
	Type      string    `json:"type"`
	URI       string    `json:"uri"`
	Timestamp time.Time `json:"timestamp"`
}

func NewVerificationResult() *VerificationResult {
	return &VerificationResult{
		MediaType: VerificationResultMediaType01,
	}
}

// MarshalJSON deals with protojson needed for the Statement.
// Can be removed when https://github.com/in-toto/attestation/pull/403 is merged.
func (b *VerificationResult) MarshalJSON() ([]byte, error) {
	statement, err := protojson.Marshal(b.Statement)
	if err != nil {
		return nil, err
	}
	// creating a type alias to avoid infinite recursion, as MarshalJSON is
	// not copied into the alias.
	type Alias VerificationResult
	return json.Marshal(struct {
		Alias
		Statement json.RawMessage `json:"statement,omitempty"`
	}{
		Alias:     Alias(*b),
		Statement: statement,
	})
}

func (b *VerificationResult) UnmarshalJSON(data []byte) error {
	b.Statement = &in_toto.Statement{}
	type Alias VerificationResult
	aux := &struct {
		Alias
		Statement json.RawMessage `json:"statement,omitempty"`
	}{
		Alias: Alias(*b),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	return protojson.Unmarshal(aux.Statement, b.Statement)
}

type PolicyOption func(*PolicyConfig) error
type ArtifactPolicyOption func(*PolicyConfig) error

// PolicyBuilder is responsible for building & validating a PolicyConfig
type PolicyBuilder struct {
	artifactPolicy ArtifactPolicyOption
	policyOptions  []PolicyOption
}

func (pc PolicyBuilder) options() []PolicyOption {
	arr := []PolicyOption{PolicyOption(pc.artifactPolicy)}
	return append(arr, pc.policyOptions...)
}

func (pc PolicyBuilder) BuildConfig() (*PolicyConfig, error) {
	var err error

	policy := &PolicyConfig{}
	for _, applyOption := range pc.options() {
		err = applyOption(policy)
		if err != nil {
			return nil, err
		}
	}

	if err := policy.validate(); err != nil {
		return nil, err
	}

	return policy, nil
}

type ArtifactDigest struct {
	Algorithm string
	Digest    []byte
}

type PolicyConfig struct {
	ignoreArtifact        bool
	ignoreIdentities      bool
	requireSigningKey     bool
	certificateIdentities CertificateIdentities
	verifyArtifacts       bool
	artifacts             []io.Reader
	verifyArtifactDigests bool
	artifactDigests       []ArtifactDigest
}

func (p *PolicyConfig) withVerifyAlreadyConfigured() error {
	if p.verifyArtifacts || p.verifyArtifactDigests {
		return errors.New("only one invocation of WithArtifact/WithArtifacts/WithArtifactDigest/WithArtifactDigests is allowed")
	}

	return nil
}

func (p *PolicyConfig) validate() error {
	if p.RequireIdentities() && len(p.certificateIdentities) == 0 {
		return errors.New("can't verify identities without providing at least one identity")
	}

	return nil
}

// RequireArtifact returns true if the Verify algorithm should perform
// signature verification with an an artifact provided by either the
// WithArtifact or the WithArtifactDigest functions.
//
// By default, unless explicitly turned off, we should always expect to verify
// a SignedEntity's signature using an artifact. Bools are initialized to false,
// so this behaviour is therefore controlled by the ignoreArtifact field.
//
// Double negatives are confusing, though. To aid with comprehension of the
// main Verify loop, this function therefore just wraps the double negative.
func (p *PolicyConfig) RequireArtifact() bool {
	return !p.ignoreArtifact
}

// RequireIdentities returns true if the Verify algorithm should check
// whether the SignedEntity's certificate was created by one of the identities
// provided by the WithCertificateIdentity function.
//
// By default, unless explicitly turned off, we should always expect to enforce
// that a SignedEntity's certificate was created by an Identity we trust. Bools
// are initialized to false, so this behaviour is therefore controlled by the
// ignoreIdentities field.
//
// Double negatives are confusing, though. To aid with comprehension of the
// main Verify loop, this function therefore just wraps the double negative.
func (p *PolicyConfig) RequireIdentities() bool {
	return !p.ignoreIdentities
}

// RequireSigningKey returns true if we expect the SignedEntity to be signed
// with a key and not a certificate.
func (p *PolicyConfig) RequireSigningKey() bool {
	return p.requireSigningKey
}

func NewPolicy(artifactOpt ArtifactPolicyOption, options ...PolicyOption) PolicyBuilder {
	return PolicyBuilder{artifactPolicy: artifactOpt, policyOptions: options}
}

// WithoutIdentitiesUnsafe allows the caller of Verify to skip enforcing any
// checks on the identity that created the SignedEntity being verified.
//
// Do not use this option unless you know what you are doing!
//
// As the name implies, using WithoutIdentitiesUnsafe is not safe: outside of
// exceptional circumstances, we should always enforce that the SignedEntity
// being verified was signed by a trusted CertificateIdentity.
//
// For more information, consult WithCertificateIdentity.
func WithoutIdentitiesUnsafe() PolicyOption {
	return func(p *PolicyConfig) error {
		if len(p.certificateIdentities) > 0 {
			return errors.New("can't use WithoutIdentitiesUnsafe while specifying CertificateIdentities")
		}

		p.ignoreIdentities = true
		return nil
	}
}

// WithCertificateIdentity allows the caller of Verify to enforce that the
// SignedEntity being verified was signed by the given identity, as defined by
// the Fulcio certificate embedded in the entity. If this policy is enabled,
// but the SignedEntity does not have a certificate, verification will fail.
//
// Providing this function multiple times will concatenate the provided
// CertificateIdentity to the list of identities being checked.
//
// If all of the provided CertificateIdentities fail to match the Fulcio
// certificate, then verification will fail. If *any* CertificateIdentity
// matches, then verification will succeed. Therefore, each CertificateIdentity
// provided to this function must define a "sufficient" identity to trust.
//
// The CertificateIdentity struct allows callers to specify:
// - The exact value, or Regexp, of the SubjectAlternativeName
// - The exact value of any Fulcio OID X.509 extension, i.e. Issuer
//
// For convenience, consult the NewShortCertificateIdentity function.
func WithCertificateIdentity(identity CertificateIdentity) PolicyOption {
	return func(p *PolicyConfig) error {
		if p.ignoreIdentities {
			return errors.New("can't use WithCertificateIdentity while using WithoutIdentitiesUnsafe")
		}
		if p.requireSigningKey {
			return errors.New("can't use WithCertificateIdentity while using WithKey")
		}

		p.certificateIdentities = append(p.certificateIdentities, identity)
		return nil
	}
}

// WithKey allows the caller of Verify to require the SignedEntity being
// verified was signed with a key and not a certificate.
func WithKey() PolicyOption {
	return func(p *PolicyConfig) error {
		if len(p.certificateIdentities) > 0 {
			return errors.New("can't use WithKey while using WithCertificateIdentity")
		}

		p.requireSigningKey = true
		p.ignoreIdentities = true
		return nil
	}
}

// WithoutArtifactUnsafe allows the caller of Verify to skip checking whether
// the SignedEntity was created from, or references, an artifact.
//
// WithoutArtifactUnsafe can only be used with SignedEntities that contain a
// DSSE envelope. If the the SignedEntity has a MessageSignature, providing
// this policy option will cause verification to always fail, since
// MessageSignatures can only be verified in the presence of an Artifact or
// artifact digest. See WithArtifact/WithArtifactDigest for more information.
//
// Do not use this function unless you know what you are doing!
//
// As the name implies, using WithoutArtifactUnsafe is not safe: outside of
// exceptional circumstances, SignedEntities should always be verified with
// an artifact.
func WithoutArtifactUnsafe() ArtifactPolicyOption {
	return func(p *PolicyConfig) error {
		if err := p.withVerifyAlreadyConfigured(); err != nil {
			return err
		}

		p.ignoreArtifact = true
		return nil
	}
}

// WithArtifact allows the caller of Verify to enforce that the SignedEntity
// being verified was created from, or references, a given artifact.
//
// If the SignedEntity contains a DSSE envelope, then the artifact digest is
// calculated from the given artifact, and compared to the digest in the
// envelope's statement.
func WithArtifact(artifact io.Reader) ArtifactPolicyOption {
	return func(p *PolicyConfig) error {
		if err := p.withVerifyAlreadyConfigured(); err != nil {
			return err
		}

		if p.ignoreArtifact {
			return errors.New("can't use WithArtifact while using WithoutArtifactUnsafe")
		}

		p.verifyArtifacts = true
		p.artifacts = []io.Reader{artifact}
		return nil
	}
}

// WithArtifacts allows the caller of Verify to enforce that the SignedEntity
// being verified was created from, or references, a slice of artifacts.
//
// If the SignedEntity contains a DSSE envelope, then the artifact digest is
// calculated from the given artifact, and compared to the digest in the
// envelope's statement.
func WithArtifacts(artifacts []io.Reader) ArtifactPolicyOption {
	return func(p *PolicyConfig) error {
		if err := p.withVerifyAlreadyConfigured(); err != nil {
			return err
		}

		if p.ignoreArtifact {
			return errors.New("can't use WithArtifacts while using WithoutArtifactUnsafe")
		}

		p.verifyArtifacts = true
		p.artifacts = artifacts
		return nil
	}
}

// WithArtifactDigest allows the caller of Verify to enforce that the
// SignedEntity being verified was created for a given artifact digest.
//
// If the SignedEntity contains a MessageSignature that was signed using the
// ED25519 algorithm, then providing only an artifactDigest will fail; the
// whole artifact must be provided. Use WithArtifact instead.
//
// If the SignedEntity contains a DSSE envelope, then the artifact digest is
// compared to the digest in the envelope's statement.
func WithArtifactDigest(algorithm string, artifactDigest []byte) ArtifactPolicyOption {
	return func(p *PolicyConfig) error {
		if err := p.withVerifyAlreadyConfigured(); err != nil {
			return err
		}

		if p.ignoreArtifact {
			return errors.New("can't use WithArtifactDigest while using WithoutArtifactUnsafe")
		}

		p.verifyArtifactDigests = true
		p.artifactDigests = []ArtifactDigest{{
			Algorithm: algorithm,
			Digest:    artifactDigest,
		}}
		return nil
	}
}

// WithArtifactDigests allows the caller of Verify to enforce that the
// SignedEntity being verified was created for a given array of artifact digests.
//
// If the SignedEntity contains a DSSE envelope, then the artifact digests
// are compared to the digests in the envelope's statement.
//
// If the SignedEntity does not contain a DSSE envelope, verification fails.
func WithArtifactDigests(digests []ArtifactDigest) ArtifactPolicyOption {
	return func(p *PolicyConfig) error {
		if err := p.withVerifyAlreadyConfigured(); err != nil {
			return err
		}

		if p.ignoreArtifact {
			return errors.New("can't use WithArtifactDigests while using WithoutArtifactUnsafe")
		}

		p.verifyArtifactDigests = true
		p.artifactDigests = digests
		return nil
	}
}

// Verify checks the cryptographic integrity of a given SignedEntity according
// to the options configured in the NewVerifier. Its purpose is to
// determine whether the SignedEntity was created by a Sigstore deployment we
// trust, as defined by keys in our TrustedMaterial.
//
// If the SignedEntity contains a MessageSignature, then the artifact or its
// digest must be provided to the Verify function, as it is required to verify
// the signature. See WithArtifact and WithArtifactDigest for more details.
//
// If and only if verification is successful, Verify will return a
// VerificationResult struct whose contents' integrity have been verified.
// Verify may then verify the contents of the VerificationResults using supplied
// PolicyOptions. See WithCertificateIdentity for more details.
//
// Callers of this function SHOULD ALWAYS:
//   - (if the signed entity has a certificate) verify that its Subject Alternate
//     Name matches a trusted identity, and that its OID Issuer field matches an
//     expected value
//   - (if the signed entity has a dsse envelope) verify that the envelope's
//     statement's subject matches the artifact being verified
func (v *Verifier) Verify(entity SignedEntity, pb PolicyBuilder) (*VerificationResult, error) {
	policy, err := pb.BuildConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build policy: %w", err)
	}

	// Let's go by the spec: https://docs.google.com/document/d/1kbhK2qyPPk8SLavHzYSDM8-Ueul9_oxIMVFuWMWKz0E/edit#heading=h.g11ovq2s1jxh
	// > ## Transparency Log Entry
	verifiedTlogTimestamps, err := v.VerifyTransparencyLogInclusion(entity)
	if err != nil {
		return nil, fmt.Errorf("failed to verify log inclusion: %w", err)
	}

	// > ## Establishing a Time for the Signature
	// > First, establish a time for the signature. This timestamp is required to validate the certificate chain, so this step comes first.
	verifiedTimestamps, err := v.VerifyObserverTimestamps(entity, verifiedTlogTimestamps)
	if err != nil {
		return nil, fmt.Errorf("failed to verify timestamps: %w", err)
	}

	verificationContent, err := entity.VerificationContent()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch verification content: %w", err)
	}

	var signedWithCertificate bool
	var certSummary certificate.Summary

	// If the bundle was signed with a long-lived key, and does not have a Fulcio certificate,
	// then skip the certificate verification steps
	if leafCert := verificationContent.Certificate(); leafCert != nil {
		if policy.RequireSigningKey() {
			return nil, errors.New("expected key signature, not certificate")
		}
		if v.config.allowNoTimestamp {
			return nil, errors.New("must provide timestamp to verify certificate")
		}

		signedWithCertificate = true

		// Get the summary before modifying the cert extensions
		certSummary, err = certificate.SummarizeCertificate(leafCert)
		if err != nil {
			return nil, fmt.Errorf("failed to summarize certificate: %w", err)
		}

		// From spec:
		// > ## Certificate
		// > …
		// > The Verifier MUST perform certification path validation (RFC 5280 §6) of the certificate chain with the pre-distributed Fulcio root certificate(s) as a trust anchor, but with a fake “current time.” If a timestamp from the timestamping service is available, the Verifier MUST perform path validation using the timestamp from the Timestamping Service. If a timestamp from the Transparency Service is available, the Verifier MUST perform path validation using the timestamp from the Transparency Service. If both are available, the Verifier performs path validation twice. If either fails, verification fails.

		// Go does not support the OtherName GeneralName SAN extension. If
		// Fulcio issued the certificate with an OtherName SAN, it will be
		// handled by SummarizeCertificate above, and it must be removed here
		// or the X.509 verification will fail.
		if len(leafCert.UnhandledCriticalExtensions) > 0 {
			var unhandledExts []asn1.ObjectIdentifier
			for _, oid := range leafCert.UnhandledCriticalExtensions {
				if !oid.Equal(cryptoutils.SANOID) {
					unhandledExts = append(unhandledExts, oid)
				}
			}
			leafCert.UnhandledCriticalExtensions = unhandledExts
		}

		var chains [][]*x509.Certificate
		for _, verifiedTs := range verifiedTimestamps {
			// verify the leaf certificate against the root
			chains, err = VerifyLeafCertificate(verifiedTs.Timestamp, leafCert, v.trustedMaterial)
			if err != nil {
				return nil, fmt.Errorf("failed to verify leaf certificate: %w", err)
			}
		}

		// From spec:
		// > Unless performing online verification (see §Alternative Workflows), the Verifier MUST extract the  SignedCertificateTimestamp embedded in the leaf certificate, and verify it as in RFC 9162 §8.1.3, using the verification key from the Certificate Transparency Log.

		if v.config.requireSCTs {
			err = VerifySignedCertificateTimestamp(chains, v.config.ctlogEntriesThreshold, v.trustedMaterial)
			if err != nil {
				return nil, fmt.Errorf("failed to verify signed certificate timestamp: %w", err)
			}
		}
	}

	// If SCTs are required, ensure the bundle is certificate-signed not public key-signed
	if v.config.requireSCTs {
		if verificationContent.PublicKey() != nil {
			return nil, errors.New("SCTs required but bundle is signed with a public key, which cannot contain SCTs")
		}
	}

	// From spec:
	// > ## Signature Verification
	// > The Verifier MUST verify the provided signature for the constructed payload against the key in the leaf of the certificate chain.

	sigContent, err := entity.SignatureContent()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch signature content: %w", err)
	}

	entityVersion, err := entity.Version()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch entity version: %w", err)
	}

	var enableCompat bool
	switch entityVersion {
	case "v0.1":
		fallthrough
	case "0.1":
		fallthrough
	case "v0.2":
		fallthrough
	case "0.2":
		fallthrough
	case "v0.3":
		fallthrough
	case "0.3":
		enableCompat = true
	}
	verifier, err := getSignatureVerifier(sigContent, verificationContent, v.trustedMaterial, enableCompat)
	if err != nil {
		return nil, fmt.Errorf("failed to get signature verifier: %w", err)
	}

	if policy.RequireArtifact() {
		switch {
		case policy.verifyArtifacts:
			err = verifySignatureWithVerifierAndArtifacts(verifier, sigContent, verificationContent, v.trustedMaterial, policy.artifacts)
		case policy.verifyArtifactDigests:
			err = verifySignatureWithVerifierAndArtifactDigests(verifier, sigContent, verificationContent, v.trustedMaterial, policy.artifactDigests)
		default:
			// should never happen, but just in case:
			err = errors.New("no artifact or artifact digest provided")
		}
	} else {
		// verifying with artifact has been explicitly turned off, so just check
		// the signature on the dsse envelope:
		err = verifySignatureWithVerifier(verifier, sigContent, verificationContent, v.trustedMaterial)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to verify signature: %w", err)
	}

	// Hooray! We've verified all of the entity's constituent parts! 🎉 🥳
	// Now we can construct the results object accordingly.
	result := NewVerificationResult()
	if signedWithCertificate {
		result.Signature = &SignatureVerificationResult{
			Certificate: &certSummary,
		}
	} else {
		pubKeyID := []byte(verificationContent.PublicKey().Hint())
		result.Signature = &SignatureVerificationResult{
			PublicKeyID: &pubKeyID,
		}
	}

	// SignatureContent can be either an Envelope or a MessageSignature.
	// If it's an Envelope, let's pop the Statement for our results:
	if envelope := sigContent.EnvelopeContent(); envelope != nil {
		stmt, err := envelope.Statement()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch envelope statement: %w", err)
		}

		result.Statement = stmt
	}

	result.VerifiedTimestamps = verifiedTimestamps

	// Now that the signed entity's crypto material has been verified, and the
	// result struct has been constructed, we can optionally enforce some
	// additional policies:
	// --------------------

	// From ## Certificate section,
	// >The Verifier MUST then check the certificate against the verification policy. Details on how to do this depend on the verification policy, but the Verifier SHOULD check the Issuer X.509 extension (OID 1.3.6.1.4.1.57264.1.1) at a minimum, and will in most cases check the SubjectAlternativeName as well. See  Spec: Fulcio §TODO for example checks on the certificate.
	if policy.RequireIdentities() {
		if !signedWithCertificate {
			// We got asked to verify identities, but the entity was not signed with
			// a certificate. That's a problem!
			return nil, errors.New("can't verify certificate identities: entity was not signed with a certificate")
		}

		if len(policy.certificateIdentities) == 0 {
			return nil, errors.New("can't verify certificate identities: no identities provided")
		}

		matchingCertID, err := policy.certificateIdentities.Verify(certSummary)
		if err != nil {
			return nil, fmt.Errorf("failed to verify certificate identity: %w", err)
		}

		result.VerifiedIdentity = matchingCertID
	}

	return result, nil
}

// VerifyTransparencyLogInclusion verifies TlogEntries if expected. Optionally returns
// a list of verified timestamps from the log integrated timestamps when verifying
// with observer timestamps.
// TODO: Return a different verification result for logs specifically (also for #48)
func (v *Verifier) VerifyTransparencyLogInclusion(entity SignedEntity) ([]TimestampVerificationResult, error) {
	verifiedTimestamps := []TimestampVerificationResult{}

	if v.config.requireTlogEntries {
		// log timestamps should be verified if with WithIntegratedTimestamps or WithObserverTimestamps is used
		verifiedTlogTimestamps, err := VerifyTlogEntry(entity, v.trustedMaterial, v.config.tlogEntriesThreshold,
			v.config.requireIntegratedTimestamps || v.config.requireObserverTimestamps)
		if err != nil {
			return nil, err
		}

		for _, vts := range verifiedTlogTimestamps {
			verifiedTimestamps = append(verifiedTimestamps, TimestampVerificationResult{Type: "Tlog", URI: vts.URI, Timestamp: vts.Time})
		}
	}

	return verifiedTimestamps, nil
}

// VerifyObserverTimestamps verifies RFC3161 signed timestamps, and verifies
// that timestamp thresholds are met with log entry integrated timestamps,
// signed timestamps, or a combination of both. The returned timestamps
// can be used to verify short-lived certificates.
// logTimestamps may be populated with verified log entry integrated timestamps
// In order to be verifiable, a SignedEntity must have at least one verified
// "observer timestamp".
func (v *Verifier) VerifyObserverTimestamps(entity SignedEntity, logTimestamps []TimestampVerificationResult) ([]TimestampVerificationResult, error) {
	verifiedTimestamps := []TimestampVerificationResult{}

	// From spec:
	// > … if verification or timestamp parsing fails, the Verifier MUST abort
	if v.config.requireSignedTimestamps {
		verifiedSignedTimestamps, err := VerifySignedTimestampWithThreshold(entity, v.trustedMaterial, v.config.signedTimestampThreshold)
		if err != nil {
			return nil, err
		}
		for _, vts := range verifiedSignedTimestamps {
			verifiedTimestamps = append(verifiedTimestamps, TimestampVerificationResult{Type: "TimestampAuthority", URI: vts.URI, Timestamp: vts.Time})
		}
	}

	if v.config.requireIntegratedTimestamps {
		if len(logTimestamps) < v.config.integratedTimeThreshold {
			return nil, fmt.Errorf("threshold not met for verified log entry integrated timestamps: %d < %d", len(logTimestamps), v.config.integratedTimeThreshold)
		}
		verifiedTimestamps = append(verifiedTimestamps, logTimestamps...)
	}

	if v.config.requireObserverTimestamps {
		verifiedSignedTimestamps, verificationErrors, err := VerifySignedTimestamp(entity, v.trustedMaterial)
		if err != nil {
			return nil, fmt.Errorf("failed to verify signed timestamps: %w", err)
		}

		// check threshold for both RFC3161 and log timestamps
		tsCount := len(verifiedSignedTimestamps) + len(logTimestamps)
		if tsCount < v.config.observerTimestampThreshold {
			return nil, fmt.Errorf("threshold not met for verified signed & log entry integrated timestamps: %d < %d; error: %w",
				tsCount, v.config.observerTimestampThreshold, errors.Join(verificationErrors...))
		}

		// append all timestamps
		verifiedTimestamps = append(verifiedTimestamps, logTimestamps...)
		for _, vts := range verifiedSignedTimestamps {
			verifiedTimestamps = append(verifiedTimestamps, TimestampVerificationResult{Type: "TimestampAuthority", URI: vts.URI, Timestamp: vts.Time})
		}
	}

	if v.config.useCurrentTime {
		// use current time to verify certificate if no signed timestamps are provided
		verifiedTimestamps = append(verifiedTimestamps, TimestampVerificationResult{Type: "CurrentTime", URI: "", Timestamp: time.Now()})
	}

	if len(verifiedTimestamps) == 0 && !v.config.allowNoTimestamp {
		return nil, fmt.Errorf("no valid observer timestamps found")
	}

	return verifiedTimestamps, nil
}
