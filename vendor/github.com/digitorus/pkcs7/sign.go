package pkcs7

import (
	"bytes"
	"crypto"
	"crypto/dsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"fmt"
	"math/big"
	"time"
)

// SignedData is an opaque data structure for creating signed data payloads
type SignedData struct {
	sd                  signedData
	certs               []*x509.Certificate
	data, messageDigest []byte
	digestOid           asn1.ObjectIdentifier
	encryptionOid       asn1.ObjectIdentifier
}

// NewSignedData takes data and initializes a PKCS7 SignedData struct that is
// ready to be signed via AddSigner. The digest algorithm is set to SHA1 by default
// and can be changed by calling SetDigestAlgorithm.
func NewSignedData(data []byte) (*SignedData, error) {
	content, err := asn1.Marshal(data)
	if err != nil {
		return nil, err
	}
	ci := contentInfo{
		ContentType: OIDData,
		Content:     asn1.RawValue{Class: 2, Tag: 0, Bytes: content, IsCompound: true},
	}
	sd := signedData{
		ContentInfo: ci,
		Version:     1,
	}
	return &SignedData{sd: sd, data: data, digestOid: OIDDigestAlgorithmSHA1}, nil
}

// SignerInfoConfig are optional values to include when adding a signer
type SignerInfoConfig struct {
	ExtraSignedAttributes   []Attribute
	ExtraUnsignedAttributes []Attribute
	SkipCertificates        bool
}

type signedData struct {
	Version                    int                        `asn1:"default:1"`
	DigestAlgorithmIdentifiers []pkix.AlgorithmIdentifier `asn1:"set"`
	ContentInfo                contentInfo
	Certificates               rawCertificates        `asn1:"optional,tag:0"`
	CRLs                       []pkix.CertificateList `asn1:"optional,tag:1"`
	SignerInfos                []signerInfo           `asn1:"set"`
}

type signerInfo struct {
	Version                   int `asn1:"default:1"`
	IssuerAndSerialNumber     issuerAndSerial
	DigestAlgorithm           pkix.AlgorithmIdentifier
	AuthenticatedAttributes   []attribute `asn1:"optional,omitempty,tag:0"`
	DigestEncryptionAlgorithm pkix.AlgorithmIdentifier
	EncryptedDigest           []byte
	UnauthenticatedAttributes []attribute `asn1:"optional,omitempty,tag:1"`
}

type attribute struct {
	Type  asn1.ObjectIdentifier
	Value asn1.RawValue `asn1:"set"`
}

func marshalAttributes(attrs []attribute) ([]byte, error) {
	encodedAttributes, err := asn1.Marshal(struct {
		A []attribute `asn1:"set"`
	}{A: attrs})
	if err != nil {
		return nil, err
	}

	// Remove the leading sequence octets
	var raw asn1.RawValue
	asn1.Unmarshal(encodedAttributes, &raw)
	return raw.Bytes, nil
}

type rawCertificates struct {
	Raw asn1.RawContent
}

type issuerAndSerial struct {
	IssuerName   asn1.RawValue
	SerialNumber *big.Int
}

// SetDigestAlgorithm sets the digest algorithm to be used in the signing process.
//
// This should be called before adding signers
func (sd *SignedData) SetDigestAlgorithm(d asn1.ObjectIdentifier) {
	sd.digestOid = d
}

// SetEncryptionAlgorithm sets the encryption algorithm to be used in the signing process.
//
// This should be called before adding signers
func (sd *SignedData) SetEncryptionAlgorithm(d asn1.ObjectIdentifier) {
	sd.encryptionOid = d
}

// AddSigner is a wrapper around AddSignerChain() that adds a signer without any parent. The signer can
// either be a crypto.Signer or crypto.PrivateKey.
func (sd *SignedData) AddSigner(ee *x509.Certificate, keyOrSigner interface{}, config SignerInfoConfig) error {
	var parents []*x509.Certificate
	return sd.AddSignerChain(ee, keyOrSigner, parents, config)
}

// AddSignerChain signs attributes about the content and adds certificates
// and signers infos to the Signed Data. The certificate and private key
// of the end-entity signer are used to issue the signature, and any
// parent of that end-entity that need to be added to the list of
// certifications can be specified in the parents slice.
//
// The signature algorithm used to hash the data is the one of the end-entity
// certificate. The signer can be either a crypto.Signer or crypto.PrivateKey.
func (sd *SignedData) AddSignerChain(ee *x509.Certificate, keyOrSigner interface{}, parents []*x509.Certificate, config SignerInfoConfig) error {
	// Following RFC 2315, 9.2 SignerInfo type, the distinguished name of
	// the issuer of the end-entity signer is stored in the issuerAndSerialNumber
	// section of the SignedData.SignerInfo, alongside the serial number of
	// the end-entity.
	var ias issuerAndSerial
	ias.SerialNumber = ee.SerialNumber
	if len(parents) == 0 {
		// no parent, the issuer is the end-entity cert itself
		ias.IssuerName = asn1.RawValue{FullBytes: ee.RawIssuer}
	} else {
		err := verifyPartialChain(ee, parents)
		if err != nil {
			return err
		}
		// the first parent is the issuer
		ias.IssuerName = asn1.RawValue{FullBytes: parents[0].RawSubject}
	}
	sd.sd.DigestAlgorithmIdentifiers = append(sd.sd.DigestAlgorithmIdentifiers,
		pkix.AlgorithmIdentifier{Algorithm: sd.digestOid},
	)
	hash, err := getHashForOID(sd.digestOid)
	if err != nil {
		return err
	}
	h := hash.New()
	h.Write(sd.data)
	sd.messageDigest = h.Sum(nil)
	encryptionOid, err := getOIDForEncryptionAlgorithm(keyOrSigner, sd.digestOid)
	if err != nil {
		return err
	}
	attrs := &attributes{}
	attrs.Add(OIDAttributeContentType, sd.sd.ContentInfo.ContentType)
	attrs.Add(OIDAttributeMessageDigest, sd.messageDigest)
	attrs.Add(OIDAttributeSigningTime, time.Now().UTC())
	for _, attr := range config.ExtraSignedAttributes {
		attrs.Add(attr.Type, attr.Value)
	}
	finalAttrs, err := attrs.ForMarshalling()
	if err != nil {
		return err
	}
	unsignedAttrs := &attributes{}
	for _, attr := range config.ExtraUnsignedAttributes {
		unsignedAttrs.Add(attr.Type, attr.Value)
	}
	finalUnsignedAttrs, err := unsignedAttrs.ForMarshalling()
	if err != nil {
		return err
	}
	// create signature of signed attributes
	signature, err := signAttributes(finalAttrs, keyOrSigner, hash)
	if err != nil {
		return err
	}
	signerInfo := signerInfo{
		AuthenticatedAttributes:   finalAttrs,
		UnauthenticatedAttributes: finalUnsignedAttrs,
		DigestAlgorithm:           pkix.AlgorithmIdentifier{Algorithm: sd.digestOid},
		DigestEncryptionAlgorithm: pkix.AlgorithmIdentifier{Algorithm: encryptionOid},
		IssuerAndSerialNumber:     ias,
		EncryptedDigest:           signature,
		Version:                   1,
	}
	if !config.SkipCertificates {
		sd.certs = append(sd.certs, ee)
		if len(parents) > 0 {
			sd.certs = append(sd.certs, parents...)
		}
	}
	sd.sd.SignerInfos = append(sd.sd.SignerInfos, signerInfo)
	return nil
}

// SignWithoutAttr issues a signature on the content of the pkcs7 SignedData.
// Unlike AddSigner/AddSignerChain, it calculates the digest on the data alone
// and does not include any signed attributes like timestamp and so on.
//
// This function is needed to sign old Android APKs, something you probably
// shouldn't do unless you're maintaining backward compatibility for old
// applications. The signer can be either a crypto.Signer or crypto.PrivateKey.
func (sd *SignedData) SignWithoutAttr(ee *x509.Certificate, keyOrSigner interface{}, config SignerInfoConfig) error {
	var signature []byte
	sd.sd.DigestAlgorithmIdentifiers = append(sd.sd.DigestAlgorithmIdentifiers, pkix.AlgorithmIdentifier{Algorithm: sd.digestOid})
	hash, err := getHashForOID(sd.digestOid)
	if err != nil {
		return err
	}
	h := hash.New()
	h.Write(sd.data)
	sd.messageDigest = h.Sum(nil)

	switch pkey := keyOrSigner.(type) {
	case *dsa.PrivateKey:
		// dsa doesn't implement crypto.Signer so we make a special case
		// https://github.com/golang/go/issues/27889
		r, s, err := dsa.Sign(rand.Reader, pkey, sd.messageDigest)
		if err != nil {
			return err
		}
		signature, err = asn1.Marshal(dsaSignature{r, s})
		if err != nil {
			return err
		}
	default:
		signer, ok := keyOrSigner.(crypto.Signer)
		if !ok {
			return errors.New("pkcs7: private key does not implement crypto.Signer")
		}

		// special case for Ed25519, which hashes as part of the signing algorithm
		_, ok = signer.Public().(ed25519.PublicKey)
		if ok {
			signature, err = signer.Sign(rand.Reader, sd.data, crypto.Hash(0))
		} else {
			signature, err = signer.Sign(rand.Reader, sd.messageDigest, hash)
			if err != nil {
				return err
			}
		}
	}

	var ias issuerAndSerial
	ias.SerialNumber = ee.SerialNumber
	// no parent, the issue is the end-entity cert itself
	ias.IssuerName = asn1.RawValue{FullBytes: ee.RawIssuer}
	if sd.encryptionOid == nil {
		// if the encryption algorithm wasn't set by SetEncryptionAlgorithm,
		// infer it from the digest algorithm
		sd.encryptionOid, err = getOIDForEncryptionAlgorithm(keyOrSigner, sd.digestOid)
	}
	if err != nil {
		return err
	}
	signerInfo := signerInfo{
		DigestAlgorithm:           pkix.AlgorithmIdentifier{Algorithm: sd.digestOid},
		DigestEncryptionAlgorithm: pkix.AlgorithmIdentifier{Algorithm: sd.encryptionOid},
		IssuerAndSerialNumber:     ias,
		EncryptedDigest:           signature,
		Version:                   1,
	}
	// create signature of signed attributes
	sd.certs = append(sd.certs, ee)
	sd.sd.SignerInfos = append(sd.sd.SignerInfos, signerInfo)
	return nil
}

func (si *signerInfo) SetUnauthenticatedAttributes(extraUnsignedAttrs []Attribute) error {
	unsignedAttrs := &attributes{}
	for _, attr := range extraUnsignedAttrs {
		unsignedAttrs.Add(attr.Type, attr.Value)
	}
	finalUnsignedAttrs, err := unsignedAttrs.ForMarshalling()
	if err != nil {
		return err
	}

	si.UnauthenticatedAttributes = finalUnsignedAttrs

	return nil
}

// AddCertificate adds the certificate to the payload. Useful for parent certificates
func (sd *SignedData) AddCertificate(cert *x509.Certificate) {
	sd.certs = append(sd.certs, cert)
}

// SetContentType sets the content type of the SignedData. For example to specify the
// content type of a time-stamp token according to RFC 3161 section 2.4.2.
func (sd *SignedData) SetContentType(contentType asn1.ObjectIdentifier) {
	sd.sd.ContentInfo.ContentType = contentType
}

// Detach removes content from the signed data struct to make it a detached signature.
// This must be called right before Finish()
func (sd *SignedData) Detach() {
	sd.sd.ContentInfo = contentInfo{ContentType: OIDData}
}

// GetSignedData returns the private Signed Data
func (sd *SignedData) GetSignedData() *signedData {
	return &sd.sd
}

// Finish marshals the content and its signers
func (sd *SignedData) Finish() ([]byte, error) {
	sd.sd.Certificates = marshalCertificates(sd.certs)
	inner, err := asn1.Marshal(sd.sd)
	if err != nil {
		return nil, err
	}
	outer := contentInfo{
		ContentType: OIDSignedData,
		Content:     asn1.RawValue{Class: 2, Tag: 0, Bytes: inner, IsCompound: true},
	}
	return asn1.Marshal(outer)
}

// RemoveAuthenticatedAttributes removes authenticated attributes from signedData
// similar to OpenSSL's PKCS7_NOATTR or -noattr flags
func (sd *SignedData) RemoveAuthenticatedAttributes() {
	for i := range sd.sd.SignerInfos {
		sd.sd.SignerInfos[i].AuthenticatedAttributes = nil
	}
}

// RemoveUnauthenticatedAttributes removes unauthenticated attributes from signedData
func (sd *SignedData) RemoveUnauthenticatedAttributes() {
	for i := range sd.sd.SignerInfos {
		sd.sd.SignerInfos[i].UnauthenticatedAttributes = nil
	}
}

// verifyPartialChain checks that a given cert is issued by the first parent in the list,
// then continue down the path. It doesn't require the last parent to be a root CA,
// or to be trusted in any truststore. It simply verifies that the chain provided, albeit
// partial, makes sense.
func verifyPartialChain(cert *x509.Certificate, parents []*x509.Certificate) error {
	if len(parents) == 0 {
		return fmt.Errorf("pkcs7: zero parents provided to verify the signature of certificate %q", cert.Subject.CommonName)
	}
	err := cert.CheckSignatureFrom(parents[0])
	if err != nil {
		return fmt.Errorf("pkcs7: certificate signature from parent is invalid: %v", err)
	}
	if len(parents) == 1 {
		// there is no more parent to check, return
		return nil
	}
	return verifyPartialChain(parents[0], parents[1:])
}

func cert2issuerAndSerial(cert *x509.Certificate) (issuerAndSerial, error) {
	var ias issuerAndSerial
	// The issuer RDNSequence has to match exactly the sequence in the certificate
	// We cannot use cert.Issuer.ToRDNSequence() here since it mangles the sequence
	ias.IssuerName = asn1.RawValue{FullBytes: cert.RawIssuer}
	ias.SerialNumber = cert.SerialNumber

	return ias, nil
}

// signs the DER encoded form of the attributes with the private key
func signAttributes(attrs []attribute, keyOrSigner interface{}, digestAlg crypto.Hash) ([]byte, error) {
	attrBytes, err := marshalAttributes(attrs)
	if err != nil {
		return nil, err
	}
	h := digestAlg.New()
	h.Write(attrBytes)
	hash := h.Sum(nil)

	// dsa doesn't implement crypto.Signer so we make a special case
	// https://github.com/golang/go/issues/27889
	switch pkey := keyOrSigner.(type) {
	case *dsa.PrivateKey:
		r, s, err := dsa.Sign(rand.Reader, pkey, hash)
		if err != nil {
			return nil, err
		}
		return asn1.Marshal(dsaSignature{r, s})
	}

	signer, ok := keyOrSigner.(crypto.Signer)
	if !ok {
		return nil, errors.New("pkcs7: private key does not implement crypto.Signer")
	}

	// special case for Ed25519, which hashes as part of the signing algorithm
	_, ok = signer.Public().(ed25519.PublicKey)
	if ok {
		return signer.Sign(rand.Reader, attrBytes, crypto.Hash(0))
	}

	return signer.Sign(rand.Reader, hash, digestAlg)
}

type dsaSignature struct {
	R, S *big.Int
}

// concats and wraps the certificates in the RawValue structure
func marshalCertificates(certs []*x509.Certificate) rawCertificates {
	var buf bytes.Buffer
	for _, cert := range certs {
		buf.Write(cert.Raw)
	}
	rawCerts, _ := marshalCertificateBytes(buf.Bytes())
	return rawCerts
}

// Even though, the tag & length are stripped out during marshalling the
// RawContent, we have to encode it into the RawContent. If its missing,
// then `asn1.Marshal()` will strip out the certificate wrapper instead.
func marshalCertificateBytes(certs []byte) (rawCertificates, error) {
	var val = asn1.RawValue{Bytes: certs, Class: 2, Tag: 0, IsCompound: true}
	b, err := asn1.Marshal(val)
	if err != nil {
		return rawCertificates{}, err
	}
	return rawCertificates{Raw: b}, nil
}

// DegenerateCertificate creates a signed data structure containing only the
// provided certificate or certificate chain.
func DegenerateCertificate(cert []byte) ([]byte, error) {
	rawCert, err := marshalCertificateBytes(cert)
	if err != nil {
		return nil, err
	}
	emptyContent := contentInfo{ContentType: OIDData}
	sd := signedData{
		Version:      1,
		ContentInfo:  emptyContent,
		Certificates: rawCert,
		CRLs:         []pkix.CertificateList{},
	}
	content, err := asn1.Marshal(sd)
	if err != nil {
		return nil, err
	}
	signedContent := contentInfo{
		ContentType: OIDSignedData,
		Content:     asn1.RawValue{Class: 2, Tag: 0, Bytes: content, IsCompound: true},
	}
	return asn1.Marshal(signedContent)
}
