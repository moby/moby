package initca

import (
	"crypto"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/cloudflare/cfssl/csr"
	cferr "github.com/cloudflare/cfssl/errors"
	"github.com/cloudflare/cfssl/helpers"
	"golang.org/x/crypto/cryptobyte"
	cbasn1 "golang.org/x/crypto/cryptobyte/asn1"
)

// idAlgUnsigned is id-alg-unsigned (RFC 9925, Section 3.1).
var idAlgUnsigned = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 6, 36}

// Unsign converts a PEM-encoded certificate into an RFC 9925 unsigned
// certificate: TBSCertificate.signature and signatureAlgorithm become
// id-alg-unsigned with absent parameters and signatureValue becomes an empty
// BIT STRING. The authority key identifier and issuer alternative name
// extensions are removed, as RFC 9925, Section 3.3 says senders should omit
// them. All other fields, including the issuer, are preserved byte for byte.
//
// The input can be a root, an intermediate or an end-entity certificate. The
// input signature is not verified, so the caller is responsible for deciding
// that the certificate is trusted, for example because it is about to be
// installed directly as a trust anchor. The input must parse as an X.509
// version 3 certificate. A certificate whose issuer has no attributes is
// rejected, because X.509 and RFC 5280 forbid an empty issuer
// (RFC 9925, Section 3.2). A certificate with an issuerUniqueID is rejected,
// because RFC 9925, Section 3.2 requires senders to omit it.
func Unsign(certPEM []byte) ([]byte, error) {
	block, rest := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" || len(rest) != 0 {
		return nil, cferr.Wrap(cferr.CertificateError, cferr.DecodeFailed,
			errors.New("expected a single CERTIFICATE PEM block"))
	}
	input, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, cferr.Wrap(cferr.CertificateError, cferr.ParseFailed, err)
	}
	if input.Version != 3 {
		return nil, cferr.Wrap(cferr.CertificateError, cferr.ParseFailed,
			fmt.Errorf("certificate is version %d, want version 3", input.Version))
	}
	der, err := unsignDER(block.Bytes)
	if err != nil {
		return nil, cferr.Wrap(cferr.CertificateError, cferr.ParseFailed, err)
	}
	if _, err := x509.ParseCertificate(der); err != nil {
		return nil, cferr.Wrap(cferr.CertificateError, cferr.ParseFailed, err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), nil
}

func unsignDER(der []byte) ([]byte, error) {
	input := cryptobyte.String(der)
	var cert, tbs cryptobyte.String
	if !input.ReadASN1(&cert, cbasn1.SEQUENCE) || !input.Empty() ||
		!cert.ReadASN1(&tbs, cbasn1.SEQUENCE) ||
		!cert.SkipASN1(cbasn1.SEQUENCE) ||
		!cert.SkipASN1(cbasn1.BIT_STRING) || !cert.Empty() {
		return nil, errors.New("malformed certificate")
	}

	var version, serial, sigAlg, issuer, validity, subject, spki cryptobyte.String
	if tbs.PeekASN1Tag(cbasn1.Tag(0).Constructed().ContextSpecific()) &&
		!tbs.ReadASN1Element(&version, cbasn1.Tag(0).Constructed().ContextSpecific()) {
		return nil, errMalformedTBSCert
	}
	if !tbs.ReadASN1Element(&serial, cbasn1.INTEGER) ||
		!tbs.ReadASN1Element(&sigAlg, cbasn1.SEQUENCE) ||
		!tbs.ReadASN1Element(&issuer, cbasn1.SEQUENCE) ||
		!tbs.ReadASN1Element(&validity, cbasn1.SEQUENCE) ||
		!tbs.ReadASN1Element(&subject, cbasn1.SEQUENCE) ||
		!tbs.ReadASN1Element(&spki, cbasn1.SEQUENCE) {
		return nil, errMalformedTBSCert
	}
	if err := csr.ValidateRawName(issuer); err != nil {
		return nil, fmt.Errorf("issuer: %w", err)
	}
	if tbs.PeekASN1Tag(cbasn1.Tag(1).ContextSpecific()) {
		return nil, errors.New("certificate has an issuerUniqueID")
	}

	var subjectUniqueID cryptobyte.String
	if tbs.PeekASN1Tag(cbasn1.Tag(2).ContextSpecific()) &&
		!tbs.ReadASN1Element(&subjectUniqueID, cbasn1.Tag(2).ContextSpecific()) {
		return nil, errMalformedTBSCert
	}
	extensions, err := keepExtensions(tbs)
	if err != nil {
		return nil, err
	}

	var b cryptobyte.Builder
	b.AddASN1(cbasn1.SEQUENCE, func(c *cryptobyte.Builder) {
		c.AddASN1(cbasn1.SEQUENCE, func(t *cryptobyte.Builder) {
			t.AddBytes(version)
			t.AddBytes(serial)
			addUnsignedAlgorithm(t)
			t.AddBytes(issuer)
			t.AddBytes(validity)
			t.AddBytes(subject)
			t.AddBytes(spki)
			t.AddBytes(subjectUniqueID)
			if len(extensions) > 0 {
				t.AddASN1(cbasn1.Tag(3).Constructed().ContextSpecific(), func(e *cryptobyte.Builder) {
					e.AddASN1(cbasn1.SEQUENCE, func(list *cryptobyte.Builder) {
						for _, extension := range extensions {
							list.AddBytes(extension)
						}
					})
				})
			}
		})
		addUnsignedAlgorithm(c)
		c.AddASN1BitString(nil)
	})
	return b.Bytes()
}

var (
	oidIssuerAltName    = asn1.ObjectIdentifier{2, 5, 29, 18}
	oidAuthorityKeyID   = asn1.ObjectIdentifier{2, 5, 29, 35}
	extensionsTag       = cbasn1.Tag(3).Constructed().ContextSpecific()
	errMalformedTBSCert = errors.New("malformed TBSCertificate")
)

// keepExtensions returns the encoded extensions of the remainder of a
// TBSCertificate, without the authority key identifier and issuer alternative
// name extensions. It returns no extensions if none are left.
func keepExtensions(rest cryptobyte.String) ([][]byte, error) {
	if rest.Empty() {
		return nil, nil
	}
	var wrapper, list cryptobyte.String
	if !rest.ReadASN1(&wrapper, extensionsTag) || !rest.Empty() ||
		!wrapper.ReadASN1(&list, cbasn1.SEQUENCE) || !wrapper.Empty() {
		return nil, errMalformedTBSCert
	}
	var kept [][]byte
	for !list.Empty() {
		var element cryptobyte.String
		if !list.ReadASN1Element(&element, cbasn1.SEQUENCE) {
			return nil, errMalformedTBSCert
		}
		body := cryptobyte.String(element)
		var oid asn1.ObjectIdentifier
		if !body.ReadASN1(&body, cbasn1.SEQUENCE) || !body.ReadASN1ObjectIdentifier(&oid) {
			return nil, errMalformedTBSCert
		}
		if oid.Equal(oidAuthorityKeyID) || oid.Equal(oidIssuerAltName) {
			continue
		}
		kept = append(kept, element)
	}
	return kept, nil
}

func addUnsignedAlgorithm(b *cryptobyte.Builder) {
	b.AddASN1(cbasn1.SEQUENCE, func(a *cryptobyte.Builder) {
		a.AddASN1ObjectIdentifier(idAlgUnsigned)
	})
}

// NewUnsigned is New followed by Unsign. req.RawSubject must be set; the
// returned certificate is an RFC 9925 unsigned root whose subject and issuer
// are req.RawSubject. req.Hosts is ignored, so the root has no SANs.
func NewUnsigned(req *csr.CertificateRequest) (cert, key []byte, err error) {
	if len(req.RawSubject) == 0 {
		return nil, nil, cferr.Wrap(cferr.PolicyError, cferr.InvalidRequest,
			errors.New("missing raw subject"))
	}
	noHosts := *req
	noHosts.Hosts = nil
	signed, _, key, err := New(&noHosts)
	if err != nil {
		return nil, nil, err
	}
	cert, err = Unsign(signed)
	if err != nil {
		return nil, nil, err
	}
	return cert, key, nil
}

// NewUnsignedFromSigner returns an RFC 9925 unsigned root for the public key
// of priv, with the same request rules as NewUnsigned. req.KeyRequest is
// ignored.
//
// Most signers that do not hold the key they report fail earlier, when the CSR
// or the certificate is signed. After priv signs the certificate, the
// signature is checked under the certificate's own public key, which fails
// with a KeyMismatch error for a signer whose Public result changes between
// operations.
func NewUnsignedFromSigner(req *csr.CertificateRequest, priv crypto.Signer) (cert []byte, err error) {
	if len(req.RawSubject) == 0 {
		return nil, cferr.Wrap(cferr.PolicyError, cferr.InvalidRequest,
			errors.New("missing raw subject"))
	}
	if priv == nil {
		return nil, cferr.Wrap(cferr.PrivateKeyError, cferr.Unknown, errors.New("missing private key"))
	}
	noHosts := *req
	noHosts.Hosts = nil
	signed, _, err := newFromSigner(&noHosts, priv, true)
	if err != nil {
		return nil, err
	}
	parsed, err := helpers.ParseCertificatePEM(signed)
	if err != nil {
		return nil, err
	}
	if err := parsed.CheckSignature(parsed.SignatureAlgorithm, parsed.RawTBSCertificate, parsed.Signature); err != nil {
		return nil, cferr.Wrap(cferr.PrivateKeyError, cferr.KeyMismatch, err)
	}
	return Unsign(signed)
}
