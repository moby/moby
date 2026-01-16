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

package root

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	prototrustroot "github.com/sigstore/protobuf-specs/gen/pb-go/trustroot/v1"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"google.golang.org/protobuf/encoding/protojson"
)

const TrustedRootMediaType01 = "application/vnd.dev.sigstore.trustedroot+json;version=0.1"

type TrustedRoot struct {
	BaseTrustedMaterial
	trustedRoot             *prototrustroot.TrustedRoot
	rekorLogs               map[string]*TransparencyLog
	certificateAuthorities  []CertificateAuthority
	ctLogs                  map[string]*TransparencyLog
	timestampingAuthorities []TimestampingAuthority
}

type TransparencyLog struct {
	BaseURL             string
	ID                  []byte
	ValidityPeriodStart time.Time
	ValidityPeriodEnd   time.Time
	// This is the hash algorithm used by the Merkle tree
	HashFunc  crypto.Hash
	PublicKey crypto.PublicKey
	// The hash algorithm used during signature creation
	SignatureHashFunc crypto.Hash
}

const (
	defaultTrustedRoot = "trusted_root.json"
)

func (tr *TrustedRoot) TimestampingAuthorities() []TimestampingAuthority {
	return tr.timestampingAuthorities
}

func (tr *TrustedRoot) FulcioCertificateAuthorities() []CertificateAuthority {
	return tr.certificateAuthorities
}

func (tr *TrustedRoot) RekorLogs() map[string]*TransparencyLog {
	return tr.rekorLogs
}

func (tr *TrustedRoot) CTLogs() map[string]*TransparencyLog {
	return tr.ctLogs
}

func (tr *TrustedRoot) MarshalJSON() ([]byte, error) {
	err := tr.constructProtoTrustRoot()
	if err != nil {
		return nil, fmt.Errorf("failed constructing protobuf TrustRoot representation: %w", err)
	}

	return protojson.Marshal(tr.trustedRoot)
}

func NewTrustedRootFromProtobuf(protobufTrustedRoot *prototrustroot.TrustedRoot) (trustedRoot *TrustedRoot, err error) {
	if protobufTrustedRoot.GetMediaType() != TrustedRootMediaType01 {
		return nil, fmt.Errorf("unsupported TrustedRoot media type: %s", protobufTrustedRoot.GetMediaType())
	}

	trustedRoot = &TrustedRoot{trustedRoot: protobufTrustedRoot}
	trustedRoot.rekorLogs, err = ParseTransparencyLogs(protobufTrustedRoot.GetTlogs())
	if err != nil {
		return nil, err
	}

	trustedRoot.certificateAuthorities, err = ParseCertificateAuthorities(protobufTrustedRoot.GetCertificateAuthorities())
	if err != nil {
		return nil, err
	}

	trustedRoot.timestampingAuthorities, err = ParseTimestampingAuthorities(protobufTrustedRoot.GetTimestampAuthorities())
	if err != nil {
		return nil, err
	}

	trustedRoot.ctLogs, err = ParseTransparencyLogs(protobufTrustedRoot.GetCtlogs())
	if err != nil {
		return nil, err
	}

	return trustedRoot, nil
}

func ParseTransparencyLogs(tlogs []*prototrustroot.TransparencyLogInstance) (transparencyLogs map[string]*TransparencyLog, err error) {
	transparencyLogs = make(map[string]*TransparencyLog)
	for _, tlog := range tlogs {
		if tlog.GetHashAlgorithm() != protocommon.HashAlgorithm_SHA2_256 {
			return nil, fmt.Errorf("unsupported tlog hash algorithm: %s", tlog.GetHashAlgorithm())
		}
		if tlog.GetLogId() == nil {
			return nil, fmt.Errorf("tlog missing log ID")
		}
		if tlog.GetLogId().GetKeyId() == nil {
			return nil, fmt.Errorf("tlog missing log ID key ID")
		}
		encodedKeyID := hex.EncodeToString(tlog.GetLogId().GetKeyId())

		if tlog.GetPublicKey() == nil {
			return nil, fmt.Errorf("tlog missing public key")
		}
		if tlog.GetPublicKey().GetRawBytes() == nil {
			return nil, fmt.Errorf("tlog missing public key raw bytes")
		}

		var hashFunc crypto.Hash
		switch tlog.GetHashAlgorithm() {
		case protocommon.HashAlgorithm_SHA2_256:
			hashFunc = crypto.SHA256
		default:
			return nil, fmt.Errorf("unsupported hash function for the tlog")
		}

		tlogEntry := &TransparencyLog{
			BaseURL:           tlog.GetBaseUrl(),
			ID:                tlog.GetLogId().GetKeyId(),
			HashFunc:          hashFunc,
			SignatureHashFunc: crypto.SHA256,
		}

		switch tlog.GetPublicKey().GetKeyDetails() {
		case protocommon.PublicKeyDetails_PKIX_ECDSA_P256_SHA_256,
			protocommon.PublicKeyDetails_PKIX_ECDSA_P384_SHA_384,
			protocommon.PublicKeyDetails_PKIX_ECDSA_P521_SHA_512:
			key, err := x509.ParsePKIXPublicKey(tlog.GetPublicKey().GetRawBytes())
			if err != nil {
				return nil, fmt.Errorf("failed to parse public key for tlog: %s %w",
					tlog.GetBaseUrl(),
					err,
				)
			}
			var ecKey *ecdsa.PublicKey
			var ok bool
			if ecKey, ok = key.(*ecdsa.PublicKey); !ok {
				return nil, fmt.Errorf("tlog public key is not ECDSA: %s", tlog.GetPublicKey().GetKeyDetails())
			}
			tlogEntry.PublicKey = ecKey
		// This key format has public key in PKIX RSA format and PKCS1#1v1.5 or RSASSA-PSS signature
		case protocommon.PublicKeyDetails_PKIX_RSA_PKCS1V15_2048_SHA256,
			protocommon.PublicKeyDetails_PKIX_RSA_PKCS1V15_3072_SHA256,
			protocommon.PublicKeyDetails_PKIX_RSA_PKCS1V15_4096_SHA256:
			key, err := x509.ParsePKIXPublicKey(tlog.GetPublicKey().GetRawBytes())
			if err != nil {
				return nil, fmt.Errorf("failed to parse public key for tlog: %s %w",
					tlog.GetBaseUrl(),
					err,
				)
			}
			var rsaKey *rsa.PublicKey
			var ok bool
			if rsaKey, ok = key.(*rsa.PublicKey); !ok {
				return nil, fmt.Errorf("tlog public key is not RSA: %s", tlog.GetPublicKey().GetKeyDetails())
			}
			tlogEntry.PublicKey = rsaKey
		case protocommon.PublicKeyDetails_PKIX_ED25519:
			key, err := x509.ParsePKIXPublicKey(tlog.GetPublicKey().GetRawBytes())
			if err != nil {
				return nil, fmt.Errorf("failed to parse public key for tlog: %s %w",
					tlog.GetBaseUrl(),
					err,
				)
			}
			var edKey ed25519.PublicKey
			var ok bool
			if edKey, ok = key.(ed25519.PublicKey); !ok {
				return nil, fmt.Errorf("tlog public key is not RSA: %s", tlog.GetPublicKey().GetKeyDetails())
			}
			tlogEntry.PublicKey = edKey
		// This key format is deprecated, but currently in use for Sigstore staging instance
		case protocommon.PublicKeyDetails_PKCS1_RSA_PKCS1V5: //nolint:staticcheck
			key, err := x509.ParsePKCS1PublicKey(tlog.GetPublicKey().GetRawBytes())
			if err != nil {
				return nil, fmt.Errorf("failed to parse public key for tlog: %s %w",
					tlog.GetBaseUrl(),
					err,
				)
			}
			tlogEntry.PublicKey = key
		default:
			return nil, fmt.Errorf("unsupported tlog public key type: %s", tlog.GetPublicKey().GetKeyDetails())
		}

		tlogEntry.SignatureHashFunc = getSignatureHashAlgo(tlogEntry.PublicKey)
		transparencyLogs[encodedKeyID] = tlogEntry

		if validFor := tlog.GetPublicKey().GetValidFor(); validFor != nil {
			if validFor.GetStart() != nil {
				transparencyLogs[encodedKeyID].ValidityPeriodStart = validFor.GetStart().AsTime()
			} else {
				return nil, fmt.Errorf("tlog missing public key validity period start time")
			}
			if validFor.GetEnd() != nil {
				transparencyLogs[encodedKeyID].ValidityPeriodEnd = validFor.GetEnd().AsTime()
			}
		} else {
			return nil, fmt.Errorf("tlog missing public key validity period")
		}
	}
	return transparencyLogs, nil
}

func ParseCertificateAuthorities(certAuthorities []*prototrustroot.CertificateAuthority) (certificateAuthorities []CertificateAuthority, err error) {
	certificateAuthorities = make([]CertificateAuthority, len(certAuthorities))
	for i, certAuthority := range certAuthorities {
		certificateAuthority, err := ParseCertificateAuthority(certAuthority)
		if err != nil {
			return nil, err
		}
		certificateAuthorities[i] = certificateAuthority
	}
	return certificateAuthorities, nil
}

func ParseCertificateAuthority(certAuthority *prototrustroot.CertificateAuthority) (*FulcioCertificateAuthority, error) {
	if certAuthority == nil {
		return nil, fmt.Errorf("CertificateAuthority is nil")
	}
	certChain := certAuthority.GetCertChain()
	if certChain == nil {
		return nil, fmt.Errorf("CertificateAuthority missing cert chain")
	}
	chainLen := len(certChain.GetCertificates())
	if chainLen < 1 {
		return nil, fmt.Errorf("CertificateAuthority cert chain is empty")
	}

	certificateAuthority := &FulcioCertificateAuthority{
		URI: certAuthority.Uri,
	}
	for i, cert := range certChain.GetCertificates() {
		parsedCert, err := x509.ParseCertificate(cert.RawBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate for %s %w",
				certAuthority.Uri,
				err,
			)
		}
		if i < chainLen-1 {
			certificateAuthority.Intermediates = append(certificateAuthority.Intermediates, parsedCert)
		} else {
			certificateAuthority.Root = parsedCert
		}
	}
	validFor := certAuthority.GetValidFor()
	if validFor != nil {
		start := validFor.GetStart()
		if start != nil {
			certificateAuthority.ValidityPeriodStart = start.AsTime()
		}
		end := validFor.GetEnd()
		if end != nil {
			certificateAuthority.ValidityPeriodEnd = end.AsTime()
		}
	}

	certificateAuthority.URI = certAuthority.Uri

	return certificateAuthority, nil
}

func ParseTimestampingAuthorities(certAuthorities []*prototrustroot.CertificateAuthority) (timestampingAuthorities []TimestampingAuthority, err error) {
	timestampingAuthorities = make([]TimestampingAuthority, len(certAuthorities))
	for i, certAuthority := range certAuthorities {
		timestampingAuthority, err := ParseTimestampingAuthority(certAuthority)
		if err != nil {
			return nil, err
		}
		timestampingAuthorities[i] = timestampingAuthority
	}
	return timestampingAuthorities, nil
}

func ParseTimestampingAuthority(certAuthority *prototrustroot.CertificateAuthority) (TimestampingAuthority, error) {
	if certAuthority == nil {
		return nil, fmt.Errorf("CertificateAuthority is nil")
	}
	certChain := certAuthority.GetCertChain()
	if certChain == nil {
		return nil, fmt.Errorf("CertificateAuthority missing cert chain")
	}
	chainLen := len(certChain.GetCertificates())
	if chainLen < 1 {
		return nil, fmt.Errorf("CertificateAuthority cert chain is empty")
	}

	timestampingAuthority := &SigstoreTimestampingAuthority{
		URI: certAuthority.Uri,
	}
	for i, cert := range certChain.GetCertificates() {
		parsedCert, err := x509.ParseCertificate(cert.RawBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate for %s %w",
				certAuthority.Uri,
				err,
			)
		}
		switch {
		case i == 0 && !parsedCert.IsCA:
			timestampingAuthority.Leaf = parsedCert
		case i < chainLen-1:
			timestampingAuthority.Intermediates = append(timestampingAuthority.Intermediates, parsedCert)
		case i == chainLen-1:
			timestampingAuthority.Root = parsedCert
		}
	}
	validFor := certAuthority.GetValidFor()
	if validFor != nil {
		start := validFor.GetStart()
		if start != nil {
			timestampingAuthority.ValidityPeriodStart = start.AsTime()
		}
		end := validFor.GetEnd()
		if end != nil {
			timestampingAuthority.ValidityPeriodEnd = end.AsTime()
		}
	}

	timestampingAuthority.URI = certAuthority.Uri

	return timestampingAuthority, nil
}

func NewTrustedRootFromPath(path string) (*TrustedRoot, error) {
	trustedrootJSON, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read trusted root %w",
			err,
		)
	}

	return NewTrustedRootFromJSON(trustedrootJSON)
}

// NewTrustedRootFromJSON returns the Sigstore trusted root.
func NewTrustedRootFromJSON(rootJSON []byte) (*TrustedRoot, error) {
	pbTrustedRoot, err := NewTrustedRootProtobuf(rootJSON)
	if err != nil {
		return nil, err
	}

	return NewTrustedRootFromProtobuf(pbTrustedRoot)
}

// NewTrustedRootProtobuf returns the Sigstore trusted root as a protobuf.
func NewTrustedRootProtobuf(rootJSON []byte) (*prototrustroot.TrustedRoot, error) {
	pbTrustedRoot := &prototrustroot.TrustedRoot{}
	err := protojson.Unmarshal(rootJSON, pbTrustedRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to proto-json unmarshal trusted root: %w", err)
	}
	return pbTrustedRoot, nil
}

// NewTrustedRoot initializes a TrustedRoot object from a mediaType string, list of Fulcio
// certificate authorities, list of timestamp authorities and maps of ctlogs and rekor
// transparency log instances.
// mediaType must be TrustedRootMediaType01 ("application/vnd.dev.sigstore.trustedroot+json;version=0.1").
func NewTrustedRoot(mediaType string,
	certificateAuthorities []CertificateAuthority,
	certificateTransparencyLogs map[string]*TransparencyLog,
	timestampAuthorities []TimestampingAuthority,
	transparencyLogs map[string]*TransparencyLog) (*TrustedRoot, error) {
	// document that we assume 1 cert chain per target and with certs already ordered from leaf to root
	if mediaType != TrustedRootMediaType01 {
		return nil, fmt.Errorf("unsupported TrustedRoot media type: %s, must be %s", mediaType, TrustedRootMediaType01)
	}
	tr := &TrustedRoot{
		certificateAuthorities:  certificateAuthorities,
		ctLogs:                  certificateTransparencyLogs,
		timestampingAuthorities: timestampAuthorities,
		rekorLogs:               transparencyLogs,
	}
	return tr, nil
}

// FetchTrustedRoot fetches the Sigstore trusted root from TUF and returns it.
func FetchTrustedRoot() (*TrustedRoot, error) {
	return FetchTrustedRootWithOptions(tuf.DefaultOptions())
}

// FetchTrustedRootWithOptions fetches the trusted root from TUF with the given options and returns it.
func FetchTrustedRootWithOptions(opts *tuf.Options) (*TrustedRoot, error) {
	client, err := tuf.New(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create TUF client %w", err)
	}
	return GetTrustedRoot(client)
}

// GetTrustedRoot returns the trusted root
func GetTrustedRoot(c *tuf.Client) (*TrustedRoot, error) {
	jsonBytes, err := c.GetTarget(defaultTrustedRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to get trusted root from TUF client %w",
			err,
		)
	}
	return NewTrustedRootFromJSON(jsonBytes)
}

func getSignatureHashAlgo(pubKey crypto.PublicKey) crypto.Hash {
	var h crypto.Hash
	switch pk := pubKey.(type) {
	case *rsa.PublicKey:
		h = crypto.SHA256
	case *ecdsa.PublicKey:
		switch pk.Curve {
		case elliptic.P256():
			h = crypto.SHA256
		case elliptic.P384():
			h = crypto.SHA384
		case elliptic.P521():
			h = crypto.SHA512
		default:
			h = crypto.SHA256
		}
	case ed25519.PublicKey:
		h = crypto.SHA512
	default:
		h = crypto.SHA256
	}
	return h
}

// LiveTrustedRoot is a wrapper around TrustedRoot that periodically
// refreshes the trusted root from TUF. This is needed for long-running
// processes to ensure that the trusted root does not expire.
type LiveTrustedRoot struct {
	*TrustedRoot
	mu sync.RWMutex
}

// NewLiveTrustedRoot returns a LiveTrustedRoot that will periodically
// refresh the trusted root from TUF.
func NewLiveTrustedRoot(opts *tuf.Options) (*LiveTrustedRoot, error) {
	return NewLiveTrustedRootFromTarget(opts, defaultTrustedRoot)
}

// NewLiveTrustedRootFromTarget returns a LiveTrustedRoot that will
// periodically refresh the trusted root from TUF using the provided target.
func NewLiveTrustedRootFromTarget(opts *tuf.Options, target string) (*LiveTrustedRoot, error) {
	return NewLiveTrustedRootFromTargetWithPeriod(opts, target, 24*time.Hour)
}

// NewLiveTrustedRootFromTargetWithPeriod returns a LiveTrustedRoot that
// performs a TUF refresh with the provided period, accesssing the provided
// target.
func NewLiveTrustedRootFromTargetWithPeriod(opts *tuf.Options, target string, rfPeriod time.Duration) (*LiveTrustedRoot, error) {
	client, err := tuf.New(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create TUF client %w", err)
	}

	b, err := client.GetTarget(target)
	if err != nil {
		return nil, fmt.Errorf("failed to get target from TUF client %w", err)
	}

	tr, err := NewTrustedRootFromJSON(b)
	if err != nil {
		return nil, err
	}
	ltr := &LiveTrustedRoot{
		TrustedRoot: tr,
		mu:          sync.RWMutex{},
	}

	ticker := time.NewTicker(rfPeriod)
	go func() {
		for range ticker.C {
			client, err = tuf.New(opts)
			if err != nil {
				log.Printf("error creating TUF client: %v", err)
			}

			b, err := client.GetTarget(target)
			if err != nil {
				log.Printf("error fetching trusted root: %v", err)
			}

			newTr, err := NewTrustedRootFromJSON(b)
			if err != nil {
				log.Printf("error fetching trusted root: %v", err)
				continue
			}
			ltr.mu.Lock()
			ltr.TrustedRoot = newTr
			ltr.mu.Unlock()
			log.Printf("successfully refreshed the TUF root")
		}
	}()
	return ltr, nil
}

func (l *LiveTrustedRoot) TimestampingAuthorities() []TimestampingAuthority {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.TrustedRoot.TimestampingAuthorities()
}

func (l *LiveTrustedRoot) FulcioCertificateAuthorities() []CertificateAuthority {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.TrustedRoot.FulcioCertificateAuthorities()
}

func (l *LiveTrustedRoot) RekorLogs() map[string]*TransparencyLog {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.TrustedRoot.RekorLogs()
}

func (l *LiveTrustedRoot) CTLogs() map[string]*TransparencyLog {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.TrustedRoot.CTLogs()
}

func (l *LiveTrustedRoot) PublicKeyVerifier(keyID string) (TimeConstrainedVerifier, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.TrustedRoot.PublicKeyVerifier(keyID)
}
