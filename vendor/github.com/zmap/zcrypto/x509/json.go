// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/zmap/zcrypto/dsa"
	"github.com/zmap/zcrypto/encoding/asn1"
	jsonKeys "github.com/zmap/zcrypto/json"
	"github.com/zmap/zcrypto/util"
	"github.com/zmap/zcrypto/x509/pkix"
)

var kMinTime, kMaxTime time.Time

func init() {
	var err error
	kMinTime, err = time.Parse(time.RFC3339, "0001-01-01T00:00:00Z")
	if err != nil {
		panic(err)
	}
	kMaxTime, err = time.Parse(time.RFC3339, "9999-12-31T23:59:59Z")
	if err != nil {
		panic(err)
	}
}

type auxKeyUsage struct {
	DigitalSignature  bool   `json:"digital_signature,omitempty"`
	ContentCommitment bool   `json:"content_commitment,omitempty"`
	KeyEncipherment   bool   `json:"key_encipherment,omitempty"`
	DataEncipherment  bool   `json:"data_encipherment,omitempty"`
	KeyAgreement      bool   `json:"key_agreement,omitempty"`
	CertificateSign   bool   `json:"certificate_sign,omitempty"`
	CRLSign           bool   `json:"crl_sign,omitempty"`
	EncipherOnly      bool   `json:"encipher_only,omitempty"`
	DecipherOnly      bool   `json:"decipher_only,omitempty"`
	Value             uint32 `json:"value"`
}

// MarshalJSON implements the json.Marshaler interface
func (k KeyUsage) MarshalJSON() ([]byte, error) {
	var enc auxKeyUsage
	enc.Value = uint32(k)
	if k&KeyUsageDigitalSignature > 0 {
		enc.DigitalSignature = true
	}
	if k&KeyUsageContentCommitment > 0 {
		enc.ContentCommitment = true
	}
	if k&KeyUsageKeyEncipherment > 0 {
		enc.KeyEncipherment = true
	}
	if k&KeyUsageDataEncipherment > 0 {
		enc.DataEncipherment = true
	}
	if k&KeyUsageKeyAgreement > 0 {
		enc.KeyAgreement = true
	}
	if k&KeyUsageCertSign > 0 {
		enc.CertificateSign = true
	}
	if k&KeyUsageCRLSign > 0 {
		enc.CRLSign = true
	}
	if k&KeyUsageEncipherOnly > 0 {
		enc.EncipherOnly = true
	}
	if k&KeyUsageDecipherOnly > 0 {
		enc.DecipherOnly = true
	}
	return json.Marshal(&enc)
}

// UnmarshalJSON implements the json.Unmarshler interface
func (k *KeyUsage) UnmarshalJSON(b []byte) error {
	var aux auxKeyUsage
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	// TODO: validate the flags match
	v := int(aux.Value)
	*k = KeyUsage(v)
	return nil
}

// JSONSignatureAlgorithm is the intermediate type
// used when marshaling a PublicKeyAlgorithm out to JSON.
type JSONSignatureAlgorithm struct {
	Name string      `json:"name,omitempty"`
	OID  pkix.AuxOID `json:"oid"`
}

// MarshalJSON implements the json.Marshaler interface
// MAY NOT PRESERVE ORIGINAL OID FROM CERTIFICATE -
// CONSIDER USING jsonifySignatureAlgorithm INSTEAD!
func (s *SignatureAlgorithm) MarshalJSON() ([]byte, error) {
	aux := JSONSignatureAlgorithm{
		Name: s.String(),
	}
	for _, val := range signatureAlgorithmDetails {
		if val.algo == *s {
			aux.OID = make([]int, len(val.oid))
			for idx := range val.oid {
				aux.OID[idx] = val.oid[idx]
			}
		}
	}
	return json.Marshal(&aux)
}

// UnmarshalJSON implements the json.Unmarshler interface
func (s *SignatureAlgorithm) UnmarshalJSON(b []byte) error {
	var aux JSONSignatureAlgorithm
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	*s = UnknownSignatureAlgorithm
	oid := asn1.ObjectIdentifier(aux.OID.AsSlice())
	if oid.Equal(oidSignatureRSAPSS) {
		pssAlgs := []SignatureAlgorithm{SHA256WithRSAPSS, SHA384WithRSAPSS, SHA512WithRSAPSS}
		for _, alg := range pssAlgs {
			if strings.Compare(alg.String(), aux.Name) == 0 {
				*s = alg
				break
			}
		}
	} else {
		for _, val := range signatureAlgorithmDetails {
			if val.oid.Equal(oid) {
				*s = val.algo
				break
			}
		}
	}
	return nil
}

// jsonifySignatureAlgorithm gathers the necessary fields in a Certificate
// into a JSONSignatureAlgorithm, which can then use the default
// JSON marhsalers and unmarshalers. THIS FUNCTION IS PREFERED OVER
// THE CUSTOM JSON MARSHALER PRESENTED ABOVE FOR SIGNATUREALGORITHM
// BECAUSE THIS METHOD PRESERVES THE OID ORIGINALLY IN THE CERTIFICATE!
// This reason also explains why we need this function -
// the OID is unfortunately stored outside the scope of a
// SignatureAlgorithm struct and cannot be recovered without access to the
// entire Certificate if we do not know the signature algorithm.
func (c *Certificate) jsonifySignatureAlgorithm() JSONSignatureAlgorithm {
	aux := JSONSignatureAlgorithm{}
	if c.SignatureAlgorithm == 0 {
		aux.Name = "unknown_algorithm"
	} else {
		aux.Name = c.SignatureAlgorithm.String()
	}
	aux.OID = make([]int, len(c.SignatureAlgorithmOID))
	for idx := range c.SignatureAlgorithmOID {
		aux.OID[idx] = c.SignatureAlgorithmOID[idx]
	}
	return aux
}

type auxPublicKeyAlgorithm struct {
	Name string       `json:"name,omitempty"`
	OID  *pkix.AuxOID `json:"oid,omitempty"`
}

var publicKeyNameToAlgorithm = map[string]PublicKeyAlgorithm{
	"RSA":   RSA,
	"DSA":   DSA,
	"ECDSA": ECDSA,
}

// MarshalJSON implements the json.Marshaler interface
func (p *PublicKeyAlgorithm) MarshalJSON() ([]byte, error) {
	aux := auxPublicKeyAlgorithm{
		Name: p.String(),
	}
	return json.Marshal(&aux)
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (p *PublicKeyAlgorithm) UnmarshalJSON(b []byte) error {
	var aux auxPublicKeyAlgorithm
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	*p = publicKeyNameToAlgorithm[aux.Name]
	return nil
}

func clampTime(t time.Time) time.Time {
	if t.Before(kMinTime) {
		return kMinTime
	}
	if t.After(kMaxTime) {
		return kMaxTime
	}
	return t
}

type auxValidity struct {
	Start          string `json:"start"`
	End            string `json:"end"`
	ValidityPeriod int    `json:"length"`
}

func (v *validity) MarshalJSON() ([]byte, error) {
	aux := auxValidity{
		Start:          clampTime(v.NotBefore.UTC()).Format(time.RFC3339),
		End:            clampTime(v.NotAfter.UTC()).Format(time.RFC3339),
		ValidityPeriod: int(v.NotAfter.Unix() - v.NotBefore.Unix()),
	}
	return json.Marshal(&aux)
}

func (v *validity) UnmarshalJSON(b []byte) error {
	var aux auxValidity
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	var err error
	if v.NotBefore, err = time.Parse(time.RFC3339, aux.Start); err != nil {
		return err
	}
	if v.NotAfter, err = time.Parse(time.RFC3339, aux.End); err != nil {
		return err
	}
	return nil
}

// ECDSAPublicKeyJSON - used to condense several fields from a
// ECDSA public key into one field for use in JSONCertificate.
// Uses default JSON marshal and unmarshal methods
type ECDSAPublicKeyJSON struct {
	B      []byte `json:"b"`
	Curve  string `json:"curve"`
	Gx     []byte `json:"gx"`
	Gy     []byte `json:"gy"`
	Length int    `json:"length"`
	N      []byte `json:"n"`
	P      []byte `json:"p"`
	Pub    []byte `json:"pub,omitempty"`
	X      []byte `json:"x"`
	Y      []byte `json:"y"`
}

// DSAPublicKeyJSON - used to condense several fields from a
// DSA public key into one field for use in JSONCertificate.
// Uses default JSON marshal and unmarshal methods
type DSAPublicKeyJSON struct {
	G []byte `json:"g"`
	P []byte `json:"p"`
	Q []byte `json:"q"`
	Y []byte `json:"y"`
}

// GetDSAPublicKeyJSON - get the DSAPublicKeyJSON for the given standard DSA PublicKey.
func GetDSAPublicKeyJSON(key *dsa.PublicKey) *DSAPublicKeyJSON {
	return &DSAPublicKeyJSON{
		P: key.P.Bytes(),
		Q: key.Q.Bytes(),
		G: key.G.Bytes(),
		Y: key.Y.Bytes(),
	}
}

// GetRSAPublicKeyJSON - get the jsonKeys.RSAPublicKey for the given standard RSA PublicKey.
func GetRSAPublicKeyJSON(key *rsa.PublicKey) *jsonKeys.RSAPublicKey {
	rsaKey := new(jsonKeys.RSAPublicKey)
	rsaKey.PublicKey = key
	return rsaKey
}

// GetECDSAPublicKeyJSON - get the GetECDSAPublicKeyJSON for the given standard ECDSA PublicKey.
func GetECDSAPublicKeyJSON(key *ecdsa.PublicKey) *ECDSAPublicKeyJSON {
	params := key.Params()
	return &ECDSAPublicKeyJSON{
		P:      params.P.Bytes(),
		N:      params.N.Bytes(),
		B:      params.B.Bytes(),
		Gx:     params.Gx.Bytes(),
		Gy:     params.Gy.Bytes(),
		X:      key.X.Bytes(),
		Y:      key.Y.Bytes(),
		Curve:  key.Curve.Params().Name,
		Length: key.Curve.Params().BitSize,
	}
}

// GetAugmentedECDSAPublicKeyJSON - get the GetECDSAPublicKeyJSON for the given "augmented"
// ECDSA PublicKey.
func GetAugmentedECDSAPublicKeyJSON(key *AugmentedECDSA) *ECDSAPublicKeyJSON {
	params := key.Pub.Params()
	return &ECDSAPublicKeyJSON{
		P:      params.P.Bytes(),
		N:      params.N.Bytes(),
		B:      params.B.Bytes(),
		Gx:     params.Gx.Bytes(),
		Gy:     params.Gy.Bytes(),
		X:      key.Pub.X.Bytes(),
		Y:      key.Pub.Y.Bytes(),
		Curve:  key.Pub.Curve.Params().Name,
		Length: key.Pub.Curve.Params().BitSize,
		Pub:    key.Raw.Bytes,
	}
}

// jsonifySubjectKey - Convert public key data in a Certificate
// into json output format for JSONCertificate
func (c *Certificate) jsonifySubjectKey() JSONSubjectKeyInfo {
	j := JSONSubjectKeyInfo{
		KeyAlgorithm:    c.PublicKeyAlgorithm,
		SPKIFingerprint: c.SPKIFingerprint,
	}

	switch key := c.PublicKey.(type) {
	case *rsa.PublicKey:
		rsaKey := new(jsonKeys.RSAPublicKey)
		rsaKey.PublicKey = key
		j.RSAPublicKey = rsaKey
	case *dsa.PublicKey:
		j.DSAPublicKey = &DSAPublicKeyJSON{
			P: key.P.Bytes(),
			Q: key.Q.Bytes(),
			G: key.G.Bytes(),
			Y: key.Y.Bytes(),
		}
	case *ecdsa.PublicKey:
		params := key.Params()
		j.ECDSAPublicKey = &ECDSAPublicKeyJSON{
			P:      params.P.Bytes(),
			N:      params.N.Bytes(),
			B:      params.B.Bytes(),
			Gx:     params.Gx.Bytes(),
			Gy:     params.Gy.Bytes(),
			X:      key.X.Bytes(),
			Y:      key.Y.Bytes(),
			Curve:  key.Curve.Params().Name,
			Length: key.Curve.Params().BitSize,
		}
	case *AugmentedECDSA:
		params := key.Pub.Params()
		j.ECDSAPublicKey = &ECDSAPublicKeyJSON{
			P:      params.P.Bytes(),
			N:      params.N.Bytes(),
			B:      params.B.Bytes(),
			Gx:     params.Gx.Bytes(),
			Gy:     params.Gy.Bytes(),
			X:      key.Pub.X.Bytes(),
			Y:      key.Pub.Y.Bytes(),
			Curve:  key.Pub.Curve.Params().Name,
			Length: key.Pub.Curve.Params().BitSize,
			Pub:    key.Raw.Bytes,
		}
	}
	return j
}

// JSONSubjectKeyInfo - used to condense several fields from x509.Certificate
// related to the subject public key into one field within JSONCertificate
// Unfortunately, this struct cannot have its own Marshal method since it
// needs information from multiple fields in x509.Certificate
type JSONSubjectKeyInfo struct {
	KeyAlgorithm    PublicKeyAlgorithm     `json:"key_algorithm"`
	RSAPublicKey    *jsonKeys.RSAPublicKey `json:"rsa_public_key,omitempty"`
	DSAPublicKey    *DSAPublicKeyJSON      `json:"dsa_public_key,omitempty"`
	ECDSAPublicKey  *ECDSAPublicKeyJSON    `json:"ecdsa_public_key,omitempty"`
	SPKIFingerprint CertificateFingerprint `json:"fingerprint_sha256"`
}

// JSONSignature - used to condense several fields from x509.Certificate
// related to the signature into one field within JSONCertificate
// Unfortunately, this struct cannot have its own Marshal method since it
// needs information from multiple fields in x509.Certificate
type JSONSignature struct {
	SignatureAlgorithm JSONSignatureAlgorithm `json:"signature_algorithm"`
	Value              []byte                 `json:"value"`
	Valid              bool                   `json:"valid"`
	SelfSigned         bool                   `json:"self_signed"`
}

// JSONValidity - used to condense several fields related
// to validity in x509.Certificate into one field within JSONCertificate
// Unfortunately, this struct cannot have its own Marshal method since it
// needs information from multiple fields in x509.Certificate
type JSONValidity struct {
	validity
	ValidityPeriod int
}

// JSONCertificate - used to condense data from x509.Certificate when marhsaling
// into JSON. This struct has a distinct and independent layout from
// x509.Certificate, mostly for condensing data across repetitive
// fields and making it more presentable.
type JSONCertificate struct {
	Version                   int                          `json:"version"`
	SerialNumber              string                       `json:"serial_number"`
	SignatureAlgorithm        JSONSignatureAlgorithm       `json:"signature_algorithm"`
	Issuer                    pkix.Name                    `json:"issuer"`
	IssuerDN                  string                       `json:"issuer_dn,omitempty"`
	Validity                  JSONValidity                 `json:"validity"`
	Subject                   pkix.Name                    `json:"subject"`
	SubjectDN                 string                       `json:"subject_dn,omitempty"`
	SubjectKeyInfo            JSONSubjectKeyInfo           `json:"subject_key_info"`
	Extensions                *CertificateExtensions       `json:"extensions,omitempty"`
	UnknownExtensions         UnknownCertificateExtensions `json:"unknown_extensions,omitempty"`
	Signature                 JSONSignature                `json:"signature"`
	FingerprintMD5            CertificateFingerprint       `json:"fingerprint_md5"`
	FingerprintSHA1           CertificateFingerprint       `json:"fingerprint_sha1"`
	FingerprintSHA256         CertificateFingerprint       `json:"fingerprint_sha256"`
	FingerprintNoCT           CertificateFingerprint       `json:"tbs_noct_fingerprint"`
	SPKISubjectFingerprint    CertificateFingerprint       `json:"spki_subject_fingerprint"`
	TBSCertificateFingerprint CertificateFingerprint       `json:"tbs_fingerprint"`
	ValidationLevel           CertValidationLevel          `json:"validation_level"`
	Names                     []string                     `json:"names,omitempty"`
	Redacted                  bool                         `json:"redacted"`
}

// CollectAllNames - Collect and validate all DNS / URI / IP Address names for a given certificate
func (c *Certificate) CollectAllNames() []string {
	var names []string

	if isValidName(c.Subject.CommonName) {
		names = append(names, c.Subject.CommonName)
	}

	for _, name := range c.DNSNames {
		if isValidName(name) {
			names = append(names, name)
		} else if !strings.Contains(name, ".") { //just a TLD
			names = append(names, name)
		}

	}

	for _, name := range c.URIs {
		if util.IsURL(name) {
			names = append(names, name)
		}
	}

	for _, name := range c.IPAddresses {
		str := name.String()
		if util.IsURL(str) {
			names = append(names, str)
		}
	}

	return purgeNameDuplicates(names)
}

func (c *Certificate) MarshalJSON() ([]byte, error) {
	// Fill out the certificate
	jc := new(JSONCertificate)
	jc.Version = c.Version
	jc.SerialNumber = c.SerialNumber.String()
	jc.Issuer = c.Issuer
	jc.IssuerDN = c.Issuer.String()

	jc.Validity.NotBefore = c.NotBefore
	jc.Validity.NotAfter = c.NotAfter
	jc.Validity.ValidityPeriod = c.ValidityPeriod
	jc.Subject = c.Subject
	jc.SubjectDN = c.Subject.String()
	jc.Names = c.CollectAllNames()
	jc.Redacted = false
	for _, name := range jc.Names {
		if strings.HasPrefix(name, "?") {
			jc.Redacted = true
		}
	}

	jc.SubjectKeyInfo = c.jsonifySubjectKey()
	jc.Extensions, jc.UnknownExtensions = c.jsonifyExtensions()

	// TODO: Handle the fact this might not match
	jc.SignatureAlgorithm = c.jsonifySignatureAlgorithm()
	jc.Signature.SignatureAlgorithm = jc.SignatureAlgorithm
	jc.Signature.Value = c.Signature
	jc.Signature.Valid = c.validSignature
	jc.Signature.SelfSigned = c.SelfSigned
	if c.SelfSigned {
		jc.Signature.Valid = true
	}
	jc.FingerprintMD5 = c.FingerprintMD5
	jc.FingerprintSHA1 = c.FingerprintSHA1
	jc.FingerprintSHA256 = c.FingerprintSHA256
	jc.FingerprintNoCT = c.FingerprintNoCT
	jc.SPKISubjectFingerprint = c.SPKISubjectFingerprint
	jc.TBSCertificateFingerprint = c.TBSCertificateFingerprint
	jc.ValidationLevel = c.ValidationLevel

	return json.Marshal(jc)
}

// UnmarshalJSON - intentionally implimented to always error,
// as this method should not be used. The MarshalJSON method
// on Certificate condenses data in a way that is not recoverable.
// Use the x509.ParseCertificate function instead or
// JSONCertificateWithRaw Marshal method
func (jc *JSONCertificate) UnmarshalJSON(b []byte) error {
	return errors.New("Do not unmarshal cert JSON directly, use JSONCertificateWithRaw or x509.ParseCertificate function")
}

// UnmarshalJSON - intentionally implimented to always error,
// as this method should not be used. The MarshalJSON method
// on Certificate condenses data in a way that is not recoverable.
// Use the x509.ParseCertificate function instead or
// JSONCertificateWithRaw Marshal method
func (c *Certificate) UnmarshalJSON(b []byte) error {
	return errors.New("Do not unmarshal cert JSON directly, use JSONCertificateWithRaw or x509.ParseCertificate function")
}

// JSONCertificateWithRaw - intermediate struct for unmarshaling json
// of a certificate - the raw is require since the
// MarshalJSON method on Certificate condenses data in a way that
// makes extraction to the original in Unmarshal impossible.
// The JSON output of Marshal is not even used to construct
// a certificate, all we need is raw
type JSONCertificateWithRaw struct {
	Raw []byte `json:"raw,omitempty"`
}

// ParseRaw - for converting the intermediate object
// JSONCertificateWithRaw into a parsed Certificate
// see description of JSONCertificateWithRaw for
// why this is used instead of UnmarshalJSON methods
func (c *JSONCertificateWithRaw) ParseRaw() (*Certificate, error) {
	return ParseCertificate(c.Raw)
}

func purgeNameDuplicates(names []string) (out []string) {
	hashset := make(map[string]bool, len(names))
	for _, name := range names {
		if _, inc := hashset[name]; !inc {
			hashset[name] = true
		}
	}

	out = make([]string, 0, len(hashset))
	for key := range hashset {
		out = append(out, key)
	}

	sort.Strings(out) // must sort to ensure output is deterministic!
	return
}

func isValidName(name string) (ret bool) {

	// Check for wildcards and redacts, ignore malformed urls
	if strings.HasPrefix(name, "?.") || strings.HasPrefix(name, "*.") {
		ret = isValidName(name[2:])
	} else {
		ret = util.IsURL(name)
	}
	return
}

func orMask(ip net.IP, mask net.IPMask) net.IP {
	if len(ip) == 0 || len(mask) == 0 {
		return nil
	}
	if len(ip) != net.IPv4len && len(ip) != net.IPv6len {
		return nil
	}
	if len(ip) != len(mask) {
		return nil
	}
	out := make([]byte, len(ip))
	for idx := range ip {
		out[idx] = ip[idx] | mask[idx]
	}
	return out
}

func invertMask(mask net.IPMask) net.IPMask {
	if mask == nil {
		return nil
	}
	out := make([]byte, len(mask))
	for idx := range mask {
		out[idx] = ^mask[idx]
	}
	return out
}

type auxGeneralSubtreeIP struct {
	CIDR  string `json:"cidr,omitempty"`
	Begin string `json:"begin,omitempty"`
	End   string `json:"end,omitempty"`
	Mask  string `json:"mask,omitempty"`
}

func (g *GeneralSubtreeIP) MarshalJSON() ([]byte, error) {
	aux := auxGeneralSubtreeIP{}
	aux.CIDR = g.Data.String()
	// Check to see if the subnet is valid. An invalid subnet will return 0,0
	// from Size(). If the subnet is invalid, only output the CIDR.
	ones, bits := g.Data.Mask.Size()
	if ones == 0 && bits == 0 {
		return json.Marshal(&aux)
	}
	// The first IP in the range should be `ip & mask`.
	begin := g.Data.IP.Mask(g.Data.Mask)
	if begin != nil {
		aux.Begin = begin.String()
	}
	// The last IP (inclusive) is `ip & (^mask)`.
	inverseMask := invertMask(g.Data.Mask)
	end := orMask(g.Data.IP, inverseMask)
	if end != nil {
		aux.End = end.String()
	}
	// Output the mask as an IP, but enforce it can be formatted correctly.
	// net.IP.String() only works on byte arrays of the correct length.
	maskLen := len(g.Data.Mask)
	if maskLen == net.IPv4len || maskLen == net.IPv6len {
		maskAsIP := net.IP(g.Data.Mask)
		aux.Mask = maskAsIP.String()
	}
	return json.Marshal(&aux)
}

func (g *GeneralSubtreeIP) UnmarshalJSON(b []byte) error {
	aux := auxGeneralSubtreeIP{}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	ip, ipNet, err := net.ParseCIDR(aux.CIDR)
	if err != nil {
		return err
	}
	g.Data.IP = ip
	g.Data.Mask = ipNet.Mask
	g.Min = 0
	g.Max = 0
	return nil
}
