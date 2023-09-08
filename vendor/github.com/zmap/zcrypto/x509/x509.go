// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package x509 parses X.509-encoded keys and certificates.
//
// Originally based on the go/crypto/x509 standard library,
// this package has now diverged enough that it is no longer
// updated with direct correspondence to new go releases.

package x509

import (
	// all of the hash libraries need to be imported for side-effects,
	// so that crypto.RegisterHash is called
	_ "crypto/md5"
	"crypto/sha256"
	_ "crypto/sha512"
	"io"
	"strings"

	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	_ "crypto/sha1"
	_ "crypto/sha256"
	_ "crypto/sha512"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"time"

	"github.com/zmap/zcrypto/dsa"

	"github.com/weppos/publicsuffix-go/publicsuffix"
	"github.com/zmap/zcrypto/x509/ct"
	"github.com/zmap/zcrypto/x509/pkix"
	"golang.org/x/crypto/ed25519"
)

// pkixPublicKey reflects a PKIX public key structure. See SubjectPublicKeyInfo
// in RFC 3280.
type pkixPublicKey struct {
	Algo      pkix.AlgorithmIdentifier
	BitString asn1.BitString
}

// ParsePKIXPublicKey parses a DER encoded public key. These values are
// typically found in PEM blocks with "BEGIN PUBLIC KEY".
//
// Supported key types include RSA, DSA, and ECDSA. Unknown key
// types result in an error.
//
// On success, pub will be of type *rsa.PublicKey, *dsa.PublicKey,
// or *ecdsa.PublicKey.
func ParsePKIXPublicKey(derBytes []byte) (pub interface{}, err error) {
	var pki publicKeyInfo
	if rest, err := asn1.Unmarshal(derBytes, &pki); err != nil {
		return nil, err
	} else if len(rest) != 0 {
		return nil, errors.New("x509: trailing data after ASN.1 of public-key")
	}
	algo := getPublicKeyAlgorithmFromOID(pki.Algorithm.Algorithm)
	if algo == UnknownPublicKeyAlgorithm {
		return nil, errors.New("x509: unknown public key algorithm")
	}
	return parsePublicKey(algo, &pki)
}

func marshalPublicKey(pub interface{}) (publicKeyBytes []byte, publicKeyAlgorithm pkix.AlgorithmIdentifier, err error) {
	switch pub := pub.(type) {
	case *rsa.PublicKey:
		publicKeyBytes, err = asn1.Marshal(pkcs1PublicKey{
			N: pub.N,
			E: pub.E,
		})
		if err != nil {
			return nil, pkix.AlgorithmIdentifier{}, err
		}
		publicKeyAlgorithm.Algorithm = oidPublicKeyRSA
		// This is a NULL parameters value which is required by
		// https://tools.ietf.org/html/rfc3279#section-2.3.1.
		publicKeyAlgorithm.Parameters = asn1.NullRawValue
	case *ecdsa.PublicKey:
		publicKeyBytes = elliptic.Marshal(pub.Curve, pub.X, pub.Y)
		oid, ok := oidFromNamedCurve(pub.Curve)
		if !ok {
			return nil, pkix.AlgorithmIdentifier{}, errors.New("x509: unsupported elliptic curve")
		}
		publicKeyAlgorithm.Algorithm = oidPublicKeyECDSA
		var paramBytes []byte
		paramBytes, err = asn1.Marshal(oid)
		if err != nil {
			return
		}
		publicKeyAlgorithm.Parameters.FullBytes = paramBytes
	case *AugmentedECDSA:
		return marshalPublicKey(pub.Pub)
	case ed25519.PublicKey:
		publicKeyAlgorithm.Algorithm = oidKeyEd25519
		return []byte(pub), publicKeyAlgorithm, nil
	case X25519PublicKey:
		publicKeyAlgorithm.Algorithm = oidKeyX25519
		return []byte(pub), publicKeyAlgorithm, nil
	default:
		return nil, pkix.AlgorithmIdentifier{}, errors.New("x509: only RSA, ECDSA, ed25519, or X25519 public keys supported")
	}

	return publicKeyBytes, publicKeyAlgorithm, nil
}

// MarshalPKIXPublicKey serialises a public key to DER-encoded PKIX format.
func MarshalPKIXPublicKey(pub interface{}) ([]byte, error) {
	var publicKeyBytes []byte
	var publicKeyAlgorithm pkix.AlgorithmIdentifier
	var err error

	if publicKeyBytes, publicKeyAlgorithm, err = marshalPublicKey(pub); err != nil {
		return nil, err
	}

	pkix := pkixPublicKey{
		Algo: publicKeyAlgorithm,
		BitString: asn1.BitString{
			Bytes:     publicKeyBytes,
			BitLength: 8 * len(publicKeyBytes),
		},
	}

	ret, _ := asn1.Marshal(pkix)
	return ret, nil
}

// These structures reflect the ASN.1 structure of X.509 certificates.:

type certificate struct {
	Raw                asn1.RawContent
	TBSCertificate     tbsCertificate
	SignatureAlgorithm pkix.AlgorithmIdentifier
	SignatureValue     asn1.BitString
}

type tbsCertificate struct {
	Raw                asn1.RawContent
	Version            int `asn1:"optional,explicit,default:0,tag:0"`
	SerialNumber       *big.Int
	SignatureAlgorithm pkix.AlgorithmIdentifier
	Issuer             asn1.RawValue
	Validity           validity
	Subject            asn1.RawValue
	PublicKey          publicKeyInfo
	UniqueId           asn1.BitString   `asn1:"optional,tag:1"`
	SubjectUniqueId    asn1.BitString   `asn1:"optional,tag:2"`
	Extensions         []pkix.Extension `asn1:"optional,explicit,tag:3"`
}

type dsaAlgorithmParameters struct {
	P, Q, G *big.Int
}

type dsaSignature struct {
	R, S *big.Int
}

type ecdsaSignature dsaSignature

type AugmentedECDSA struct {
	Pub *ecdsa.PublicKey
	Raw asn1.BitString
}

type validity struct {
	NotBefore, NotAfter time.Time
}

type publicKeyInfo struct {
	Raw       asn1.RawContent
	Algorithm pkix.AlgorithmIdentifier
	PublicKey asn1.BitString
}

// RFC 5280,  4.2.1.1
type authKeyId struct {
	Id []byte `asn1:"optional,tag:0"`
}

type SignatureAlgorithmOID asn1.ObjectIdentifier

type SignatureAlgorithm int

const (
	UnknownSignatureAlgorithm SignatureAlgorithm = iota
	MD2WithRSA
	MD5WithRSA
	SHA1WithRSA
	SHA256WithRSA
	SHA384WithRSA
	SHA512WithRSA
	DSAWithSHA1
	DSAWithSHA256
	ECDSAWithSHA1
	ECDSAWithSHA256
	ECDSAWithSHA384
	ECDSAWithSHA512
	SHA256WithRSAPSS
	SHA384WithRSAPSS
	SHA512WithRSAPSS
	Ed25519Sig
)

func (algo SignatureAlgorithm) isRSAPSS() bool {
	switch algo {
	case SHA256WithRSAPSS, SHA384WithRSAPSS, SHA512WithRSAPSS:
		return true
	default:
		return false
	}
}

var algoName = [...]string{
	MD2WithRSA:       "MD2-RSA",
	MD5WithRSA:       "MD5-RSA",
	SHA1WithRSA:      "SHA1-RSA",
	SHA256WithRSA:    "SHA256-RSA",
	SHA384WithRSA:    "SHA384-RSA",
	SHA512WithRSA:    "SHA512-RSA",
	SHA256WithRSAPSS: "SHA256-RSAPSS",
	SHA384WithRSAPSS: "SHA384-RSAPSS",
	SHA512WithRSAPSS: "SHA512-RSAPSS",
	DSAWithSHA1:      "DSA-SHA1",
	DSAWithSHA256:    "DSA-SHA256",
	ECDSAWithSHA1:    "ECDSA-SHA1",
	ECDSAWithSHA256:  "ECDSA-SHA256",
	ECDSAWithSHA384:  "ECDSA-SHA384",
	ECDSAWithSHA512:  "ECDSA-SHA512",
	Ed25519Sig:       "Ed25519",
}

func (algo SignatureAlgorithm) String() string {
	if 0 < algo && int(algo) < len(algoName) {
		return algoName[algo]
	}
	return strconv.Itoa(int(algo))
}

var keyAlgorithmNames = []string{
	"unknown_algorithm",
	"RSA",
	"DSA",
	"ECDSA",
	"Ed25519",
	"X25519",
}

type PublicKeyAlgorithm int

const (
	UnknownPublicKeyAlgorithm PublicKeyAlgorithm = iota
	RSA
	DSA
	ECDSA
	Ed25519
	X25519
	total_key_algorithms
)

// curve25519 package does not expose key types
type X25519PublicKey []byte

// OIDs for signature algorithms
//
// pkcs-1 OBJECT IDENTIFIER ::= {
//    iso(1) member-body(2) us(840) rsadsi(113549) pkcs(1) 1 }
//
//
// RFC 3279 2.2.1 RSA Signature Algorithms
//
// md2WithRSAEncryption OBJECT IDENTIFIER ::= { pkcs-1 2 }
//
// md5WithRSAEncryption OBJECT IDENTIFIER ::= { pkcs-1 4 }
//
// sha-1WithRSAEncryption OBJECT IDENTIFIER ::= { pkcs-1 5 }
//
// dsaWithSha1 OBJECT IDENTIFIER ::= {
//    iso(1) member-body(2) us(840) x9-57(10040) x9cm(4) 3 }
//
// RFC 3279 2.2.3 ECDSA Signature Algorithm
//
// ecdsa-with-SHA1 OBJECT IDENTIFIER ::= {
// 	  iso(1) member-body(2) us(840) ansi-x962(10045)
//    signatures(4) ecdsa-with-SHA1(1)}
//
//
// RFC 4055 5 PKCS #1 Version 1.5
//
// sha256WithRSAEncryption OBJECT IDENTIFIER ::= { pkcs-1 11 }
//
// sha384WithRSAEncryption OBJECT IDENTIFIER ::= { pkcs-1 12 }
//
// sha512WithRSAEncryption OBJECT IDENTIFIER ::= { pkcs-1 13 }
//
//
// RFC 5758 3.1 DSA Signature Algorithms
//
// dsaWithSha256 OBJECT IDENTIFIER ::= {
//    joint-iso-ccitt(2) country(16) us(840) organization(1) gov(101)
//    csor(3) algorithms(4) id-dsa-with-sha2(3) 2}
//
// RFC 5758 3.2 ECDSA Signature Algorithm
//
// ecdsa-with-SHA256 OBJECT IDENTIFIER ::= { iso(1) member-body(2)
//    us(840) ansi-X9-62(10045) signatures(4) ecdsa-with-SHA2(3) 2 }
//
// ecdsa-with-SHA384 OBJECT IDENTIFIER ::= { iso(1) member-body(2)
//    us(840) ansi-X9-62(10045) signatures(4) ecdsa-with-SHA2(3) 3 }
//
// ecdsa-with-SHA512 OBJECT IDENTIFIER ::= { iso(1) member-body(2)
//    us(840) ansi-X9-62(10045) signatures(4) ecdsa-with-SHA2(3) 4 }

var (
	oidSignatureMD2WithRSA      = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 2}
	oidSignatureMD5WithRSA      = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 4}
	oidSignatureSHA1WithRSA     = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 5}
	oidSignatureSHA256WithRSA   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 11}
	oidSignatureSHA384WithRSA   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 12}
	oidSignatureSHA512WithRSA   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 13}
	oidSignatureRSAPSS          = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 10}
	oidSignatureDSAWithSHA1     = asn1.ObjectIdentifier{1, 2, 840, 10040, 4, 3}
	oidSignatureDSAWithSHA256   = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 3, 2}
	oidSignatureECDSAWithSHA1   = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 1}
	oidSignatureECDSAWithSHA256 = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2}
	oidSignatureECDSAWithSHA384 = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 3}
	oidSignatureECDSAWithSHA512 = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 4}

	oidSHA256 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	oidSHA384 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}
	oidSHA512 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 3}

	oidMGF1 = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 8}

	// oidISOSignatureSHA1WithRSA means the same as oidSignatureSHA1WithRSA
	// but it's specified by ISO. Microsoft's makecert.exe has been known
	// to produce certificates with this OID.
	oidISOSignatureSHA1WithRSA = asn1.ObjectIdentifier{1, 3, 14, 3, 2, 29}
)

// cryptoNoDigest means that the signature algorithm does not require a hash
// digest. The distinction between cryptoNoDigest and crypto.Hash(0)
// is purely superficial. crypto.Hash(0) is used in place of a null value
// when hashing is not supported for the given algorithm (as in the case of
// MD2WithRSA below).
var cryptoNoDigest = crypto.Hash(0)

var signatureAlgorithmDetails = []struct {
	algo       SignatureAlgorithm
	oid        asn1.ObjectIdentifier
	pubKeyAlgo PublicKeyAlgorithm
	hash       crypto.Hash
}{
	{MD2WithRSA, oidSignatureMD2WithRSA, RSA, crypto.Hash(0) /* no value for MD2 */},
	{MD5WithRSA, oidSignatureMD5WithRSA, RSA, crypto.MD5},
	{SHA1WithRSA, oidSignatureSHA1WithRSA, RSA, crypto.SHA1},
	{SHA1WithRSA, oidISOSignatureSHA1WithRSA, RSA, crypto.SHA1},
	{SHA256WithRSA, oidSignatureSHA256WithRSA, RSA, crypto.SHA256},
	{SHA384WithRSA, oidSignatureSHA384WithRSA, RSA, crypto.SHA384},
	{SHA512WithRSA, oidSignatureSHA512WithRSA, RSA, crypto.SHA512},
	{SHA256WithRSAPSS, oidSignatureRSAPSS, RSA, crypto.SHA256},
	{SHA384WithRSAPSS, oidSignatureRSAPSS, RSA, crypto.SHA384},
	{SHA512WithRSAPSS, oidSignatureRSAPSS, RSA, crypto.SHA512},
	{DSAWithSHA1, oidSignatureDSAWithSHA1, DSA, crypto.SHA1},
	{DSAWithSHA256, oidSignatureDSAWithSHA256, DSA, crypto.SHA256},
	{ECDSAWithSHA1, oidSignatureECDSAWithSHA1, ECDSA, crypto.SHA1},
	{ECDSAWithSHA256, oidSignatureECDSAWithSHA256, ECDSA, crypto.SHA256},
	{ECDSAWithSHA384, oidSignatureECDSAWithSHA384, ECDSA, crypto.SHA384},
	{ECDSAWithSHA512, oidSignatureECDSAWithSHA512, ECDSA, crypto.SHA512},
	{Ed25519Sig, oidKeyEd25519, Ed25519, cryptoNoDigest},
}

// pssParameters reflects the parameters in an AlgorithmIdentifier that
// specifies RSA PSS. See https://tools.ietf.org/html/rfc3447#appendix-A.2.3
type pssParameters struct {
	// The following three fields are not marked as
	// optional because the default values specify SHA-1,
	// which is no longer suitable for use in signatures.
	Hash         pkix.AlgorithmIdentifier `asn1:"explicit,tag:0"`
	MGF          pkix.AlgorithmIdentifier `asn1:"explicit,tag:1"`
	SaltLength   int                      `asn1:"explicit,tag:2"`
	TrailerField int                      `asn1:"optional,explicit,tag:3,default:1"`
}

// rsaPSSParameters returns an asn1.RawValue suitable for use as the Parameters
// in an AlgorithmIdentifier that specifies RSA PSS.
func rsaPSSParameters(hashFunc crypto.Hash) asn1.RawValue {
	var hashOID asn1.ObjectIdentifier

	switch hashFunc {
	case crypto.SHA256:
		hashOID = oidSHA256
	case crypto.SHA384:
		hashOID = oidSHA384
	case crypto.SHA512:
		hashOID = oidSHA512
	}

	params := pssParameters{
		Hash: pkix.AlgorithmIdentifier{
			Algorithm:  hashOID,
			Parameters: asn1.NullRawValue,
		},
		MGF: pkix.AlgorithmIdentifier{
			Algorithm: oidMGF1,
		},
		SaltLength:   hashFunc.Size(),
		TrailerField: 1,
	}

	mgf1Params := pkix.AlgorithmIdentifier{
		Algorithm:  hashOID,
		Parameters: asn1.NullRawValue,
	}

	var err error
	params.MGF.Parameters.FullBytes, err = asn1.Marshal(mgf1Params)
	if err != nil {
		panic(err)
	}

	serialized, err := asn1.Marshal(params)
	if err != nil {
		panic(err)
	}

	return asn1.RawValue{FullBytes: serialized}
}

// GetSignatureAlgorithmFromAI converts asn1 AlgorithmIdentifier to SignatureAlgorithm int
func GetSignatureAlgorithmFromAI(ai pkix.AlgorithmIdentifier) SignatureAlgorithm {
	if !ai.Algorithm.Equal(oidSignatureRSAPSS) {
		for _, details := range signatureAlgorithmDetails {
			if ai.Algorithm.Equal(details.oid) {
				return details.algo
			}
		}
		return UnknownSignatureAlgorithm
	}

	// RSA PSS is special because it encodes important parameters
	// in the Parameters.

	var params pssParameters
	if _, err := asn1.Unmarshal(ai.Parameters.FullBytes, &params); err != nil {
		return UnknownSignatureAlgorithm
	}

	var mgf1HashFunc pkix.AlgorithmIdentifier
	if _, err := asn1.Unmarshal(params.MGF.Parameters.FullBytes, &mgf1HashFunc); err != nil {
		return UnknownSignatureAlgorithm
	}

	// PSS is greatly overburdened with options. This code forces
	// them into three buckets by requiring that the MGF1 hash
	// function always match the message hash function (as
	// recommended in
	// https://tools.ietf.org/html/rfc3447#section-8.1), that the
	// salt length matches the hash length, and that the trailer
	// field has the default value.
	if !bytes.Equal(params.Hash.Parameters.FullBytes, asn1.NullBytes) ||
		!params.MGF.Algorithm.Equal(oidMGF1) ||
		!mgf1HashFunc.Algorithm.Equal(params.Hash.Algorithm) ||
		!bytes.Equal(mgf1HashFunc.Parameters.FullBytes, asn1.NullBytes) ||
		params.TrailerField != 1 {
		return UnknownSignatureAlgorithm
	}

	switch {
	case params.Hash.Algorithm.Equal(oidSHA256) && params.SaltLength == 32:
		return SHA256WithRSAPSS
	case params.Hash.Algorithm.Equal(oidSHA384) && params.SaltLength == 48:
		return SHA384WithRSAPSS
	case params.Hash.Algorithm.Equal(oidSHA512) && params.SaltLength == 64:
		return SHA512WithRSAPSS
	}

	return UnknownSignatureAlgorithm
}

// RFC 3279, 2.3 Public Key Algorithms
//
// pkcs-1 OBJECT IDENTIFIER ::== { iso(1) member-body(2) us(840)
//    rsadsi(113549) pkcs(1) 1 }
//
// rsaEncryption OBJECT IDENTIFIER ::== { pkcs1-1 1 }
//
// id-dsa OBJECT IDENTIFIER ::== { iso(1) member-body(2) us(840)
//    x9-57(10040) x9cm(4) 1 }
//
// RFC 5480, 2.1.1 Unrestricted Algorithm Identifier and Parameters
//
// id-ecPublicKey OBJECT IDENTIFIER ::= {
//       iso(1) member-body(2) us(840) ansi-X9-62(10045) keyType(2) 1 }
var (
	oidPublicKeyRSA   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
	oidPublicKeyDSA   = asn1.ObjectIdentifier{1, 2, 840, 10040, 4, 1}
	oidPublicKeyECDSA = asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1}
)

func getPublicKeyAlgorithmFromOID(oid asn1.ObjectIdentifier) PublicKeyAlgorithm {
	switch {
	case oid.Equal(oidPublicKeyRSA):
		return RSA
	case oid.Equal(oidPublicKeyDSA):
		return DSA
	case oid.Equal(oidPublicKeyECDSA):
		return ECDSA
	case oid.Equal(oidKeyEd25519):
		return Ed25519
	case oid.Equal(oidKeyX25519):
		return X25519
	}
	return UnknownPublicKeyAlgorithm
}

// RFC 5480, 2.1.1.1. Named Curve
//
// secp224r1 OBJECT IDENTIFIER ::= {
//   iso(1) identified-organization(3) certicom(132) curve(0) 33 }
//
// secp256r1 OBJECT IDENTIFIER ::= {
//   iso(1) member-body(2) us(840) ansi-X9-62(10045) curves(3)
//   prime(1) 7 }
//
// secp384r1 OBJECT IDENTIFIER ::= {
//   iso(1) identified-organization(3) certicom(132) curve(0) 34 }
//
// secp521r1 OBJECT IDENTIFIER ::= {
//   iso(1) identified-organization(3) certicom(132) curve(0) 35 }
//
// NB: secp256r1 is equivalent to prime256v1
var (
	oidNamedCurveP224 = asn1.ObjectIdentifier{1, 3, 132, 0, 33}
	oidNamedCurveP256 = asn1.ObjectIdentifier{1, 2, 840, 10045, 3, 1, 7}
	oidNamedCurveP384 = asn1.ObjectIdentifier{1, 3, 132, 0, 34}
	oidNamedCurveP521 = asn1.ObjectIdentifier{1, 3, 132, 0, 35}
)

// https://datatracker.ietf.org/doc/draft-ietf-curdle-pkix/?include_text=1
// id-X25519    OBJECT IDENTIFIER ::= { 1 3 101 110 }
// id-Ed25519   OBJECT IDENTIFIER ::= { 1 3 101 112 }
var (
	oidKeyX25519  = asn1.ObjectIdentifier{1, 3, 101, 110}
	oidKeyEd25519 = asn1.ObjectIdentifier{1, 3, 101, 112}
)

func namedCurveFromOID(oid asn1.ObjectIdentifier) elliptic.Curve {
	switch {
	case oid.Equal(oidNamedCurveP224):
		return elliptic.P224()
	case oid.Equal(oidNamedCurveP256):
		return elliptic.P256()
	case oid.Equal(oidNamedCurveP384):
		return elliptic.P384()
	case oid.Equal(oidNamedCurveP521):
		return elliptic.P521()
	}
	return nil
}

func oidFromNamedCurve(curve elliptic.Curve) (asn1.ObjectIdentifier, bool) {
	switch curve {
	case elliptic.P224():
		return oidNamedCurveP224, true
	case elliptic.P256():
		return oidNamedCurveP256, true
	case elliptic.P384():
		return oidNamedCurveP384, true
	case elliptic.P521():
		return oidNamedCurveP521, true
	}

	return nil, false
}

// KeyUsage represents the set of actions that are valid for a given key. It's
// a bitmap of the KeyUsage* constants.
type KeyUsage int

const (
	KeyUsageDigitalSignature KeyUsage = 1 << iota
	KeyUsageContentCommitment
	KeyUsageKeyEncipherment
	KeyUsageDataEncipherment
	KeyUsageKeyAgreement
	KeyUsageCertSign
	KeyUsageCRLSign
	KeyUsageEncipherOnly
	KeyUsageDecipherOnly
)

// RFC 5280, 4.2.1.12  Extended Key Usage
//
// anyExtendedKeyUsage OBJECT IDENTIFIER ::= { id-ce-extKeyUsage 0 }
//
// id-kp OBJECT IDENTIFIER ::= { id-pkix 3 }
//
// id-kp-serverAuth             OBJECT IDENTIFIER ::= { id-kp 1 }
// id-kp-clientAuth             OBJECT IDENTIFIER ::= { id-kp 2 }
// id-kp-codeSigning            OBJECT IDENTIFIER ::= { id-kp 3 }
// id-kp-emailProtection        OBJECT IDENTIFIER ::= { id-kp 4 }
// id-kp-timeStamping           OBJECT IDENTIFIER ::= { id-kp 8 }
// id-kp-OCSPSigning            OBJECT IDENTIFIER ::= { id-kp 9 }
//var (
//	oidExtKeyUsageAny                        = asn1.ObjectIdentifier{2, 5, 29, 37, 0}
//	oidExtKeyUsageServerAuth                 = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 1}
//	oidExtKeyUsageClientAuth                 = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 2}
//	oidExtKeyUsageCodeSigning                = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 3}
//	oidExtKeyUsageEmailProtection            = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 4}
//	oidExtKeyUsageIPSECEndSystem             = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 5}
//	oidExtKeyUsageIPSECTunnel                = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 6}
//	oidExtKeyUsageIPSECUser                  = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 7}
//	oidExtKeyUsageTimeStamping               = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 8}
//	oidExtKeyUsageOCSPSigning                = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 9}
//	oidExtKeyUsageMicrosoftServerGatedCrypto = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 10, 3, 3}
//	oidExtKeyUsageNetscapeServerGatedCrypto  = asn1.ObjectIdentifier{2, 16, 840, 1, 113730, 4, 1}
//)

// ExtKeyUsage represents an extended set of actions that are valid for a given key.
// Each of the ExtKeyUsage* constants define a unique action.
type ExtKeyUsage int

// TODO: slight differences in case in some names. Should be easy to align with stdlib.
// leaving for now to not break compatibility

// extKeyUsageOIDs contains the mapping between an ExtKeyUsage and its OID.
var extKeyUsageOIDs = []struct {
	extKeyUsage ExtKeyUsage
	oid         asn1.ObjectIdentifier
}{
	{ExtKeyUsageAny, oidExtKeyUsageAny},
	{ExtKeyUsageServerAuth, oidExtKeyUsageServerAuth},
	{ExtKeyUsageClientAuth, oidExtKeyUsageClientAuth},
	{ExtKeyUsageCodeSigning, oidExtKeyUsageCodeSigning},
	{ExtKeyUsageEmailProtection, oidExtKeyUsageEmailProtection},
	//{ExtKeyUsageIPSECEndSystem, oidExtKeyUsageIPSECEndSystem},
	{ExtKeyUsageIpsecUser, oidExtKeyUsageIpsecEndSystem},
	//{ExtKeyUsageIPSECTunnel, oidExtKeyUsageIPSECTunnel},
	{ExtKeyUsageIpsecTunnel, oidExtKeyUsageIpsecTunnel},
	//{ExtKeyUsageIPSECUser, oidExtKeyUsageIPSECUser},
	{ExtKeyUsageIpsecUser, oidExtKeyUsageIpsecUser},
	{ExtKeyUsageTimeStamping, oidExtKeyUsageTimeStamping},
	//{ExtKeyUsageOCSPSigning, oidExtKeyUsageOCSPSigning},
	{ExtKeyUsageOcspSigning, oidExtKeyUsageOcspSigning},
	{ExtKeyUsageMicrosoftServerGatedCrypto, oidExtKeyUsageMicrosoftServerGatedCrypto},
	{ExtKeyUsageNetscapeServerGatedCrypto, oidExtKeyUsageNetscapeServerGatedCrypto},
}

// TODO: slight differences in case in some names. Should be easy to align with stdlib.
// leaving for now to not break compatibility

// extKeyUsageOIDs contains the mapping between an ExtKeyUsage and its OID.
var nativeExtKeyUsageOIDs = []struct {
	extKeyUsage ExtKeyUsage
	oid         asn1.ObjectIdentifier
}{
	{ExtKeyUsageAny, oidExtKeyUsageAny},
	{ExtKeyUsageServerAuth, oidExtKeyUsageServerAuth},
	{ExtKeyUsageClientAuth, oidExtKeyUsageClientAuth},
	{ExtKeyUsageCodeSigning, oidExtKeyUsageCodeSigning},
	{ExtKeyUsageEmailProtection, oidExtKeyUsageEmailProtection},
	{ExtKeyUsageIpsecEndSystem, oidExtKeyUsageIpsecEndSystem},
	{ExtKeyUsageIpsecTunnel, oidExtKeyUsageIpsecTunnel},
	{ExtKeyUsageIpsecUser, oidExtKeyUsageIpsecUser},
	{ExtKeyUsageTimeStamping, oidExtKeyUsageTimeStamping},
	{ExtKeyUsageOcspSigning, oidExtKeyUsageOcspSigning},
	{ExtKeyUsageMicrosoftServerGatedCrypto, oidExtKeyUsageMicrosoftServerGatedCrypto},
	{ExtKeyUsageNetscapeServerGatedCrypto, oidExtKeyUsageNetscapeServerGatedCrypto},
}

func extKeyUsageFromOID(oid asn1.ObjectIdentifier) (eku ExtKeyUsage, ok bool) {
	s := oid.String()
	eku, ok = ekuConstants[s]
	return
}

func oidFromExtKeyUsage(eku ExtKeyUsage) (oid asn1.ObjectIdentifier, ok bool) {
	for _, pair := range nativeExtKeyUsageOIDs {
		if eku == pair.extKeyUsage {
			return pair.oid, true
		}
	}
	return
}

// A Certificate represents an X.509 certificate.
type Certificate struct {
	Raw                     []byte // Complete ASN.1 DER content (certificate, signature algorithm and signature).
	RawTBSCertificate       []byte // Certificate part of raw ASN.1 DER content.
	RawSubjectPublicKeyInfo []byte // DER encoded SubjectPublicKeyInfo.
	RawSubject              []byte // DER encoded Subject
	RawIssuer               []byte // DER encoded Issuer

	Signature          []byte
	SignatureAlgorithm SignatureAlgorithm

	SelfSigned bool

	SignatureAlgorithmOID asn1.ObjectIdentifier

	PublicKeyAlgorithm PublicKeyAlgorithm
	PublicKey          interface{}

	PublicKeyAlgorithmOID asn1.ObjectIdentifier

	Version             int
	SerialNumber        *big.Int
	Issuer              pkix.Name
	Subject             pkix.Name
	NotBefore, NotAfter time.Time // Validity bounds.
	ValidityPeriod      int
	KeyUsage            KeyUsage

	IssuerUniqueId  asn1.BitString
	SubjectUniqueId asn1.BitString

	// Extensions contains raw X.509 extensions. When parsing certificates,
	// this can be used to extract non-critical extensions that are not
	// parsed by this package. When marshaling certificates, the Extensions
	// field is ignored, see ExtraExtensions.
	Extensions []pkix.Extension

	// ExtensionsMap contains raw x.509 extensions keyed by OID (in string
	// representation). It allows fast membership testing of specific OIDs. Like
	// the Extensions field this field is ignored when marshaling certificates. If
	// multiple extensions with the same OID are present only the last
	// pkix.Extension will be in this map. Consult the `Extensions` slice when it
	// is required to process all extensions including duplicates.
	ExtensionsMap map[string]pkix.Extension

	// ExtraExtensions contains extensions to be copied, raw, into any
	// marshaled certificates. Values override any extensions that would
	// otherwise be produced based on the other fields. The ExtraExtensions
	// field is not populated when parsing certificates, see Extensions.
	ExtraExtensions []pkix.Extension

	// UnhandledCriticalExtensions contains a list of extension IDs that
	// were not (fully) processed when parsing. Verify will fail if this
	// slice is non-empty, unless verification is delegated to an OS
	// library which understands all the critical extensions.
	//
	// Users can access these extensions using Extensions and can remove
	// elements from this slice if they believe that they have been
	// handled.
	UnhandledCriticalExtensions []asn1.ObjectIdentifier

	ExtKeyUsage        []ExtKeyUsage           // Sequence of extended key usages.
	UnknownExtKeyUsage []asn1.ObjectIdentifier // Encountered extended key usages unknown to this package.

	BasicConstraintsValid bool // if true then the next two fields are valid.
	IsCA                  bool

	// MaxPathLen and MaxPathLenZero indicate the presence and
	// value of the BasicConstraints' "pathLenConstraint".
	//
	// When parsing a certificate, a positive non-zero MaxPathLen
	// means that the field was specified, -1 means it was unset,
	// and MaxPathLenZero being true mean that the field was
	// explicitly set to zero. The case of MaxPathLen==0 with MaxPathLenZero==false
	// should be treated equivalent to -1 (unset).
	//
	// When generating a certificate, an unset pathLenConstraint
	// can be requested with either MaxPathLen == -1 or using the
	// zero value for both MaxPathLen and MaxPathLenZero.
	MaxPathLen int
	// MaxPathLenZero indicates that BasicConstraintsValid==true and
	// MaxPathLen==0 should be interpreted as an actual Max path length
	// of zero. Otherwise, that combination is interpreted as MaxPathLen
	// not being set.
	MaxPathLenZero bool

	SubjectKeyId   []byte
	AuthorityKeyId []byte

	// RFC 5280, 4.2.2.1 (Authority Information Access)
	OCSPServer            []string
	IssuingCertificateURL []string

	// Subject Alternate Name values
	OtherNames     []pkix.OtherName
	DNSNames       []string
	EmailAddresses []string
	DirectoryNames []pkix.Name
	EDIPartyNames  []pkix.EDIPartyName
	URIs           []string
	IPAddresses    []net.IP
	RegisteredIDs  []asn1.ObjectIdentifier

	// Issuer Alternative Name values
	IANOtherNames     []pkix.OtherName
	IANDNSNames       []string
	IANEmailAddresses []string
	IANDirectoryNames []pkix.Name
	IANEDIPartyNames  []pkix.EDIPartyName
	IANURIs           []string
	IANIPAddresses    []net.IP
	IANRegisteredIDs  []asn1.ObjectIdentifier

	// Certificate Policies values
	QualifierId          [][]asn1.ObjectIdentifier
	CPSuri               [][]string
	ExplicitTexts        [][]asn1.RawValue
	NoticeRefOrgnization [][]asn1.RawValue
	NoticeRefNumbers     [][]NoticeNumber

	ParsedExplicitTexts         [][]string
	ParsedNoticeRefOrganization [][]string

	// Name constraints
	NameConstraintsCritical bool // if true then the name constraints are marked critical.
	PermittedDNSNames       []GeneralSubtreeString
	ExcludedDNSNames        []GeneralSubtreeString
	PermittedEmailAddresses []GeneralSubtreeString
	ExcludedEmailAddresses  []GeneralSubtreeString
	PermittedURIs           []GeneralSubtreeString
	ExcludedURIs            []GeneralSubtreeString
	PermittedIPAddresses    []GeneralSubtreeIP
	ExcludedIPAddresses     []GeneralSubtreeIP
	PermittedDirectoryNames []GeneralSubtreeName
	ExcludedDirectoryNames  []GeneralSubtreeName
	PermittedEdiPartyNames  []GeneralSubtreeEdi
	ExcludedEdiPartyNames   []GeneralSubtreeEdi
	PermittedRegisteredIDs  []GeneralSubtreeOid
	ExcludedRegisteredIDs   []GeneralSubtreeOid
	PermittedX400Addresses  []GeneralSubtreeRaw
	ExcludedX400Addresses   []GeneralSubtreeRaw

	// CRL Distribution Points
	CRLDistributionPoints []string

	PolicyIdentifiers []asn1.ObjectIdentifier
	ValidationLevel   CertValidationLevel

	// Fingerprints
	FingerprintMD5    CertificateFingerprint
	FingerprintSHA1   CertificateFingerprint
	FingerprintSHA256 CertificateFingerprint
	FingerprintNoCT   CertificateFingerprint

	// SPKI
	SPKIFingerprint           CertificateFingerprint
	SPKISubjectFingerprint    CertificateFingerprint
	TBSCertificateFingerprint CertificateFingerprint

	IsPrecert bool

	// Internal
	validSignature bool

	// CT
	SignedCertificateTimestampList []*ct.SignedCertificateTimestamp

	// QWACS
	CABFOrganizationIdentifier *CABFOrganizationIdentifier
	QCStatements               *QCStatements

	// Used to speed up the zlint checks. Populated by the GetParsedDNSNames method.
	parsedDNSNames []ParsedDomainName
	// Used to speed up the zlint checks. Populated by the GetParsedCommonName method
	parsedCommonName *ParsedDomainName

	// CAB Forum Tor Service Descriptor Hash Extensions (see EV Guidelines
	// Appendix F)
	TorServiceDescriptors []*TorServiceDescriptorHash
}

// ParsedDomainName is a structure holding a parsed domain name (CommonName or
// DNS SAN) and a parsing error.
type ParsedDomainName struct {
	DomainString string
	ParsedDomain *publicsuffix.DomainName
	ParseError   error
}

// GetParsedDNSNames returns a list of parsed SAN DNS names. It is used to cache the parsing result and
// speed up zlint linters. If invalidateCache is true, then the cache is repopulated with current list of string from
// Certificate.DNSNames. This parameter should always be false, unless the Certificate.DNSNames have been modified
// after calling GetParsedDNSNames the previous time.
func (c *Certificate) GetParsedDNSNames(invalidateCache bool) []ParsedDomainName {
	if c.parsedDNSNames != nil && !invalidateCache {
		return c.parsedDNSNames
	}
	c.parsedDNSNames = make([]ParsedDomainName, len(c.DNSNames))

	for i := range c.DNSNames {
		var parsedDomain, parseError = publicsuffix.ParseFromListWithOptions(publicsuffix.DefaultList,
			c.DNSNames[i],
			&publicsuffix.FindOptions{IgnorePrivate: true, DefaultRule: publicsuffix.DefaultRule})

		c.parsedDNSNames[i].DomainString = c.DNSNames[i]
		c.parsedDNSNames[i].ParsedDomain = parsedDomain
		c.parsedDNSNames[i].ParseError = parseError
	}

	return c.parsedDNSNames
}

// GetParsedCommonName returns parsed subject CommonName. It is used to cache the parsing result and
// speed up zlint linters. If invalidateCache is true, then the cache is repopulated with current subject CommonName.
// This parameter should always be false, unless the Certificate.Subject.CommonName have been modified
// after calling GetParsedSubjectCommonName the previous time.
func (c *Certificate) GetParsedSubjectCommonName(invalidateCache bool) ParsedDomainName {
	if c.parsedCommonName != nil && !invalidateCache {
		return *c.parsedCommonName
	}

	var parsedDomain, parseError = publicsuffix.ParseFromListWithOptions(publicsuffix.DefaultList,
		c.Subject.CommonName,
		&publicsuffix.FindOptions{IgnorePrivate: true, DefaultRule: publicsuffix.DefaultRule})

	c.parsedCommonName = &ParsedDomainName{
		DomainString: c.Subject.CommonName,
		ParsedDomain: parsedDomain,
		ParseError:   parseError,
	}

	return *c.parsedCommonName
}

// ErrUnsupportedAlgorithm results from attempting to perform an operation that
// involves algorithms that are not currently implemented.
var ErrUnsupportedAlgorithm = errors.New("x509: cannot verify signature: algorithm unimplemented")

// An InsecureAlgorithmError
type InsecureAlgorithmError SignatureAlgorithm

func (e InsecureAlgorithmError) Error() string {
	return fmt.Sprintf("x509: cannot verify signature: insecure algorithm %v", SignatureAlgorithm(e))
}

// ConstraintViolationError results when a requested usage is not permitted by
// a certificate. For example: checking a signature when the public key isn't a
// certificate signing key.
type ConstraintViolationError struct{}

func (ConstraintViolationError) Error() string {
	return "x509: invalid signature: parent certificate cannot sign this kind of certificate"
}

func (c *Certificate) Equal(other *Certificate) bool {
	return bytes.Equal(c.Raw, other.Raw)
}

func (c *Certificate) hasSANExtension() bool {
	return oidInExtensions(oidExtensionSubjectAltName, c.Extensions)
}

// Entrust have a broken root certificate (CN=Entrust.net Certification
// Authority (2048)) which isn't marked as a CA certificate and is thus invalid
// according to PKIX.
// We recognise this certificate by its SubjectPublicKeyInfo and exempt it
// from the Basic Constraints requirement.
// See http://www.entrust.net/knowledge-base/technote.cfm?tn=7869
//
// TODO(agl): remove this hack once their reissued root is sufficiently
// widespread.
var entrustBrokenSPKI = []byte{
	0x30, 0x82, 0x01, 0x22, 0x30, 0x0d, 0x06, 0x09,
	0x2a, 0x86, 0x48, 0x86, 0xf7, 0x0d, 0x01, 0x01,
	0x01, 0x05, 0x00, 0x03, 0x82, 0x01, 0x0f, 0x00,
	0x30, 0x82, 0x01, 0x0a, 0x02, 0x82, 0x01, 0x01,
	0x00, 0x97, 0xa3, 0x2d, 0x3c, 0x9e, 0xde, 0x05,
	0xda, 0x13, 0xc2, 0x11, 0x8d, 0x9d, 0x8e, 0xe3,
	0x7f, 0xc7, 0x4b, 0x7e, 0x5a, 0x9f, 0xb3, 0xff,
	0x62, 0xab, 0x73, 0xc8, 0x28, 0x6b, 0xba, 0x10,
	0x64, 0x82, 0x87, 0x13, 0xcd, 0x57, 0x18, 0xff,
	0x28, 0xce, 0xc0, 0xe6, 0x0e, 0x06, 0x91, 0x50,
	0x29, 0x83, 0xd1, 0xf2, 0xc3, 0x2a, 0xdb, 0xd8,
	0xdb, 0x4e, 0x04, 0xcc, 0x00, 0xeb, 0x8b, 0xb6,
	0x96, 0xdc, 0xbc, 0xaa, 0xfa, 0x52, 0x77, 0x04,
	0xc1, 0xdb, 0x19, 0xe4, 0xae, 0x9c, 0xfd, 0x3c,
	0x8b, 0x03, 0xef, 0x4d, 0xbc, 0x1a, 0x03, 0x65,
	0xf9, 0xc1, 0xb1, 0x3f, 0x72, 0x86, 0xf2, 0x38,
	0xaa, 0x19, 0xae, 0x10, 0x88, 0x78, 0x28, 0xda,
	0x75, 0xc3, 0x3d, 0x02, 0x82, 0x02, 0x9c, 0xb9,
	0xc1, 0x65, 0x77, 0x76, 0x24, 0x4c, 0x98, 0xf7,
	0x6d, 0x31, 0x38, 0xfb, 0xdb, 0xfe, 0xdb, 0x37,
	0x02, 0x76, 0xa1, 0x18, 0x97, 0xa6, 0xcc, 0xde,
	0x20, 0x09, 0x49, 0x36, 0x24, 0x69, 0x42, 0xf6,
	0xe4, 0x37, 0x62, 0xf1, 0x59, 0x6d, 0xa9, 0x3c,
	0xed, 0x34, 0x9c, 0xa3, 0x8e, 0xdb, 0xdc, 0x3a,
	0xd7, 0xf7, 0x0a, 0x6f, 0xef, 0x2e, 0xd8, 0xd5,
	0x93, 0x5a, 0x7a, 0xed, 0x08, 0x49, 0x68, 0xe2,
	0x41, 0xe3, 0x5a, 0x90, 0xc1, 0x86, 0x55, 0xfc,
	0x51, 0x43, 0x9d, 0xe0, 0xb2, 0xc4, 0x67, 0xb4,
	0xcb, 0x32, 0x31, 0x25, 0xf0, 0x54, 0x9f, 0x4b,
	0xd1, 0x6f, 0xdb, 0xd4, 0xdd, 0xfc, 0xaf, 0x5e,
	0x6c, 0x78, 0x90, 0x95, 0xde, 0xca, 0x3a, 0x48,
	0xb9, 0x79, 0x3c, 0x9b, 0x19, 0xd6, 0x75, 0x05,
	0xa0, 0xf9, 0x88, 0xd7, 0xc1, 0xe8, 0xa5, 0x09,
	0xe4, 0x1a, 0x15, 0xdc, 0x87, 0x23, 0xaa, 0xb2,
	0x75, 0x8c, 0x63, 0x25, 0x87, 0xd8, 0xf8, 0x3d,
	0xa6, 0xc2, 0xcc, 0x66, 0xff, 0xa5, 0x66, 0x68,
	0x55, 0x02, 0x03, 0x01, 0x00, 0x01,
}

// CheckSignatureFrom verifies that the signature on c is a valid signature
// from parent.
func (c *Certificate) CheckSignatureFrom(parent *Certificate) (err error) {
	// RFC 5280, 4.2.1.9:
	// "If the basic constraints extension is not present in a version 3
	// certificate, or the extension is present but the cA boolean is not
	// asserted, then the certified public key MUST NOT be used to verify
	// certificate signatures."
	// (except for Entrust, see comment above entrustBrokenSPKI)
	if (parent.Version == 3 && !parent.BasicConstraintsValid ||
		parent.BasicConstraintsValid && !parent.IsCA) &&
		!bytes.Equal(c.RawSubjectPublicKeyInfo, entrustBrokenSPKI) {
		return ConstraintViolationError{}
	}

	if parent.KeyUsage != 0 && parent.KeyUsage&KeyUsageCertSign == 0 {
		return ConstraintViolationError{}
	}

	if parent.PublicKeyAlgorithm == UnknownPublicKeyAlgorithm {
		return ErrUnsupportedAlgorithm
	}

	// TODO(agl): don't ignore the path length constraint.

	if !bytes.Equal(parent.RawSubject, c.RawIssuer) {
		return errors.New("Mis-match issuer/subject")
	}

	return parent.CheckSignature(c.SignatureAlgorithm, c.RawTBSCertificate, c.Signature)
}

func CheckSignatureFromKey(publicKey interface{}, algo SignatureAlgorithm, signed, signature []byte) (err error) {
	var hashType crypto.Hash

	switch algo {
	// NOTE: exception to stdlib, allow MD5 algorithm
	case MD5WithRSA:
		hashType = crypto.MD5
	case SHA1WithRSA, DSAWithSHA1, ECDSAWithSHA1:
		hashType = crypto.SHA1
	case SHA256WithRSA, SHA256WithRSAPSS, DSAWithSHA256, ECDSAWithSHA256:
		hashType = crypto.SHA256
	case SHA384WithRSA, SHA384WithRSAPSS, ECDSAWithSHA384:
		hashType = crypto.SHA384
	case SHA512WithRSA, SHA512WithRSAPSS, ECDSAWithSHA512:
		hashType = crypto.SHA512
	//case MD2WithRSA, MD5WithRSA:
	case MD2WithRSA:
		return InsecureAlgorithmError(algo)
	case Ed25519Sig:
		hashType = 0
	default:
		return ErrUnsupportedAlgorithm
	}

	if hashType != 0 && !hashType.Available() {
		return ErrUnsupportedAlgorithm
	}
	digest := hash(hashType, signed)

	switch pub := publicKey.(type) {
	case *rsa.PublicKey:
		if algo.isRSAPSS() {
			return rsa.VerifyPSS(pub, hashType, digest, signature, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash})
		} else {
			return rsa.VerifyPKCS1v15(pub, hashType, digest, signature)
		}
	case *dsa.PublicKey:
		dsaSig := new(dsaSignature)
		if rest, err := asn1.Unmarshal(signature, dsaSig); err != nil {
			return err
		} else if len(rest) != 0 {
			return errors.New("x509: trailing data after DSA signature")
		}
		if dsaSig.R.Sign() <= 0 || dsaSig.S.Sign() <= 0 {
			return errors.New("x509: DSA signature contained zero or negative values")
		}
		if !dsa.Verify(pub, digest, dsaSig.R, dsaSig.S) {
			return errors.New("x509: DSA verification failure")
		}
		return
	case *ecdsa.PublicKey:
		ecdsaSig := new(ecdsaSignature)
		if rest, err := asn1.Unmarshal(signature, ecdsaSig); err != nil {
			return err
		} else if len(rest) != 0 {
			return errors.New("x509: trailing data after ECDSA signature")
		}
		if ecdsaSig.R.Sign() <= 0 || ecdsaSig.S.Sign() <= 0 {
			return errors.New("x509: ECDSA signature contained zero or negative values")
		}
		if !ecdsa.Verify(pub, digest, ecdsaSig.R, ecdsaSig.S) {
			return errors.New("x509: ECDSA verification failure")
		}
		return
	case *AugmentedECDSA:
		ecdsaSig := new(ecdsaSignature)
		if _, err := asn1.Unmarshal(signature, ecdsaSig); err != nil {
			return err
		}
		if ecdsaSig.R.Sign() <= 0 || ecdsaSig.S.Sign() <= 0 {
			return errors.New("x509: ECDSA signature contained zero or negative values")
		}
		if !ecdsa.Verify(pub.Pub, digest, ecdsaSig.R, ecdsaSig.S) {
			return errors.New("x509: ECDSA verification failure")
		}
		return
	case ed25519.PublicKey:
		if !ed25519.Verify(pub, digest, signature) {
			return errors.New("x509: Ed25519 verification failure")
		}
		return
	}
	return ErrUnsupportedAlgorithm
}

// CheckSignature verifies that signature is a valid signature over signed from
// c's public key.
func (c *Certificate) CheckSignature(algo SignatureAlgorithm, signed, signature []byte) (err error) {
	return CheckSignatureFromKey(c.PublicKey, algo, signed, signature)
}

// CheckCRLSignature checks that the signature in crl is from c.
func (c *Certificate) CheckCRLSignature(crl *pkix.CertificateList) error {
	algo := GetSignatureAlgorithmFromAI(crl.SignatureAlgorithm)
	return c.CheckSignature(algo, crl.TBSCertList.Raw, crl.SignatureValue.RightAlign())
}

// UnhandledCriticalExtension results when the certificate contains an
// unimplemented X.509 extension marked as critical.
type UnhandledCriticalExtension struct {
	oid     asn1.ObjectIdentifier
	message string
}

func (h UnhandledCriticalExtension) Error() string {
	return fmt.Sprintf("x509: unhandled critical extension: %s | %s", h.oid, h.message)
}

// TimeInValidityPeriod returns true if NotBefore < t < NotAfter
func (c *Certificate) TimeInValidityPeriod(t time.Time) bool {
	return c.NotBefore.Before(t) && c.NotAfter.After(t)
}

// RFC 5280 4.2.1.4
type policyInformation struct {
	Policy     asn1.ObjectIdentifier
	Qualifiers []policyQualifierInfo `asn1:"optional"`
}

type policyQualifierInfo struct {
	PolicyQualifierId asn1.ObjectIdentifier
	Qualifier         asn1.RawValue
}

type userNotice struct {
	NoticeRef    noticeReference `asn1:"optional"`
	ExplicitText asn1.RawValue   `asn1:"optional"`
}

type noticeReference struct {
	Organization  asn1.RawValue
	NoticeNumbers []int
}

type NoticeNumber []int

type generalSubtree struct {
	Value asn1.RawValue `asn1:"optional"`
	Min   int           `asn1:"tag:0,default:0,optional"`
	Max   int           `asn1:"tag:1,optional"`
}

type GeneralSubtreeString struct {
	Data string
	Max  int
	Min  int
}

type GeneralSubtreeIP struct {
	Data net.IPNet
	Max  int
	Min  int
}

type GeneralSubtreeName struct {
	Data pkix.Name
	Max  int
	Min  int
}

type GeneralSubtreeEdi struct {
	Data pkix.EDIPartyName
	Max  int
	Min  int
}

type GeneralSubtreeOid struct {
	Data asn1.ObjectIdentifier
	Max  int
	Min  int
}

type GeneralSubtreeRaw struct {
	Data asn1.RawValue
	Max  int
	Min  int
}

type basicConstraints struct {
	IsCA       bool `asn1:"optional"`
	MaxPathLen int  `asn1:"optional,default:-1"`
}

// RFC 5280, 4.2.1.10
type nameConstraints struct {
	Permitted []generalSubtree `asn1:"optional,tag:0"`
	Excluded  []generalSubtree `asn1:"optional,tag:1"`
}

// RFC 5280, 4.2.2.1
type authorityInfoAccess struct {
	Method   asn1.ObjectIdentifier
	Location asn1.RawValue
}

// RFC 5280, 4.2.1.14
type distributionPoint struct {
	DistributionPoint distributionPointName `asn1:"optional,tag:0"`
	Reason            asn1.BitString        `asn1:"optional,tag:1"`
	CRLIssuer         asn1.RawValue         `asn1:"optional,tag:2"`
}

type distributionPointName struct {
	FullName     asn1.RawValue    `asn1:"optional,tag:0"`
	RelativeName pkix.RDNSequence `asn1:"optional,tag:1"`
}

func maxValidationLevel(a, b CertValidationLevel) CertValidationLevel {
	if a > b {
		return a
	}
	return b
}

func hash(hashFunc crypto.Hash, raw []byte) []byte {
	digest := raw
	if hashFunc != 0 {
		h := hashFunc.New()
		h.Write(raw)
		digest = h.Sum(nil)
	}
	return digest
}

func getMaxCertValidationLevel(oids []asn1.ObjectIdentifier) CertValidationLevel {
	maxOID := UnknownValidationLevel
	for _, oid := range oids {
		if _, ok := ExtendedValidationOIDs[oid.String()]; ok {
			return EV
		} else if _, ok := OrganizationValidationOIDs[oid.String()]; ok {
			maxOID = maxValidationLevel(maxOID, OV)
		} else if _, ok := DomainValidationOIDs[oid.String()]; ok {
			maxOID = maxValidationLevel(maxOID, DV)
		}
	}
	return maxOID
}

func parsePublicKey(algo PublicKeyAlgorithm, keyData *publicKeyInfo) (interface{}, error) {
	asn1Data := keyData.PublicKey.RightAlign()
	switch algo {
	case RSA:

		// TODO: disabled since current behaviour does not expect it. Should be enabled though
		// RSA public keys must have a NULL in the parameters
		// (https://tools.ietf.org/html/rfc3279#section-2.3.1).
		//if !bytes.Equal(keyData.Algorithm.Parameters.FullBytes, asn1.NullBytes) {
		//	return nil, errors.New("x509: RSA key missing NULL parameters")
		//}

		p := new(pkcs1PublicKey)
		rest, err := asn1.Unmarshal(asn1Data, p)
		if err != nil {
			return nil, err
		}
		if len(rest) != 0 {
			return nil, errors.New("x509: trailing data after RSA public key")
		}

		if p.N.Sign() <= 0 {
			return nil, errors.New("x509: RSA modulus is not a positive number")
		}
		if p.E <= 0 {
			return nil, errors.New("x509: RSA public exponent is not a positive number")
		}

		pub := &rsa.PublicKey{
			E: p.E,
			N: p.N,
		}
		return pub, nil
	case DSA:
		var p *big.Int
		rest, err := asn1.Unmarshal(asn1Data, &p)
		if err != nil {
			return nil, err
		}
		if len(rest) != 0 {
			return nil, errors.New("x509: trailing data after DSA public key")
		}
		paramsData := keyData.Algorithm.Parameters.FullBytes
		params := new(dsaAlgorithmParameters)
		rest, err = asn1.Unmarshal(paramsData, params)
		if err != nil {
			return nil, err
		}
		if len(rest) != 0 {
			return nil, errors.New("x509: trailing data after DSA parameters")
		}
		if p.Sign() <= 0 || params.P.Sign() <= 0 || params.Q.Sign() <= 0 || params.G.Sign() <= 0 {
			return nil, errors.New("x509: zero or negative DSA parameter")
		}
		pub := &dsa.PublicKey{
			Parameters: dsa.Parameters{
				P: params.P,
				Q: params.Q,
				G: params.G,
			},
			Y: p,
		}
		return pub, nil
	case ECDSA:
		paramsData := keyData.Algorithm.Parameters.FullBytes
		namedCurveOID := new(asn1.ObjectIdentifier)
		rest, err := asn1.Unmarshal(paramsData, namedCurveOID)
		if err != nil {
			return nil, err
		}
		if len(rest) != 0 {
			return nil, errors.New("x509: trailing data after ECDSA parameters")
		}
		namedCurve := namedCurveFromOID(*namedCurveOID)
		if namedCurve == nil {
			return nil, errors.New("x509: unsupported elliptic curve")
		}
		x, y := elliptic.Unmarshal(namedCurve, asn1Data)
		if x == nil {
			return nil, errors.New("x509: failed to unmarshal elliptic curve point")
		}
		key := &ecdsa.PublicKey{
			Curve: namedCurve,
			X:     x,
			Y:     y,
		}

		pub := &AugmentedECDSA{
			Pub: key,
			Raw: keyData.PublicKey,
		}
		return pub, nil
	case Ed25519:
		p := ed25519.PublicKey(asn1Data)
		if len(p) > ed25519.PublicKeySize {
			return nil, errors.New("x509: trailing data after Ed25519 data")
		}
		return p, nil
	case X25519:
		p := X25519PublicKey(asn1Data)
		if len(p) > 32 {
			return nil, errors.New("x509: trailing data after X25519 public key")
		}
		return p, nil
	default:
		return nil, nil
	}
}

func parseSANExtension(value []byte) (dnsNames, emailAddresses []string, ipAddresses []net.IP, err error) {
	// RFC 5280, 4.2.1.6

	// SubjectAltName ::= GeneralNames
	//
	// GeneralNames ::= SEQUENCE SIZE (1..MAX) OF GeneralName
	//
	// GeneralName ::= CHOICE {
	//      otherName                       [0]     OtherName,
	//      rfc822Name                      [1]     IA5String,
	//      dNSName                         [2]     IA5String,
	//      x400Address                     [3]     ORAddress,
	//      directoryName                   [4]     Name,
	//      ediPartyName                    [5]     EDIPartyName,
	//      uniformResourceIdentifier       [6]     IA5String,
	//      iPAddress                       [7]     OCTET STRING,
	//      registeredID                    [8]     OBJECT IDENTIFIER }
	var seq asn1.RawValue
	var rest []byte
	if rest, err = asn1.Unmarshal(value, &seq); err != nil {
		return
	} else if len(rest) != 0 {
		err = errors.New("x509: trailing data after X.509 extension")
		return
	}
	if !seq.IsCompound || seq.Tag != 16 || seq.Class != 0 {
		err = asn1.StructuralError{Msg: "bad SAN sequence"}
		return
	}

	rest = seq.Bytes
	for len(rest) > 0 {
		var v asn1.RawValue
		rest, err = asn1.Unmarshal(rest, &v)
		if err != nil {
			return
		}
		switch v.Tag {
		case 1:
			emailAddresses = append(emailAddresses, string(v.Bytes))
		case 2:
			dnsNames = append(dnsNames, string(v.Bytes))
		case 7:
			switch len(v.Bytes) {
			case net.IPv4len, net.IPv6len:
				ipAddresses = append(ipAddresses, v.Bytes)
			default:
				err = errors.New("x509: certificate contained IP address of length " + strconv.Itoa(len(v.Bytes)))
				return
			}
		}
	}

	return
}

func parseGeneralNames(value []byte) (otherNames []pkix.OtherName, dnsNames, emailAddresses, URIs []string, directoryNames []pkix.Name, ediPartyNames []pkix.EDIPartyName, ipAddresses []net.IP, registeredIDs []asn1.ObjectIdentifier, err error) {
	// RFC 5280, 4.2.1.6

	// SubjectAltName ::= GeneralNames
	//
	// GeneralNames ::= SEQUENCE SIZE (1..MAX) OF GeneralName
	//
	// GeneralName ::= CHOICE {
	//      otherName                       [0]     OtherName,
	//      rfc822Name                      [1]     IA5String,
	//      dNSName                         [2]     IA5String,
	//      x400Address                     [3]     ORAddress,
	//      directoryName                   [4]     Name,
	//      ediPartyName                    [5]     EDIPartyName,
	//      uniformResourceIdentifier       [6]     IA5String,
	//      iPAddress                       [7]     OCTET STRING,
	//      registeredID                    [8]     OBJECT IDENTIFIER }
	var seq asn1.RawValue
	if _, err = asn1.Unmarshal(value, &seq); err != nil {
		return
	}
	if !seq.IsCompound || seq.Tag != 16 || seq.Class != 0 {
		err = asn1.StructuralError{Msg: "bad SAN sequence"}
		return
	}

	rest := seq.Bytes
	for len(rest) > 0 {
		var v asn1.RawValue
		rest, err = asn1.Unmarshal(rest, &v)
		if err != nil {
			return
		}
		switch v.Tag {
		case 0:
			var oName pkix.OtherName
			_, err = asn1.UnmarshalWithParams(v.FullBytes, &oName, "tag:0")
			if err != nil {
				return
			}
			otherNames = append(otherNames, oName)
		case 1:
			emailAddresses = append(emailAddresses, string(v.Bytes))
		case 2:
			dnsNames = append(dnsNames, string(v.Bytes))
		case 4:
			var rdn pkix.RDNSequence
			_, err = asn1.Unmarshal(v.Bytes, &rdn)
			if err != nil {
				return
			}
			var dir pkix.Name
			dir.FillFromRDNSequence(&rdn)
			directoryNames = append(directoryNames, dir)
		case 5:
			var ediName pkix.EDIPartyName
			_, err = asn1.UnmarshalWithParams(v.FullBytes, &ediName, "tag:5")
			if err != nil {
				return
			}
			ediPartyNames = append(ediPartyNames, ediName)
		case 6:
			URIs = append(URIs, string(v.Bytes))
		case 7:
			switch len(v.Bytes) {
			case net.IPv4len, net.IPv6len:
				ipAddresses = append(ipAddresses, v.Bytes)
			default:
				err = errors.New("x509: certificate contained IP address of length " + strconv.Itoa(len(v.Bytes)))
				return
			}
		case 8:
			var id asn1.ObjectIdentifier
			_, err = asn1.UnmarshalWithParams(v.FullBytes, &id, "tag:8")
			if err != nil {
				return
			}
			registeredIDs = append(registeredIDs, id)
		}
	}

	return
}

//TODO
func parseCertificate(in *certificate) (*Certificate, error) {
	out := new(Certificate)
	out.Raw = in.Raw
	out.RawTBSCertificate = in.TBSCertificate.Raw
	out.RawSubjectPublicKeyInfo = in.TBSCertificate.PublicKey.Raw
	out.RawSubject = in.TBSCertificate.Subject.FullBytes
	out.RawIssuer = in.TBSCertificate.Issuer.FullBytes

	// Fingerprints
	out.FingerprintMD5 = MD5Fingerprint(in.Raw)
	out.FingerprintSHA1 = SHA1Fingerprint(in.Raw)
	out.FingerprintSHA256 = SHA256Fingerprint(in.Raw)
	out.SPKIFingerprint = SHA256Fingerprint(in.TBSCertificate.PublicKey.Raw)
	out.TBSCertificateFingerprint = SHA256Fingerprint(in.TBSCertificate.Raw)

	tbs := in.TBSCertificate
	originalExtensions := in.TBSCertificate.Extensions

	// Blow away the raw data since it also includes CT data
	tbs.Raw = nil

	// remove the CT extensions
	extensions := make([]pkix.Extension, 0, len(originalExtensions))
	for _, extension := range originalExtensions {
		if extension.Id.Equal(oidExtensionCTPrecertificatePoison) {
			continue
		}
		if extension.Id.Equal(oidExtensionSignedCertificateTimestampList) {
			continue
		}
		extensions = append(extensions, extension)
	}

	tbs.Extensions = extensions

	tbsbytes, err := asn1.Marshal(tbs)
	if err != nil {
		return nil, err
	}
	if tbsbytes == nil {
		return nil, asn1.SyntaxError{Msg: "Trailing data"}
	}
	out.FingerprintNoCT = SHA256Fingerprint(tbsbytes[:])

	// Hash both SPKI and Subject to create a fingerprint that we can use to describe a CA
	hasher := sha256.New()
	hasher.Write(in.TBSCertificate.PublicKey.Raw)
	hasher.Write(in.TBSCertificate.Subject.FullBytes)
	out.SPKISubjectFingerprint = hasher.Sum(nil)

	out.Signature = in.SignatureValue.RightAlign()
	out.SignatureAlgorithm =
		GetSignatureAlgorithmFromAI(in.TBSCertificate.SignatureAlgorithm)

	out.SignatureAlgorithmOID = in.TBSCertificate.SignatureAlgorithm.Algorithm

	out.PublicKeyAlgorithm =
		getPublicKeyAlgorithmFromOID(in.TBSCertificate.PublicKey.Algorithm.Algorithm)
	out.PublicKey, err = parsePublicKey(out.PublicKeyAlgorithm, &in.TBSCertificate.PublicKey)
	if err != nil {
		return nil, err
	}

	out.PublicKeyAlgorithmOID = in.TBSCertificate.PublicKey.Algorithm.Algorithm
	out.Version = in.TBSCertificate.Version + 1
	out.SerialNumber = in.TBSCertificate.SerialNumber

	var issuer, subject pkix.RDNSequence
	if _, err := asn1.Unmarshal(in.TBSCertificate.Subject.FullBytes, &subject); err != nil {
		return nil, err
	}
	if _, err := asn1.Unmarshal(in.TBSCertificate.Issuer.FullBytes, &issuer); err != nil {
		return nil, err
	}

	out.Issuer.FillFromRDNSequence(&issuer)
	out.Subject.FillFromRDNSequence(&subject)

	// Check if self-signed
	if bytes.Equal(out.RawSubject, out.RawIssuer) {
		// Possibly self-signed, check the signature against itself.
		if err := out.CheckSignature(out.SignatureAlgorithm, out.RawTBSCertificate, out.Signature); err == nil {
			out.SelfSigned = true
		}
	}

	out.NotBefore = in.TBSCertificate.Validity.NotBefore
	out.NotAfter = in.TBSCertificate.Validity.NotAfter

	out.ValidityPeriod = int(out.NotAfter.Sub(out.NotBefore).Seconds())

	out.IssuerUniqueId = in.TBSCertificate.UniqueId
	out.SubjectUniqueId = in.TBSCertificate.SubjectUniqueId

	out.ExtensionsMap = make(map[string]pkix.Extension, len(in.TBSCertificate.Extensions))
	for _, e := range in.TBSCertificate.Extensions {
		out.Extensions = append(out.Extensions, e)
		out.ExtensionsMap[e.Id.String()] = e

		if len(e.Id) == 4 && e.Id[0] == 2 && e.Id[1] == 5 && e.Id[2] == 29 {
			switch e.Id[3] {
			case 15:
				// RFC 5280, 4.2.1.3
				var usageBits asn1.BitString
				_, err := asn1.Unmarshal(e.Value, &usageBits)

				if err == nil {
					var usage int
					for i := 0; i < 9; i++ {
						if usageBits.At(i) != 0 {
							usage |= 1 << uint(i)
						}
					}
					out.KeyUsage = KeyUsage(usage)
					continue
				}
			case 19:
				// RFC 5280, 4.2.1.9
				var constraints basicConstraints
				_, err := asn1.Unmarshal(e.Value, &constraints)

				if err == nil {
					out.BasicConstraintsValid = true
					out.IsCA = constraints.IsCA
					out.MaxPathLen = constraints.MaxPathLen
					out.MaxPathLenZero = out.MaxPathLen == 0
					continue
				}
			case 17:
				out.OtherNames, out.DNSNames, out.EmailAddresses, out.URIs, out.DirectoryNames, out.EDIPartyNames, out.IPAddresses, out.RegisteredIDs, err = parseGeneralNames(e.Value)
				if err != nil {
					return nil, err
				}

				if len(out.DNSNames) > 0 || len(out.EmailAddresses) > 0 || len(out.IPAddresses) > 0 {
					continue
				}
				// If we didn't parse any of the names then we
				// fall through to the critical check below.
			case 18:
				out.IANOtherNames, out.IANDNSNames, out.IANEmailAddresses, out.IANURIs, out.IANDirectoryNames, out.IANEDIPartyNames, out.IANIPAddresses, out.IANRegisteredIDs, err = parseGeneralNames(e.Value)
				if err != nil {
					return nil, err
				}

				if len(out.IANDNSNames) > 0 || len(out.IANEmailAddresses) > 0 || len(out.IANIPAddresses) > 0 {
					continue
				}
			case 30:
				// RFC 5280, 4.2.1.10

				// NameConstraints ::= SEQUENCE {
				//      permittedSubtrees       [0]     GeneralSubtrees OPTIONAL,
				//      excludedSubtrees        [1]     GeneralSubtrees OPTIONAL }
				//
				// GeneralSubtrees ::= SEQUENCE SIZE (1..MAX) OF GeneralSubtree
				//
				// GeneralSubtree ::= SEQUENCE {
				//      base                    GeneralName,
				//      Min         [0]     BaseDistance DEFAULT 0,
				//      Max         [1]     BaseDistance OPTIONAL }
				//
				// BaseDistance ::= INTEGER (0..MAX)

				var constraints nameConstraints
				_, err := asn1.Unmarshal(e.Value, &constraints)
				if err != nil {
					return nil, err
				}

				if e.Critical {
					out.NameConstraintsCritical = true
				}

				for _, subtree := range constraints.Permitted {
					switch subtree.Value.Tag {
					case 1:
						out.PermittedEmailAddresses = append(out.PermittedEmailAddresses, GeneralSubtreeString{Data: string(subtree.Value.Bytes), Max: subtree.Max, Min: subtree.Min})
					case 2:
						out.PermittedDNSNames = append(out.PermittedDNSNames, GeneralSubtreeString{Data: string(subtree.Value.Bytes), Max: subtree.Max, Min: subtree.Min})
					case 3:
						out.PermittedX400Addresses = append(out.PermittedX400Addresses, GeneralSubtreeRaw{Data: subtree.Value, Max: subtree.Max, Min: subtree.Min})
					case 4:
						var rawdn pkix.RDNSequence
						if _, err := asn1.Unmarshal(subtree.Value.Bytes, &rawdn); err != nil {
							return out, err
						}
						var dn pkix.Name
						dn.FillFromRDNSequence(&rawdn)
						out.PermittedDirectoryNames = append(out.PermittedDirectoryNames, GeneralSubtreeName{Data: dn, Max: subtree.Max, Min: subtree.Min})
					case 5:
						var ediName pkix.EDIPartyName
						_, err = asn1.UnmarshalWithParams(subtree.Value.FullBytes, &ediName, "tag:5")
						if err != nil {
							return out, err
						}
						out.PermittedEdiPartyNames = append(out.PermittedEdiPartyNames, GeneralSubtreeEdi{Data: ediName, Max: subtree.Max, Min: subtree.Min})
					case 6:
						out.PermittedURIs = append(out.PermittedURIs, GeneralSubtreeString{Data: string(subtree.Value.Bytes), Max: subtree.Max, Min: subtree.Min})
					case 7:
						switch len(subtree.Value.Bytes) {
						case net.IPv4len * 2:
							ip := net.IPNet{IP: subtree.Value.Bytes[:net.IPv4len], Mask: subtree.Value.Bytes[net.IPv4len:]}
							out.PermittedIPAddresses = append(out.PermittedIPAddresses, GeneralSubtreeIP{Data: ip, Max: subtree.Max, Min: subtree.Min})
						case net.IPv6len * 2:
							ip := net.IPNet{IP: subtree.Value.Bytes[:net.IPv6len], Mask: subtree.Value.Bytes[net.IPv6len:]}
							out.PermittedIPAddresses = append(out.PermittedIPAddresses, GeneralSubtreeIP{Data: ip, Max: subtree.Max, Min: subtree.Min})
						default:
							return out, errors.New("x509: certificate name constraint contained IP address range of length " + strconv.Itoa(len(subtree.Value.Bytes)))
						}
					case 8:
						var id asn1.ObjectIdentifier
						_, err = asn1.UnmarshalWithParams(subtree.Value.FullBytes, &id, "tag:8")
						if err != nil {
							return out, err
						}
						out.PermittedRegisteredIDs = append(out.PermittedRegisteredIDs, GeneralSubtreeOid{Data: id, Max: subtree.Max, Min: subtree.Min})
					}
				}
				for _, subtree := range constraints.Excluded {
					switch subtree.Value.Tag {
					case 1:
						out.ExcludedEmailAddresses = append(out.ExcludedEmailAddresses, GeneralSubtreeString{Data: string(subtree.Value.Bytes), Max: subtree.Max, Min: subtree.Min})
					case 2:
						out.ExcludedDNSNames = append(out.ExcludedDNSNames, GeneralSubtreeString{Data: string(subtree.Value.Bytes), Max: subtree.Max, Min: subtree.Min})
					case 3:
						out.ExcludedX400Addresses = append(out.ExcludedX400Addresses, GeneralSubtreeRaw{Data: subtree.Value, Max: subtree.Max, Min: subtree.Min})
					case 4:
						var rawdn pkix.RDNSequence
						if _, err := asn1.Unmarshal(subtree.Value.Bytes, &rawdn); err != nil {
							return out, err
						}
						var dn pkix.Name
						dn.FillFromRDNSequence(&rawdn)
						out.ExcludedDirectoryNames = append(out.ExcludedDirectoryNames, GeneralSubtreeName{Data: dn, Max: subtree.Max, Min: subtree.Min})
					case 5:
						var ediName pkix.EDIPartyName
						_, err = asn1.Unmarshal(subtree.Value.Bytes, &ediName)
						if err != nil {
							return out, err
						}
						out.ExcludedEdiPartyNames = append(out.ExcludedEdiPartyNames, GeneralSubtreeEdi{Data: ediName, Max: subtree.Max, Min: subtree.Min})
					case 6:
						out.ExcludedURIs = append(out.ExcludedURIs, GeneralSubtreeString{Data: string(subtree.Value.Bytes), Max: subtree.Max, Min: subtree.Min})
					case 7:
						switch len(subtree.Value.Bytes) {
						case net.IPv4len * 2:
							ip := net.IPNet{IP: subtree.Value.Bytes[:net.IPv4len], Mask: subtree.Value.Bytes[net.IPv4len:]}
							out.ExcludedIPAddresses = append(out.ExcludedIPAddresses, GeneralSubtreeIP{Data: ip, Max: subtree.Max, Min: subtree.Min})
						case net.IPv6len * 2:
							ip := net.IPNet{IP: subtree.Value.Bytes[:net.IPv6len], Mask: subtree.Value.Bytes[net.IPv6len:]}
							out.ExcludedIPAddresses = append(out.ExcludedIPAddresses, GeneralSubtreeIP{Data: ip, Max: subtree.Max, Min: subtree.Min})
						default:
							return out, errors.New("x509: certificate name constraint contained IP address range of length " + strconv.Itoa(len(subtree.Value.Bytes)))
						}
					case 8:
						var id asn1.ObjectIdentifier
						_, err = asn1.Unmarshal(subtree.Value.Bytes, &id)
						if err != nil {
							return out, err
						}
						out.ExcludedRegisteredIDs = append(out.ExcludedRegisteredIDs, GeneralSubtreeOid{Data: id, Max: subtree.Max, Min: subtree.Min})
					}
				}
				continue

			case 31:
				// RFC 5280, 4.2.1.14

				// CRLDistributionPoints ::= SEQUENCE SIZE (1..MAX) OF DistributionPoint
				//
				// DistributionPoint ::= SEQUENCE {
				//     distributionPoint       [0]     DistributionPointName OPTIONAL,
				//     reasons                 [1]     ReasonFlags OPTIONAL,
				//     cRLIssuer               [2]     GeneralNames OPTIONAL }
				//
				// DistributionPointName ::= CHOICE {
				//     fullName                [0]     GeneralNames,
				//     nameRelativeToCRLIssuer [1]     RelativeDistinguishedName }

				var cdp []distributionPoint
				_, err := asn1.Unmarshal(e.Value, &cdp)
				if err != nil {
					return nil, err
				}

				for _, dp := range cdp {
					// Per RFC 5280, 4.2.1.13, one of distributionPoint or cRLIssuer may be empty.
					if len(dp.DistributionPoint.FullName.Bytes) == 0 {
						continue
					}

					var n asn1.RawValue
					dpName := dp.DistributionPoint.FullName.Bytes
					// FullName is a GeneralNames, which is a SEQUENCE OF
					// GeneralName, which in turn is a CHOICE.
					// Per https://www.ietf.org/rfc/rfc5280.txt, multiple names
					// for a single DistributionPoint give different pointers to
					// the same CRL.
					for len(dpName) > 0 {
						dpName, err = asn1.Unmarshal(dpName, &n)
						if err != nil {
							return nil, err
						}
						if n.Tag == 6 {
							out.CRLDistributionPoints = append(out.CRLDistributionPoints, string(n.Bytes))
						}
					}
				}
				continue

			case 35:
				// RFC 5280, 4.2.1.1
				var a authKeyId
				_, err = asn1.Unmarshal(e.Value, &a)
				if err != nil {
					return nil, err
				}
				out.AuthorityKeyId = a.Id
				continue

			case 37:
				// RFC 5280, 4.2.1.12.  Extended Key Usage

				// id-ce-extKeyUsage OBJECT IDENTIFIER ::= { id-ce 37 }
				//
				// ExtKeyUsageSyntax ::= SEQUENCE SIZE (1..MAX) OF KeyPurposeId
				//
				// KeyPurposeId ::= OBJECT IDENTIFIER

				var keyUsage []asn1.ObjectIdentifier
				_, err = asn1.Unmarshal(e.Value, &keyUsage)
				if err != nil {
					return nil, err
				}

				for _, u := range keyUsage {
					if extKeyUsage, ok := extKeyUsageFromOID(u); ok {
						out.ExtKeyUsage = append(out.ExtKeyUsage, extKeyUsage)
					} else {
						out.UnknownExtKeyUsage = append(out.UnknownExtKeyUsage, u)
					}
				}

				continue

			case 14:
				// RFC 5280, 4.2.1.2
				var keyid []byte
				_, err = asn1.Unmarshal(e.Value, &keyid)
				if err != nil {
					return nil, err
				}
				out.SubjectKeyId = keyid
				continue

			case 32:
				// RFC 5280 4.2.1.4: Certificate Policies
				var policies []policyInformation
				if _, err = asn1.Unmarshal(e.Value, &policies); err != nil {
					return nil, err
				}
				out.PolicyIdentifiers = make([]asn1.ObjectIdentifier, len(policies))
				out.QualifierId = make([][]asn1.ObjectIdentifier, len(policies))
				out.ExplicitTexts = make([][]asn1.RawValue, len(policies))
				out.NoticeRefOrgnization = make([][]asn1.RawValue, len(policies))
				out.NoticeRefNumbers = make([][]NoticeNumber, len(policies))
				out.ParsedExplicitTexts = make([][]string, len(policies))
				out.ParsedNoticeRefOrganization = make([][]string, len(policies))
				out.CPSuri = make([][]string, len(policies))

				for i, policy := range policies {
					out.PolicyIdentifiers[i] = policy.Policy
					// parse optional Qualifier for zlint
					for _, qualifier := range policy.Qualifiers {
						out.QualifierId[i] = append(out.QualifierId[i], qualifier.PolicyQualifierId)
						userNoticeOID := asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 2, 2}
						cpsURIOID := asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 2, 1}
						if qualifier.PolicyQualifierId.Equal(userNoticeOID) {
							var un userNotice
							if _, err = asn1.Unmarshal(qualifier.Qualifier.FullBytes, &un); err != nil {
								return nil, err
							}
							if len(un.ExplicitText.Bytes) != 0 {
								out.ExplicitTexts[i] = append(out.ExplicitTexts[i], un.ExplicitText)
								out.ParsedExplicitTexts[i] = append(out.ParsedExplicitTexts[i], string(un.ExplicitText.Bytes))
							}
							if un.NoticeRef.Organization.Bytes != nil || un.NoticeRef.NoticeNumbers != nil {
								out.NoticeRefOrgnization[i] = append(out.NoticeRefOrgnization[i], un.NoticeRef.Organization)
								out.NoticeRefNumbers[i] = append(out.NoticeRefNumbers[i], un.NoticeRef.NoticeNumbers)
								out.ParsedNoticeRefOrganization[i] = append(out.ParsedNoticeRefOrganization[i], string(un.NoticeRef.Organization.Bytes))
							}
						}
						if qualifier.PolicyQualifierId.Equal(cpsURIOID) {
							var cpsURIRaw asn1.RawValue
							if _, err = asn1.Unmarshal(qualifier.Qualifier.FullBytes, &cpsURIRaw); err != nil {
								return nil, err
							}
							out.CPSuri[i] = append(out.CPSuri[i], string(cpsURIRaw.Bytes))
						}
					}
				}
				if out.SelfSigned {
					out.ValidationLevel = UnknownValidationLevel
				} else {
					// See http://unmitigatedrisk.com/?p=203
					validationLevel := getMaxCertValidationLevel(out.PolicyIdentifiers)
					if validationLevel == UnknownValidationLevel {
						if (len(out.Subject.Organization) > 0 && out.Subject.Organization[0] == out.Subject.CommonName) || (len(out.Subject.OrganizationalUnit) > 0 && strings.Contains(out.Subject.OrganizationalUnit[0], "Domain Control Validated")) {
							if len(out.Subject.Locality) == 0 && len(out.Subject.Province) == 0 && len(out.Subject.PostalCode) == 0 {
								validationLevel = DV
							}
						} else if len(out.Subject.Organization) > 0 && out.Subject.Organization[0] == "Persona Not Validated" && strings.Contains(out.Issuer.CommonName, "StartCom") {
							validationLevel = DV
						}
					}
					out.ValidationLevel = validationLevel
				}
			}
		} else if e.Id.Equal(oidExtensionAuthorityInfoAccess) {
			// RFC 5280 4.2.2.1: Authority Information Access
			var aia []authorityInfoAccess
			if _, err = asn1.Unmarshal(e.Value, &aia); err != nil {
				return nil, err
			}

			for _, v := range aia {
				// GeneralName: uniformResourceIdentifier [6] IA5String
				if v.Location.Tag != 6 {
					continue
				}
				if v.Method.Equal(oidAuthorityInfoAccessOcsp) {
					out.OCSPServer = append(out.OCSPServer, string(v.Location.Bytes))
				} else if v.Method.Equal(oidAuthorityInfoAccessIssuers) {
					out.IssuingCertificateURL = append(out.IssuingCertificateURL, string(v.Location.Bytes))
				}
			}
		} else if e.Id.Equal(oidExtensionSignedCertificateTimestampList) {
			err := parseSignedCertificateTimestampList(out, e)
			if err != nil {
				return nil, err
			}
		} else if e.Id.Equal(oidExtensionCTPrecertificatePoison) {
			if e.Value[0] == 5 && e.Value[1] == 0 {
				out.IsPrecert = true
				continue
			} else {
				return nil, UnhandledCriticalExtension{e.Id, "Malformed precert poison"}
			}
		} else if e.Id.Equal(oidBRTorServiceDescriptor) {
			descs, err := parseTorServiceDescriptorSyntax(e)
			if err != nil {
				return nil, err
			}
			out.TorServiceDescriptors = descs
		} else if e.Id.Equal(oidExtCABFOrganizationID) {
			cabf := CABFOrganizationIDASN{}
			_, err := asn1.Unmarshal(e.Value, &cabf)
			if err != nil {
				return nil, err
			}
			out.CABFOrganizationIdentifier = &CABFOrganizationIdentifier{
				Scheme:    cabf.RegistrationSchemeIdentifier,
				Country:   cabf.RegistrationCountry,
				Reference: cabf.RegistrationReference,
				State:     cabf.RegistrationStateOrProvince,
			}
		} else if e.Id.Equal(oidExtQCStatements) {
			rawStatements := QCStatementsASN{}
			_, err := asn1.Unmarshal(e.Value, &rawStatements.QCStatements)
			if err != nil {
				return nil, err
			}
			qcStatements := QCStatements{}
			if err := qcStatements.Parse(&rawStatements); err != nil {
				return nil, err
			}
			out.QCStatements = &qcStatements
		}

		//if e.Critical {
		//	return out, UnhandledCriticalExtension{e.Id}
		//}
	}

	return out, nil
}

func parseSignedCertificateTimestampList(out *Certificate, ext pkix.Extension) error {
	var scts []byte
	if _, err := asn1.Unmarshal(ext.Value, &scts); err != nil {
		return err
	}
	// ignore length of
	if len(scts) < 2 {
		return errors.New("malformed SCT extension: incomplete length field")
	}
	scts = scts[2:]
	headerLength := 2
	for {
		switch len(scts) {
		case 0:
			return nil
		case 1:
			return errors.New("malformed SCT extension: trailing data")
		default:
			sctLength := int(scts[1]) + (int(scts[0]) << 8) + headerLength
			if !(sctLength <= len(scts)) {
				return errors.New("malformed SCT extension: incomplete SCT")
			}
			sct, err := ct.DeserializeSCT(bytes.NewReader(scts[headerLength:sctLength]))
			if err != nil {
				return fmt.Errorf("malformed SCT extension: SCT parse err: %v", err)
			}
			out.SignedCertificateTimestampList = append(out.SignedCertificateTimestampList, sct)
			scts = scts[sctLength:]
		}
	}
}

// ParseCertificate parses a single certificate from the given ASN.1 DER data.
func ParseCertificate(asn1Data []byte) (*Certificate, error) {
	var cert certificate
	rest, err := asn1.Unmarshal(asn1Data, &cert)
	if err != nil {
		return nil, err
	}
	if len(rest) > 0 {
		return nil, asn1.SyntaxError{Msg: "trailing data"}
	}

	return parseCertificate(&cert)
}

// ParseCertificates parses one or more certificates from the given ASN.1 DER
// data. The certificates must be concatenated with no intermediate padding.
func ParseCertificates(asn1Data []byte) ([]*Certificate, error) {
	var v []*certificate

	for len(asn1Data) > 0 {
		cert := new(certificate)
		var err error
		asn1Data, err = asn1.Unmarshal(asn1Data, cert)
		if err != nil {
			return nil, err
		}
		v = append(v, cert)
	}

	ret := make([]*Certificate, len(v))
	for i, ci := range v {
		cert, err := parseCertificate(ci)
		if err != nil {
			return nil, err
		}
		ret[i] = cert
	}

	return ret, nil
}

func ParseTBSCertificate(asn1Data []byte) (*Certificate, error) {
	var tbsCert tbsCertificate
	rest, err := asn1.Unmarshal(asn1Data, &tbsCert)
	if err != nil {
		//log.Print("Err unmarshalling asn1Data", asn1Data, rest)
		return nil, err
	}
	if len(rest) > 0 {
		return nil, asn1.SyntaxError{Msg: "trailing data"}
	}
	return parseCertificate(&certificate{
		Raw:            tbsCert.Raw,
		TBSCertificate: tbsCert})
}

// SubjectAndKey represents a (subjecty, subject public key info) tuple.
type SubjectAndKey struct {
	RawSubject              []byte
	RawSubjectPublicKeyInfo []byte
	Fingerprint             CertificateFingerprint
	PublicKey               interface{}
	PublicKeyAlgorithm      PublicKeyAlgorithm
}

// SubjectAndKey returns a SubjectAndKey for this certificate.
func (c *Certificate) SubjectAndKey() *SubjectAndKey {
	return &SubjectAndKey{
		RawSubject:              c.RawSubject,
		RawSubjectPublicKeyInfo: c.RawSubjectPublicKeyInfo,
		Fingerprint:             c.SPKISubjectFingerprint,
		PublicKey:               c.PublicKey,
		PublicKeyAlgorithm:      c.PublicKeyAlgorithm,
	}
}

func reverseBitsInAByte(in byte) byte {
	b1 := in>>4 | in<<4
	b2 := b1>>2&0x33 | b1<<2&0xcc
	b3 := b2>>1&0x55 | b2<<1&0xaa
	return b3
}

// asn1BitLength returns the bit-length of bitString by considering the
// most-significant bit in a byte to be the "first" bit. This convention
// matches ASN.1, but differs from almost everything else.
func asn1BitLength(bitString []byte) int {
	bitLen := len(bitString) * 8

	for i := range bitString {
		b := bitString[len(bitString)-i-1]

		for bit := uint(0); bit < 8; bit++ {
			if (b>>bit)&1 == 1 {
				return bitLen
			}
			bitLen--
		}
	}

	return 0
}

var (
	oidExtensionSubjectKeyId                   = []int{2, 5, 29, 14}
	oidExtensionKeyUsage                       = []int{2, 5, 29, 15}
	oidExtensionExtendedKeyUsage               = []int{2, 5, 29, 37}
	oidExtensionAuthorityKeyId                 = []int{2, 5, 29, 35}
	oidExtensionBasicConstraints               = []int{2, 5, 29, 19}
	oidExtensionSubjectAltName                 = []int{2, 5, 29, 17}
	oidExtensionIssuerAltName                  = []int{2, 5, 29, 18}
	oidExtensionCertificatePolicies            = []int{2, 5, 29, 32}
	oidExtensionNameConstraints                = []int{2, 5, 29, 30}
	oidExtensionCRLDistributionPoints          = []int{2, 5, 29, 31}
	oidExtensionAuthorityInfoAccess            = []int{1, 3, 6, 1, 5, 5, 7, 1, 1}
	oidExtensionSignedCertificateTimestampList = []int{1, 3, 6, 1, 4, 1, 11129, 2, 4, 2}
)

var (
	oidAuthorityInfoAccessOcsp    = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 48, 1}
	oidAuthorityInfoAccessIssuers = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 48, 2}
)

// oidNotInExtensions returns whether an extension with the given oid exists in
// extensions.
func oidInExtensions(oid asn1.ObjectIdentifier, extensions []pkix.Extension) bool {
	for _, e := range extensions {
		if e.Id.Equal(oid) {
			return true
		}
	}
	return false
}

// marshalSANs marshals a list of addresses into a the contents of an X.509
// SubjectAlternativeName extension.
func marshalSANs(dnsNames, emailAddresses []string, ipAddresses []net.IP) (derBytes []byte, err error) {
	var rawValues []asn1.RawValue
	for _, name := range dnsNames {
		rawValues = append(rawValues, asn1.RawValue{Tag: 2, Class: 2, Bytes: []byte(name)})
	}
	for _, email := range emailAddresses {
		rawValues = append(rawValues, asn1.RawValue{Tag: 1, Class: 2, Bytes: []byte(email)})
	}
	for _, rawIP := range ipAddresses {
		// If possible, we always want to encode IPv4 addresses in 4 bytes.
		ip := rawIP.To4()
		if ip == nil {
			ip = rawIP
		}
		rawValues = append(rawValues, asn1.RawValue{Tag: 7, Class: 2, Bytes: ip})
	}
	return asn1.Marshal(rawValues)
}

// NOTE ignoring authorityKeyID argument
func buildExtensions(template *Certificate, _ []byte) (ret []pkix.Extension, err error) {
	ret = make([]pkix.Extension, 10 /* Max number of elements. */)
	n := 0

	if template.KeyUsage != 0 &&
		!oidInExtensions(oidExtensionKeyUsage, template.ExtraExtensions) {
		ret[n].Id = oidExtensionKeyUsage
		ret[n].Critical = true

		var a [2]byte
		a[0] = reverseBitsInAByte(byte(template.KeyUsage))
		a[1] = reverseBitsInAByte(byte(template.KeyUsage >> 8))

		l := 1
		if a[1] != 0 {
			l = 2
		}

		ret[n].Value, err = asn1.Marshal(asn1.BitString{Bytes: a[0:l], BitLength: l * 8})
		if err != nil {
			return
		}
		n++
	}

	if (len(template.ExtKeyUsage) > 0 || len(template.UnknownExtKeyUsage) > 0) &&
		!oidInExtensions(oidExtensionExtendedKeyUsage, template.ExtraExtensions) {
		ret[n].Id = oidExtensionExtendedKeyUsage

		var oids []asn1.ObjectIdentifier
		for _, u := range template.ExtKeyUsage {
			if oid, ok := oidFromExtKeyUsage(u); ok {
				oids = append(oids, oid)
			} else {
				panic("internal error")
			}
		}

		oids = append(oids, template.UnknownExtKeyUsage...)

		ret[n].Value, err = asn1.Marshal(oids)
		if err != nil {
			return
		}
		n++
	}

	if template.BasicConstraintsValid && !oidInExtensions(oidExtensionBasicConstraints, template.ExtraExtensions) {
		// Leaving MaxPathLen as zero indicates that no Max path
		// length is desired, unless MaxPathLenZero is set. A value of
		// -1 causes encoding/asn1 to omit the value as desired.
		maxPathLen := template.MaxPathLen
		if maxPathLen == 0 && !template.MaxPathLenZero {
			maxPathLen = -1
		}
		ret[n].Id = oidExtensionBasicConstraints
		ret[n].Value, err = asn1.Marshal(basicConstraints{template.IsCA, maxPathLen})
		ret[n].Critical = true
		if err != nil {
			return
		}
		n++
	}

	if len(template.SubjectKeyId) > 0 && !oidInExtensions(oidExtensionSubjectKeyId, template.ExtraExtensions) {
		ret[n].Id = oidExtensionSubjectKeyId
		ret[n].Value, err = asn1.Marshal(template.SubjectKeyId)
		if err != nil {
			return
		}
		n++
	}

	if len(template.AuthorityKeyId) > 0 && !oidInExtensions(oidExtensionAuthorityKeyId, template.ExtraExtensions) {
		ret[n].Id = oidExtensionAuthorityKeyId
		ret[n].Value, err = asn1.Marshal(authKeyId{template.AuthorityKeyId})
		if err != nil {
			return
		}
		n++
	}

	if (len(template.OCSPServer) > 0 || len(template.IssuingCertificateURL) > 0) &&
		!oidInExtensions(oidExtensionAuthorityInfoAccess, template.ExtraExtensions) {
		ret[n].Id = oidExtensionAuthorityInfoAccess
		var aiaValues []authorityInfoAccess
		for _, name := range template.OCSPServer {
			aiaValues = append(aiaValues, authorityInfoAccess{
				Method:   oidAuthorityInfoAccessOcsp,
				Location: asn1.RawValue{Tag: 6, Class: 2, Bytes: []byte(name)},
			})
		}
		for _, name := range template.IssuingCertificateURL {
			aiaValues = append(aiaValues, authorityInfoAccess{
				Method:   oidAuthorityInfoAccessIssuers,
				Location: asn1.RawValue{Tag: 6, Class: 2, Bytes: []byte(name)},
			})
		}
		ret[n].Value, err = asn1.Marshal(aiaValues)
		if err != nil {
			return
		}
		n++
	}

	if (len(template.DNSNames) > 0 || len(template.EmailAddresses) > 0 || len(template.IPAddresses) > 0) &&
		!oidInExtensions(oidExtensionSubjectAltName, template.ExtraExtensions) {
		ret[n].Id = oidExtensionSubjectAltName
		ret[n].Value, err = marshalSANs(template.DNSNames, template.EmailAddresses, template.IPAddresses)
		if err != nil {
			return
		}
		n++
	}

	if len(template.PolicyIdentifiers) > 0 &&
		!oidInExtensions(oidExtensionCertificatePolicies, template.ExtraExtensions) {
		ret[n].Id = oidExtensionCertificatePolicies
		policies := make([]policyInformation, len(template.PolicyIdentifiers))
		for i, policy := range template.PolicyIdentifiers {
			policies[i].Policy = policy
		}
		ret[n].Value, err = asn1.Marshal(policies)
		if err != nil {
			return
		}
		n++
	}

	// TODO: this can be cleaned up in go1.10
	if (len(template.PermittedEmailAddresses) > 0 || len(template.PermittedDNSNames) > 0 || len(template.PermittedDirectoryNames) > 0 ||
		len(template.PermittedIPAddresses) > 0 || len(template.ExcludedEmailAddresses) > 0 || len(template.ExcludedDNSNames) > 0 ||
		len(template.ExcludedDirectoryNames) > 0 || len(template.ExcludedIPAddresses) > 0) &&
		!oidInExtensions(oidExtensionNameConstraints, template.ExtraExtensions) {
		ret[n].Id = oidExtensionNameConstraints
		if template.NameConstraintsCritical {
			ret[n].Critical = true
		}

		var out nameConstraints

		for _, permitted := range template.PermittedEmailAddresses {
			out.Permitted = append(out.Permitted, generalSubtree{Value: asn1.RawValue{Tag: 1, Class: 2, Bytes: []byte(permitted.Data)}})
		}
		for _, excluded := range template.ExcludedEmailAddresses {
			out.Excluded = append(out.Excluded, generalSubtree{Value: asn1.RawValue{Tag: 1, Class: 2, Bytes: []byte(excluded.Data)}})
		}
		for _, permitted := range template.PermittedDNSNames {
			out.Permitted = append(out.Permitted, generalSubtree{Value: asn1.RawValue{Tag: 2, Class: 2, Bytes: []byte(permitted.Data)}})
		}
		for _, excluded := range template.ExcludedDNSNames {
			out.Excluded = append(out.Excluded, generalSubtree{Value: asn1.RawValue{Tag: 2, Class: 2, Bytes: []byte(excluded.Data)}})
		}
		for _, permitted := range template.PermittedDirectoryNames {
			var dn []byte
			dn, err = asn1.Marshal(permitted.Data.ToRDNSequence())
			if err != nil {
				return
			}
			out.Permitted = append(out.Permitted, generalSubtree{Value: asn1.RawValue{Tag: 4, Class: 2, IsCompound: true, Bytes: dn}})
		}
		for _, excluded := range template.ExcludedDirectoryNames {
			var dn []byte
			dn, err = asn1.Marshal(excluded.Data.ToRDNSequence())
			if err != nil {
				return
			}
			out.Excluded = append(out.Excluded, generalSubtree{Value: asn1.RawValue{Tag: 4, Class: 2, IsCompound: true, Bytes: dn}})
		}
		for _, permitted := range template.PermittedIPAddresses {
			ip := append(permitted.Data.IP, permitted.Data.Mask...)
			out.Permitted = append(out.Permitted, generalSubtree{Value: asn1.RawValue{Tag: 7, Class: 2, Bytes: ip}})
		}
		for _, excluded := range template.ExcludedIPAddresses {
			ip := append(excluded.Data.IP, excluded.Data.Mask...)
			out.Excluded = append(out.Excluded, generalSubtree{Value: asn1.RawValue{Tag: 7, Class: 2, Bytes: ip}})
		}
		ret[n].Value, err = asn1.Marshal(out)
		if err != nil {
			return
		}
		n++
	}

	if len(template.CRLDistributionPoints) > 0 &&
		!oidInExtensions(oidExtensionCRLDistributionPoints, template.ExtraExtensions) {
		ret[n].Id = oidExtensionCRLDistributionPoints

		var crlDp []distributionPoint
		for _, name := range template.CRLDistributionPoints {
			rawFullName, _ := asn1.Marshal(asn1.RawValue{Tag: 6, Class: 2, Bytes: []byte(name)})

			dp := distributionPoint{
				DistributionPoint: distributionPointName{
					FullName: asn1.RawValue{Tag: 0, Class: 2, IsCompound: true, Bytes: rawFullName},
				},
			}
			crlDp = append(crlDp, dp)
		}

		ret[n].Value, err = asn1.Marshal(crlDp)
		if err != nil {
			return
		}
		n++
	}

	// Adding another extension here? Remember to update the Max number
	// of elements in the make() at the top of the function.

	return append(ret[:n], template.ExtraExtensions...), nil
}

func subjectBytes(cert *Certificate) ([]byte, error) {
	if len(cert.RawSubject) > 0 {
		return cert.RawSubject, nil
	}

	return asn1.Marshal(cert.Subject.ToRDNSequence())
}

// signingParamsForPublicKey returns the parameters to use for signing with
// priv. If requestedSigAlgo is not zero then it overrides the default
// signature algorithm.
func signingParamsForPublicKey(pub interface{}, requestedSigAlgo SignatureAlgorithm) (hashFunc crypto.Hash, sigAlgo pkix.AlgorithmIdentifier, err error) {
	var pubType PublicKeyAlgorithm
	shouldHash := true

	switch pub := pub.(type) {
	case *rsa.PublicKey:
		pubType = RSA
		hashFunc = crypto.SHA256
		sigAlgo.Algorithm = oidSignatureSHA256WithRSA
		sigAlgo.Parameters = asn1.NullRawValue

	case *ecdsa.PublicKey:
		pubType = ECDSA

		switch pub.Curve {
		case elliptic.P224(), elliptic.P256():
			hashFunc = crypto.SHA256
			sigAlgo.Algorithm = oidSignatureECDSAWithSHA256
		case elliptic.P384():
			hashFunc = crypto.SHA384
			sigAlgo.Algorithm = oidSignatureECDSAWithSHA384
		case elliptic.P521():
			hashFunc = crypto.SHA512
			sigAlgo.Algorithm = oidSignatureECDSAWithSHA512
		default:
			err = errors.New("x509: unknown elliptic curve")
		}

	case ed25519.PublicKey:
		pubType = Ed25519
		hashFunc = 0
		shouldHash = false
		sigAlgo.Algorithm = oidKeyEd25519

	default:
		err = errors.New("x509: only RSA, ECDSA, Ed25519, and X25519 keys supported")
	}

	if err != nil {
		return
	}

	if requestedSigAlgo == 0 {
		return
	}

	found := false
	for _, details := range signatureAlgorithmDetails {
		if details.algo == requestedSigAlgo {
			if details.pubKeyAlgo != pubType {
				err = errors.New("x509: requested SignatureAlgorithm does not match private key type")
				return
			}
			sigAlgo.Algorithm, hashFunc = details.oid, details.hash
			if hashFunc == 0 && shouldHash {
				err = errors.New("x509: cannot sign with hash function requested")
				return
			}
			if requestedSigAlgo.isRSAPSS() {
				sigAlgo.Parameters = rsaPSSParameters(hashFunc)
			}
			found = true
			break
		}
	}

	if !found {
		err = errors.New("x509: unknown SignatureAlgorithm")
	}

	return
}

// CreateCertificate creates a new certificate based on a template.
// The following members of template are used: AuthorityKeyId,
// BasicConstraintsValid, DNSNames, ExcludedDNSDomains, ExtKeyUsage,
// IsCA, KeyUsage, MaxPathLen, MaxPathLenZero, NotAfter, NotBefore,
// PermittedDNSDomains, PermittedDNSDomainsCritical, SerialNumber,
// SignatureAlgorithm, Subject, SubjectKeyId, and UnknownExtKeyUsage.
//
// The certificate is signed by parent. If parent is equal to template then the
// certificate is self-signed. The parameter pub is the public key of the
// signee and priv is the private key of the signer.
//
// The returned slice is the certificate in DER encoding.
//
// All keys types that are implemented via crypto.Signer are supported (This
// includes *rsa.PublicKey and *ecdsa.PublicKey.)
//
// The AuthorityKeyId will be taken from the SubjectKeyId of parent, if any,
// unless the resulting certificate is self-signed. Otherwise the value from
// template will be used.
func CreateCertificate(rand io.Reader, template, parent *Certificate, pub, priv interface{}) (cert []byte, err error) {
	key, ok := priv.(crypto.Signer)
	if !ok {
		return nil, errors.New("x509: certificate private key does not implement crypto.Signer")
	}

	if template.SerialNumber == nil {
		return nil, errors.New("x509: no SerialNumber given")
	}

	hashFunc, signatureAlgorithm, err := signingParamsForPublicKey(key.Public(), template.SignatureAlgorithm)
	if err != nil {
		return nil, err
	}

	publicKeyBytes, publicKeyAlgorithm, err := marshalPublicKey(pub)
	if err != nil {
		return nil, err
	}

	asn1Issuer, err := subjectBytes(parent)
	if err != nil {
		return
	}

	asn1Subject, err := subjectBytes(template)
	if err != nil {
		return
	}

	authorityKeyId := template.AuthorityKeyId
	if !bytes.Equal(asn1Issuer, asn1Subject) && len(parent.SubjectKeyId) > 0 {
		authorityKeyId = parent.SubjectKeyId
	}

	extensions, err := buildExtensions(template, authorityKeyId)
	if err != nil {
		return
	}

	encodedPublicKey := asn1.BitString{BitLength: len(publicKeyBytes) * 8, Bytes: publicKeyBytes}
	c := tbsCertificate{
		Version:            2,
		SerialNumber:       template.SerialNumber,
		SignatureAlgorithm: signatureAlgorithm,
		Issuer:             asn1.RawValue{FullBytes: asn1Issuer},
		Validity:           validity{template.NotBefore.UTC(), template.NotAfter.UTC()},
		Subject:            asn1.RawValue{FullBytes: asn1Subject},
		PublicKey:          publicKeyInfo{nil, publicKeyAlgorithm, encodedPublicKey},
		Extensions:         extensions,
	}

	tbsCertContents, err := asn1.Marshal(c)
	if err != nil {
		return
	}
	c.Raw = tbsCertContents

	digest := hash(hashFunc, c.Raw)

	var signerOpts crypto.SignerOpts
	signerOpts = hashFunc
	if template.SignatureAlgorithm != 0 && template.SignatureAlgorithm.isRSAPSS() {
		signerOpts = &rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthEqualsHash,
			Hash:       hashFunc,
		}
	}

	var signature []byte
	signature, err = key.Sign(rand, digest, signerOpts)
	if err != nil {
		return
	}

	return asn1.Marshal(certificate{
		nil,
		c,
		signatureAlgorithm,
		asn1.BitString{Bytes: signature, BitLength: len(signature) * 8},
	})
}

// pemCRLPrefix is the magic string that indicates that we have a PEM encoded
// CRL.
var pemCRLPrefix = []byte("-----BEGIN X509 CRL")

// pemType is the type of a PEM encoded CRL.
var pemType = "X509 CRL"

// ParseCRL parses a CRL from the given bytes. It's often the case that PEM
// encoded CRLs will appear where they should be DER encoded, so this function
// will transparently handle PEM encoding as long as there isn't any leading
// garbage.
func ParseCRL(crlBytes []byte) (*pkix.CertificateList, error) {
	if bytes.HasPrefix(crlBytes, pemCRLPrefix) {
		block, _ := pem.Decode(crlBytes)
		if block != nil && block.Type == pemType {
			crlBytes = block.Bytes
		}
	}
	return ParseDERCRL(crlBytes)
}

// ParseDERCRL parses a DER encoded CRL from the given bytes.
func ParseDERCRL(derBytes []byte) (*pkix.CertificateList, error) {
	certList := new(pkix.CertificateList)
	if rest, err := asn1.Unmarshal(derBytes, certList); err != nil {
		return nil, err
	} else if len(rest) != 0 {
		return nil, errors.New("x509: trailing data after CRL")
	}
	return certList, nil
}

// CreateCRL returns a DER encoded CRL, signed by this Certificate, that
// contains the given list of revoked certificates.
func (c *Certificate) CreateCRL(rand io.Reader, priv interface{}, revokedCerts []pkix.RevokedCertificate, now, expiry time.Time) (crlBytes []byte, err error) {
	key, ok := priv.(crypto.Signer)
	if !ok {
		return nil, errors.New("x509: certificate private key does not implement crypto.Signer")
	}

	hashFunc, signatureAlgorithm, err := signingParamsForPublicKey(key.Public(), 0)
	if err != nil {
		return nil, err
	}

	// Force revocation times to UTC per RFC 5280.
	revokedCertsUTC := make([]pkix.RevokedCertificate, len(revokedCerts))
	for i, rc := range revokedCerts {
		rc.RevocationTime = rc.RevocationTime.UTC()
		revokedCertsUTC[i] = rc
	}

	tbsCertList := pkix.TBSCertificateList{
		Version:             1,
		Signature:           signatureAlgorithm,
		Issuer:              c.Subject.ToRDNSequence(),
		ThisUpdate:          now.UTC(),
		NextUpdate:          expiry.UTC(),
		RevokedCertificates: revokedCertsUTC,
	}

	// Authority Key Id
	if len(c.SubjectKeyId) > 0 {
		var aki pkix.Extension
		aki.Id = oidExtensionAuthorityKeyId
		aki.Value, err = asn1.Marshal(authKeyId{Id: c.SubjectKeyId})
		if err != nil {
			return
		}
		tbsCertList.Extensions = append(tbsCertList.Extensions, aki)
	}

	tbsCertListContents, err := asn1.Marshal(tbsCertList)
	if err != nil {
		return
	}

	digest := hash(hashFunc, tbsCertListContents)

	var signature []byte
	signature, err = key.Sign(rand, digest, hashFunc)
	if err != nil {
		return
	}

	return asn1.Marshal(pkix.CertificateList{
		TBSCertList:        tbsCertList,
		SignatureAlgorithm: signatureAlgorithm,
		SignatureValue:     asn1.BitString{Bytes: signature, BitLength: len(signature) * 8},
	})
}

// CertificateRequest represents a PKCS #10, certificate signature request.
type CertificateRequest struct {
	Raw                      []byte // Complete ASN.1 DER content (CSR, signature algorithm and signature).
	RawTBSCertificateRequest []byte // Certificate request info part of raw ASN.1 DER content.
	RawSubjectPublicKeyInfo  []byte // DER encoded SubjectPublicKeyInfo.
	RawSubject               []byte // DER encoded Subject.

	Version            int
	Signature          []byte
	SignatureAlgorithm SignatureAlgorithm

	PublicKeyAlgorithm PublicKeyAlgorithm
	PublicKey          interface{}

	Subject pkix.Name

	// Attributes is the dried husk of a bug and shouldn't be used.
	Attributes []pkix.AttributeTypeAndValueSET

	// Extensions contains raw X.509 extensions. When parsing CSRs, this
	// can be used to extract extensions that are not parsed by this
	// package.
	Extensions []pkix.Extension

	// ExtraExtensions contains extensions to be copied, raw, into any
	// marshaled CSR. Values override any extensions that would otherwise
	// be produced based on the other fields but are overridden by any
	// extensions specified in Attributes.
	//
	// The ExtraExtensions field is not populated when parsing CSRs, see
	// Extensions.
	ExtraExtensions []pkix.Extension

	// Subject Alternate Name values.
	DNSNames       []string
	EmailAddresses []string
	IPAddresses    []net.IP
}

// These structures reflect the ASN.1 structure of X.509 certificate
// signature requests (see RFC 2986):

type tbsCertificateRequest struct {
	Raw           asn1.RawContent
	Version       int
	Subject       asn1.RawValue
	PublicKey     publicKeyInfo
	RawAttributes []asn1.RawValue `asn1:"tag:0"`
}

type certificateRequest struct {
	Raw                asn1.RawContent
	TBSCSR             tbsCertificateRequest
	SignatureAlgorithm pkix.AlgorithmIdentifier
	SignatureValue     asn1.BitString
}

// oidExtensionRequest is a PKCS#9 OBJECT IDENTIFIER that indicates requested
// extensions in a CSR.
var oidExtensionRequest = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 14}

// newRawAttributes converts AttributeTypeAndValueSETs from a template
// CertificateRequest's Attributes into tbsCertificateRequest RawAttributes.
func newRawAttributes(attributes []pkix.AttributeTypeAndValueSET) ([]asn1.RawValue, error) {
	var rawAttributes []asn1.RawValue
	b, err := asn1.Marshal(attributes)
	if err != nil {
		return nil, err
	}
	rest, err := asn1.Unmarshal(b, &rawAttributes)
	if err != nil {
		return nil, err
	}
	if len(rest) != 0 {
		return nil, errors.New("x509: failed to unmarshal raw CSR Attributes")
	}
	return rawAttributes, nil
}

// parseRawAttributes Unmarshals RawAttributes intos AttributeTypeAndValueSETs.
func parseRawAttributes(rawAttributes []asn1.RawValue) []pkix.AttributeTypeAndValueSET {
	var attributes []pkix.AttributeTypeAndValueSET
	for _, rawAttr := range rawAttributes {
		var attr pkix.AttributeTypeAndValueSET
		rest, err := asn1.Unmarshal(rawAttr.FullBytes, &attr)
		// Ignore attributes that don't parse into pkix.AttributeTypeAndValueSET
		// (i.e.: challengePassword or unstructuredName).
		if err == nil && len(rest) == 0 {
			attributes = append(attributes, attr)
		}
	}
	return attributes
}

// parseCSRExtensions parses the attributes from a CSR and extracts any
// requested extensions.
func parseCSRExtensions(rawAttributes []asn1.RawValue) ([]pkix.Extension, error) {
	// pkcs10Attribute reflects the Attribute structure from section 4.1 of
	// https://tools.ietf.org/html/rfc2986.
	type pkcs10Attribute struct {
		Id     asn1.ObjectIdentifier
		Values []asn1.RawValue `asn1:"set"`
	}

	var ret []pkix.Extension
	for _, rawAttr := range rawAttributes {
		var attr pkcs10Attribute
		if rest, err := asn1.Unmarshal(rawAttr.FullBytes, &attr); err != nil || len(rest) != 0 || len(attr.Values) == 0 {
			// Ignore attributes that don't parse.
			continue
		}

		if !attr.Id.Equal(oidExtensionRequest) {
			continue
		}

		var extensions []pkix.Extension
		if _, err := asn1.Unmarshal(attr.Values[0].FullBytes, &extensions); err != nil {
			return nil, err
		}
		ret = append(ret, extensions...)
	}

	return ret, nil
}

// CreateCertificateRequest creates a new certificate request based on a
// template. The following members of template are used: Attributes, DNSNames,
// EmailAddresses, ExtraExtensions, IPAddresses, SignatureAlgorithm, and
// Subject. The private key is the private key of the signer.
//
// The returned slice is the certificate request in DER encoding.
//
// All keys types that are implemented via crypto.Signer are supported (This
// includes *rsa.PublicKey and *ecdsa.PublicKey.)
func CreateCertificateRequest(rand io.Reader, template *CertificateRequest, priv interface{}) (csr []byte, err error) {
	key, ok := priv.(crypto.Signer)
	if !ok {
		return nil, errors.New("x509: certificate private key does not implement crypto.Signer")
	}

	var hashFunc crypto.Hash
	var sigAlgo pkix.AlgorithmIdentifier
	hashFunc, sigAlgo, err = signingParamsForPublicKey(key.Public(), template.SignatureAlgorithm)
	if err != nil {
		return nil, err
	}

	var publicKeyBytes []byte
	var publicKeyAlgorithm pkix.AlgorithmIdentifier
	publicKeyBytes, publicKeyAlgorithm, err = marshalPublicKey(key.Public())
	if err != nil {
		return nil, err
	}

	var extensions []pkix.Extension

	if (len(template.DNSNames) > 0 || len(template.EmailAddresses) > 0 || len(template.IPAddresses) > 0) &&
		!oidInExtensions(oidExtensionSubjectAltName, template.ExtraExtensions) {
		sanBytes, err := marshalSANs(template.DNSNames, template.EmailAddresses, template.IPAddresses)
		if err != nil {
			return nil, err
		}

		extensions = append(extensions, pkix.Extension{
			Id:    oidExtensionSubjectAltName,
			Value: sanBytes,
		})
	}

	extensions = append(extensions, template.ExtraExtensions...)

	var attributes []pkix.AttributeTypeAndValueSET
	attributes = append(attributes, template.Attributes...)

	if len(extensions) > 0 {
		// specifiedExtensions contains all the extensions that we
		// found specified via template.Attributes.
		specifiedExtensions := make(map[string]bool)

		for _, atvSet := range template.Attributes {
			if !atvSet.Type.Equal(oidExtensionRequest) {
				continue
			}

			for _, atvs := range atvSet.Value {
				for _, atv := range atvs {
					specifiedExtensions[atv.Type.String()] = true
				}
			}
		}

		atvs := make([]pkix.AttributeTypeAndValue, 0, len(extensions))
		for _, e := range extensions {
			if specifiedExtensions[e.Id.String()] {
				// Attributes already contained a value for
				// this extension and it takes priority.
				continue
			}

			atvs = append(atvs, pkix.AttributeTypeAndValue{
				// There is no place for the critical flag in a CSR.
				Type:  e.Id,
				Value: e.Value,
			})
		}

		// Append the extensions to an existing attribute if possible.
		appended := false
		for _, atvSet := range attributes {
			if !atvSet.Type.Equal(oidExtensionRequest) || len(atvSet.Value) == 0 {
				continue
			}

			atvSet.Value[0] = append(atvSet.Value[0], atvs...)
			appended = true
			break
		}

		// Otherwise, add a new attribute for the extensions.
		if !appended {
			attributes = append(attributes, pkix.AttributeTypeAndValueSET{
				Type: oidExtensionRequest,
				Value: [][]pkix.AttributeTypeAndValue{
					atvs,
				},
			})
		}
	}

	asn1Subject := template.RawSubject
	if len(asn1Subject) == 0 {
		asn1Subject, err = asn1.Marshal(template.Subject.ToRDNSequence())
		if err != nil {
			return
		}
	}

	rawAttributes, err := newRawAttributes(attributes)
	if err != nil {
		return
	}

	tbsCSR := tbsCertificateRequest{
		Version: 0, // PKCS #10, RFC 2986
		Subject: asn1.RawValue{FullBytes: asn1Subject},
		PublicKey: publicKeyInfo{
			Algorithm: publicKeyAlgorithm,
			PublicKey: asn1.BitString{
				Bytes:     publicKeyBytes,
				BitLength: len(publicKeyBytes) * 8,
			},
		},
		RawAttributes: rawAttributes,
	}

	tbsCSRContents, err := asn1.Marshal(tbsCSR)
	if err != nil {
		return
	}
	tbsCSR.Raw = tbsCSRContents

	digest := hash(hashFunc, tbsCSRContents)

	var signature []byte
	signature, err = key.Sign(rand, digest, hashFunc)
	if err != nil {
		return
	}

	return asn1.Marshal(certificateRequest{
		TBSCSR:             tbsCSR,
		SignatureAlgorithm: sigAlgo,
		SignatureValue: asn1.BitString{
			Bytes:     signature,
			BitLength: len(signature) * 8,
		},
	})
}

// ParseCertificateRequest parses a single certificate request from the
// given ASN.1 DER data.
func ParseCertificateRequest(asn1Data []byte) (*CertificateRequest, error) {
	var csr certificateRequest

	rest, err := asn1.Unmarshal(asn1Data, &csr)
	if err != nil {
		return nil, err
	} else if len(rest) != 0 {
		return nil, asn1.SyntaxError{Msg: "trailing data"}
	}

	return parseCertificateRequest(&csr)
}

func parseCertificateRequest(in *certificateRequest) (*CertificateRequest, error) {
	out := &CertificateRequest{
		Raw:                      in.Raw,
		RawTBSCertificateRequest: in.TBSCSR.Raw,
		RawSubjectPublicKeyInfo:  in.TBSCSR.PublicKey.Raw,
		RawSubject:               in.TBSCSR.Subject.FullBytes,

		Signature:          in.SignatureValue.RightAlign(),
		SignatureAlgorithm: GetSignatureAlgorithmFromAI(in.SignatureAlgorithm),

		PublicKeyAlgorithm: getPublicKeyAlgorithmFromOID(in.TBSCSR.PublicKey.Algorithm.Algorithm),

		Version:    in.TBSCSR.Version,
		Attributes: parseRawAttributes(in.TBSCSR.RawAttributes),
	}

	var err error
	out.PublicKey, err = parsePublicKey(out.PublicKeyAlgorithm, &in.TBSCSR.PublicKey)
	if err != nil {
		return nil, err
	}

	var subject pkix.RDNSequence
	if rest, err := asn1.Unmarshal(in.TBSCSR.Subject.FullBytes, &subject); err != nil {
		return nil, err
	} else if len(rest) != 0 {
		return nil, errors.New("x509: trailing data after X.509 Subject")
	}

	out.Subject.FillFromRDNSequence(&subject)

	if out.Extensions, err = parseCSRExtensions(in.TBSCSR.RawAttributes); err != nil {
		return nil, err
	}

	for _, extension := range out.Extensions {
		if extension.Id.Equal(oidExtensionSubjectAltName) {
			out.DNSNames, out.EmailAddresses, out.IPAddresses, err = parseSANExtension(extension.Value)
			if err != nil {
				return nil, err
			}
		}
	}

	return out, nil
}

// CheckSignature reports whether the signature on c is valid.
func (c *CertificateRequest) CheckSignature() error {
	return CheckSignatureFromKey(c.PublicKey, c.SignatureAlgorithm, c.RawTBSCertificateRequest, c.Signature)
}
