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
	"fmt"
	"sort"
	"time"

	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	prototrustroot "github.com/sigstore/protobuf-specs/gen/pb-go/trustroot/v1"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

func (tr *TrustedRoot) constructProtoTrustRoot() error {
	tr.trustedRoot = &prototrustroot.TrustedRoot{}
	tr.trustedRoot.MediaType = TrustedRootMediaType01

	for logID, transparencyLog := range tr.rekorLogs {
		tlProto, err := transparencyLogToProtobufTL(transparencyLog)
		if err != nil {
			return fmt.Errorf("failed converting rekor log %s to protobuf: %w", logID, err)
		}
		tr.trustedRoot.Tlogs = append(tr.trustedRoot.Tlogs, tlProto)
	}
	// ensure stable sorting of the slice
	sortTlogSlice(tr.trustedRoot.Tlogs)

	for logID, ctLog := range tr.ctLogs {
		ctProto, err := transparencyLogToProtobufTL(ctLog)
		if err != nil {
			return fmt.Errorf("failed converting ctlog %s to protobuf: %w", logID, err)
		}
		tr.trustedRoot.Ctlogs = append(tr.trustedRoot.Ctlogs, ctProto)
	}
	// ensure stable sorting of the slice
	sortTlogSlice(tr.trustedRoot.Ctlogs)

	for _, ca := range tr.certificateAuthorities {
		caProto, err := certificateAuthorityToProtobufCA(ca.(*FulcioCertificateAuthority))
		if err != nil {
			return fmt.Errorf("failed converting fulcio cert chain to protobuf: %w", err)
		}
		tr.trustedRoot.CertificateAuthorities = append(tr.trustedRoot.CertificateAuthorities, caProto)
	}
	// ensure stable sorting of the slice
	sortCASlice(tr.trustedRoot.CertificateAuthorities)

	for _, ca := range tr.timestampingAuthorities {
		caProto, err := timestampingAuthorityToProtobufCA(ca.(*SigstoreTimestampingAuthority))
		if err != nil {
			return fmt.Errorf("failed converting TSA cert chain to protobuf: %w", err)
		}
		tr.trustedRoot.TimestampAuthorities = append(tr.trustedRoot.TimestampAuthorities, caProto)
	}
	// ensure stable sorting of the slice
	sortCASlice(tr.trustedRoot.TimestampAuthorities)

	return nil
}

func sortCASlice(slc []*prototrustroot.CertificateAuthority) {
	sort.Slice(slc, func(i, j int) bool {
		iTime := time.Unix(0, 0)
		jTime := time.Unix(0, 0)

		if slc[i].ValidFor.Start != nil {
			iTime = slc[i].ValidFor.Start.AsTime()
		}
		if slc[j].ValidFor.Start != nil {
			jTime = slc[j].ValidFor.Start.AsTime()
		}

		return iTime.Before(jTime)
	})
}

func sortTlogSlice(slc []*prototrustroot.TransparencyLogInstance) {
	sort.Slice(slc, func(i, j int) bool {
		iTime := time.Unix(0, 0)
		jTime := time.Unix(0, 0)

		if slc[i].PublicKey.ValidFor.Start != nil {
			iTime = slc[i].PublicKey.ValidFor.Start.AsTime()
		}
		if slc[j].PublicKey.ValidFor.Start != nil {
			jTime = slc[j].PublicKey.ValidFor.Start.AsTime()
		}

		return iTime.Before(jTime)
	})
}

func certificateAuthorityToProtobufCA(ca *FulcioCertificateAuthority) (*prototrustroot.CertificateAuthority, error) {
	org := ""
	if len(ca.Root.Subject.Organization) > 0 {
		org = ca.Root.Subject.Organization[0]
	}
	var allCerts []*protocommon.X509Certificate
	for _, intermed := range ca.Intermediates {
		allCerts = append(allCerts, &protocommon.X509Certificate{RawBytes: intermed.Raw})
	}
	if ca.Root == nil {
		return nil, fmt.Errorf("root certificate is nil")
	}
	allCerts = append(allCerts, &protocommon.X509Certificate{RawBytes: ca.Root.Raw})

	caProto := prototrustroot.CertificateAuthority{
		Uri: ca.URI,
		Subject: &protocommon.DistinguishedName{
			Organization: org,
			CommonName:   ca.Root.Subject.CommonName,
		},
		ValidFor: &protocommon.TimeRange{
			Start: timestamppb.New(ca.ValidityPeriodStart),
		},
		CertChain: &protocommon.X509CertificateChain{
			Certificates: allCerts,
		},
	}

	if !ca.ValidityPeriodEnd.IsZero() {
		caProto.ValidFor.End = timestamppb.New(ca.ValidityPeriodEnd)
	}

	return &caProto, nil
}

func timestampingAuthorityToProtobufCA(ca *SigstoreTimestampingAuthority) (*prototrustroot.CertificateAuthority, error) {
	org := ""
	if len(ca.Root.Subject.Organization) > 0 {
		org = ca.Root.Subject.Organization[0]
	}
	var allCerts []*protocommon.X509Certificate
	if ca.Leaf != nil {
		allCerts = append(allCerts, &protocommon.X509Certificate{RawBytes: ca.Leaf.Raw})
	}
	for _, intermed := range ca.Intermediates {
		allCerts = append(allCerts, &protocommon.X509Certificate{RawBytes: intermed.Raw})
	}
	if ca.Root == nil {
		return nil, fmt.Errorf("root certificate is nil")
	}
	allCerts = append(allCerts, &protocommon.X509Certificate{RawBytes: ca.Root.Raw})

	caProto := prototrustroot.CertificateAuthority{
		Uri: ca.URI,
		Subject: &protocommon.DistinguishedName{
			Organization: org,
			CommonName:   ca.Root.Subject.CommonName,
		},
		ValidFor: &protocommon.TimeRange{
			Start: timestamppb.New(ca.ValidityPeriodStart),
		},
		CertChain: &protocommon.X509CertificateChain{
			Certificates: allCerts,
		},
	}

	if !ca.ValidityPeriodEnd.IsZero() {
		caProto.ValidFor.End = timestamppb.New(ca.ValidityPeriodEnd)
	}

	return &caProto, nil
}

func transparencyLogToProtobufTL(tl *TransparencyLog) (*prototrustroot.TransparencyLogInstance, error) {
	hashAlgo, err := hashAlgorithmToProtobufHashAlgorithm(tl.HashFunc)
	if err != nil {
		return nil, fmt.Errorf("failed converting hash algorithm to protobuf: %w", err)
	}
	publicKey, err := publicKeyToProtobufPublicKey(tl.PublicKey, tl.ValidityPeriodStart, tl.ValidityPeriodEnd)
	if err != nil {
		return nil, fmt.Errorf("failed converting public key to protobuf: %w", err)
	}
	trProto := prototrustroot.TransparencyLogInstance{
		BaseUrl:       tl.BaseURL,
		HashAlgorithm: hashAlgo,
		PublicKey:     publicKey,
		LogId: &protocommon.LogId{
			KeyId: tl.ID,
		},
	}

	return &trProto, nil
}

func hashAlgorithmToProtobufHashAlgorithm(hashAlgorithm crypto.Hash) (protocommon.HashAlgorithm, error) {
	switch hashAlgorithm {
	case crypto.SHA256:
		return protocommon.HashAlgorithm_SHA2_256, nil
	case crypto.SHA384:
		return protocommon.HashAlgorithm_SHA2_384, nil
	case crypto.SHA512:
		return protocommon.HashAlgorithm_SHA2_512, nil
	case crypto.SHA3_256:
		return protocommon.HashAlgorithm_SHA3_256, nil
	case crypto.SHA3_384:
		return protocommon.HashAlgorithm_SHA3_384, nil
	default:
		return 0, fmt.Errorf("unsupported hash algorithm for Merkle tree: %v", hashAlgorithm)
	}
}

func publicKeyToProtobufPublicKey(publicKey crypto.PublicKey, start time.Time, end time.Time) (*protocommon.PublicKey, error) {
	pkd := protocommon.PublicKey{
		ValidFor: &protocommon.TimeRange{
			Start: timestamppb.New(start),
		},
	}

	if !end.IsZero() {
		pkd.ValidFor.End = timestamppb.New(end)
	}

	rawBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed marshalling public key: %w", err)
	}
	pkd.RawBytes = rawBytes

	switch p := publicKey.(type) {
	case *ecdsa.PublicKey:
		switch p.Curve {
		case elliptic.P256():
			pkd.KeyDetails = protocommon.PublicKeyDetails_PKIX_ECDSA_P256_SHA_256
		case elliptic.P384():
			pkd.KeyDetails = protocommon.PublicKeyDetails_PKIX_ECDSA_P384_SHA_384
		case elliptic.P521():
			pkd.KeyDetails = protocommon.PublicKeyDetails_PKIX_ECDSA_P521_SHA_512
		default:
			return nil, fmt.Errorf("unsupported curve for ecdsa key: %T", p.Curve)
		}
	case *rsa.PublicKey:
		switch p.Size() * 8 {
		case 2048:
			pkd.KeyDetails = protocommon.PublicKeyDetails_PKIX_RSA_PKCS1V15_2048_SHA256
		case 3072:
			pkd.KeyDetails = protocommon.PublicKeyDetails_PKIX_RSA_PKCS1V15_3072_SHA256
		case 4096:
			pkd.KeyDetails = protocommon.PublicKeyDetails_PKIX_RSA_PKCS1V15_4096_SHA256
		default:
			return nil, fmt.Errorf("unsupported public modulus for RSA key: %d", p.Size())
		}
	case ed25519.PublicKey:
		pkd.KeyDetails = protocommon.PublicKeyDetails_PKIX_ED25519
	default:
		return nil, fmt.Errorf("unknown public key type: %T", p)
	}

	return &pkd, nil
}
