// Package local implements certificate signature functionality for CFSSL.
package local

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"time"

	"github.com/cloudflare/cfssl/certdb"
	"github.com/cloudflare/cfssl/config"
	cferr "github.com/cloudflare/cfssl/errors"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/info"
	"github.com/cloudflare/cfssl/log"
	"github.com/cloudflare/cfssl/signer"
	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/jsonclient"

	zx509 "github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3"
	"github.com/zmap/zlint/v3/lint"
)

// Signer contains a signer that uses the standard library to
// support both ECDSA and RSA CA keys.
type Signer struct {
	ca   *x509.Certificate
	priv crypto.Signer
	// lintPriv is generated randomly when pre-issuance linting is configured and
	// used to sign TBSCertificates for linting.
	lintPriv   crypto.Signer
	policy     *config.Signing
	sigAlgo    x509.SignatureAlgorithm
	dbAccessor certdb.Accessor
}

// NewSigner creates a new Signer directly from a
// private key and certificate, with optional policy.
func NewSigner(priv crypto.Signer, cert *x509.Certificate, sigAlgo x509.SignatureAlgorithm, policy *config.Signing) (*Signer, error) {
	if policy == nil {
		policy = &config.Signing{
			Profiles: map[string]*config.SigningProfile{},
			Default:  config.DefaultConfig()}
	}

	if !policy.Valid() {
		return nil, cferr.New(cferr.PolicyError, cferr.InvalidPolicy)
	}

	var lintPriv crypto.Signer
	// If there is at least one profile (including the default) that configures
	// pre-issuance linting then generate the one-off lintPriv key.
	for _, profile := range policy.Profiles {
		if profile.LintErrLevel > 0 || policy.Default.LintErrLevel > 0 {
			// In the future there may be demand for specifying the type of signer used
			// for pre-issuance linting in configuration. For now we assume that signing
			// with a randomly generated P-256 ECDSA private key is acceptable for all cases
			// where linting is requested.
			k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				return nil, cferr.New(cferr.PrivateKeyError, cferr.GenerationFailed)
			}
			lintPriv = k
			break
		}
	}

	return &Signer{
		ca:       cert,
		priv:     priv,
		lintPriv: lintPriv,
		sigAlgo:  sigAlgo,
		policy:   policy,
	}, nil
}

// NewSignerFromFile generates a new local signer from a caFile
// and a caKey file, both PEM encoded.
func NewSignerFromFile(caFile, caKeyFile string, policy *config.Signing) (*Signer, error) {
	log.Debug("Loading CA: ", caFile)
	ca, err := helpers.ReadBytes(caFile)
	if err != nil {
		return nil, err
	}
	log.Debug("Loading CA key: ", caKeyFile)
	cakey, err := helpers.ReadBytes(caKeyFile)
	if err != nil {
		return nil, cferr.Wrap(cferr.CertificateError, cferr.ReadFailed, err)
	}

	parsedCa, err := helpers.ParseCertificatePEM(ca)
	if err != nil {
		return nil, err
	}

	strPassword := os.Getenv("CFSSL_CA_PK_PASSWORD")
	password := []byte(strPassword)
	if strPassword == "" {
		password = nil
	}

	priv, err := helpers.ParsePrivateKeyPEMWithPassword(cakey, password)
	if err != nil {
		log.Debugf("Malformed private key %v", err)
		return nil, err
	}

	return NewSigner(priv, parsedCa, signer.DefaultSigAlgo(priv), policy)
}

// LintError is an error type returned when pre-issuance linting is configured
// in a signing profile and a TBS Certificate fails linting. It wraps the
// concrete zlint LintResults so that callers can further inspect the cause of
// the failing lints.
type LintError struct {
	ErrorResults map[string]lint.LintResult
}

func (e *LintError) Error() string {
	return fmt.Sprintf("pre-issuance linting found %d error results",
		len(e.ErrorResults))
}

// lint performs pre-issuance linting of a given TBS certificate template when
// the provided errLevel is > 0. Note that the template is provided by-value and
// not by-reference. This is important as the lint function needs to mutate the
// template's signature algorithm to match the lintPriv.
func (s *Signer) lint(template x509.Certificate, errLevel lint.LintStatus, lintRegistry lint.Registry) error {
	// Always return nil when linting is disabled (lint.Reserved == 0).
	if errLevel == lint.Reserved {
		return nil
	}
	// without a lintPriv key to use to sign the tbsCertificate we can't lint it.
	if s.lintPriv == nil {
		return cferr.New(cferr.PrivateKeyError, cferr.Unavailable)
	}

	// The template's SignatureAlgorithm must be mutated to match the lintPriv or
	// x509.CreateCertificate will error because of the mismatch. At the time of
	// writing s.lintPriv is always an ECDSA private key. This switch will need to
	// be expanded if the lint key type is made configurable.
	switch s.lintPriv.(type) {
	case *ecdsa.PrivateKey:
		template.SignatureAlgorithm = x509.ECDSAWithSHA256
	default:
		return cferr.New(cferr.PrivateKeyError, cferr.KeyMismatch)
	}

	prelintBytes, err := x509.CreateCertificate(rand.Reader, &template, s.ca, template.PublicKey, s.lintPriv)
	if err != nil {
		return cferr.Wrap(cferr.CertificateError, cferr.Unknown, err)
	}
	prelintCert, err := zx509.ParseCertificate(prelintBytes)
	if err != nil {
		return cferr.Wrap(cferr.CertificateError, cferr.ParseFailed, err)
	}
	errorResults := map[string]lint.LintResult{}
	results := zlint.LintCertificateEx(prelintCert, lintRegistry)
	for name, res := range results.Results {
		if res.Status > errLevel {
			errorResults[name] = *res
		}
	}
	if len(errorResults) > 0 {
		return &LintError{
			ErrorResults: errorResults,
		}
	}
	return nil
}

func (s *Signer) sign(template *x509.Certificate, lintErrLevel lint.LintStatus, lintRegistry lint.Registry) (cert []byte, err error) {
	var initRoot bool
	if s.ca == nil {
		if !template.IsCA {
			err = cferr.New(cferr.PolicyError, cferr.InvalidRequest)
			return
		}
		template.DNSNames = nil
		template.EmailAddresses = nil
		template.URIs = nil
		s.ca = template
		initRoot = true
	}

	if err := s.lint(*template, lintErrLevel, lintRegistry); err != nil {
		return nil, err
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, s.ca, template.PublicKey, s.priv)
	if err != nil {
		return nil, cferr.Wrap(cferr.CertificateError, cferr.Unknown, err)
	}
	if initRoot {
		s.ca, err = x509.ParseCertificate(derBytes)
		if err != nil {
			return nil, cferr.Wrap(cferr.CertificateError, cferr.ParseFailed, err)
		}
	}

	cert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	log.Infof("signed certificate with serial number %d", template.SerialNumber)
	return
}

// replaceSliceIfEmpty replaces the contents of replaced with newContents if
// the slice referenced by replaced is empty
func replaceSliceIfEmpty(replaced, newContents *[]string) {
	if len(*replaced) == 0 {
		*replaced = *newContents
	}
}

// PopulateSubjectFromCSR has functionality similar to Name, except
// it fills the fields of the resulting pkix.Name with req's if the
// subject's corresponding fields are empty
func PopulateSubjectFromCSR(s *signer.Subject, req pkix.Name) pkix.Name {
	// if no subject, use req
	if s == nil {
		return req
	}

	name := s.Name()

	if name.CommonName == "" {
		name.CommonName = req.CommonName
	}

	replaceSliceIfEmpty(&name.Country, &req.Country)
	replaceSliceIfEmpty(&name.Province, &req.Province)
	replaceSliceIfEmpty(&name.Locality, &req.Locality)
	replaceSliceIfEmpty(&name.Organization, &req.Organization)
	replaceSliceIfEmpty(&name.OrganizationalUnit, &req.OrganizationalUnit)
	if name.SerialNumber == "" {
		name.SerialNumber = req.SerialNumber
	}
	return name
}

// OverrideHosts fills template's IPAddresses, EmailAddresses, DNSNames, and URIs with the
// content of hosts, if it is not nil.
func OverrideHosts(template *x509.Certificate, hosts []string) {
	if hosts != nil {
		template.IPAddresses = []net.IP{}
		template.EmailAddresses = []string{}
		template.DNSNames = []string{}
		template.URIs = []*url.URL{}
	}

	for i := range hosts {
		if ip := net.ParseIP(hosts[i]); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else if email, err := mail.ParseAddress(hosts[i]); err == nil && email != nil {
			template.EmailAddresses = append(template.EmailAddresses, email.Address)
		} else if uri, err := url.ParseRequestURI(hosts[i]); err == nil && uri != nil {
			template.URIs = append(template.URIs, uri)
		} else {
			template.DNSNames = append(template.DNSNames, hosts[i])
		}
	}

}

// Sign signs a new certificate based on the PEM-encoded client
// certificate or certificate request with the signing profile,
// specified by profileName.
func (s *Signer) Sign(req signer.SignRequest) (cert []byte, err error) {
	profile, err := signer.Profile(s, req.Profile)
	if err != nil {
		return
	}

	block, _ := pem.Decode([]byte(req.Request))
	if block == nil {
		return nil, cferr.New(cferr.CSRError, cferr.DecodeFailed)
	}

	if block.Type != "NEW CERTIFICATE REQUEST" && block.Type != "CERTIFICATE REQUEST" {
		return nil, cferr.Wrap(cferr.CSRError,
			cferr.BadRequest, errors.New("not a csr"))
	}

	csrTemplate, err := signer.ParseCertificateRequest(s, profile, block.Bytes)
	if err != nil {
		return nil, err
	}

	// Copy out only the fields from the CSR authorized by policy.
	safeTemplate := x509.Certificate{}
	// If the profile contains no explicit whitelist, assume that all fields
	// should be copied from the CSR.
	if profile.CSRWhitelist == nil {
		safeTemplate = *csrTemplate
	} else {
		if profile.CSRWhitelist.Subject {
			safeTemplate.Subject = csrTemplate.Subject
		}
		if profile.CSRWhitelist.PublicKeyAlgorithm {
			safeTemplate.PublicKeyAlgorithm = csrTemplate.PublicKeyAlgorithm
		}
		if profile.CSRWhitelist.PublicKey {
			safeTemplate.PublicKey = csrTemplate.PublicKey
		}
		if profile.CSRWhitelist.SignatureAlgorithm {
			safeTemplate.SignatureAlgorithm = csrTemplate.SignatureAlgorithm
		}
		if profile.CSRWhitelist.DNSNames {
			safeTemplate.DNSNames = csrTemplate.DNSNames
		}
		if profile.CSRWhitelist.IPAddresses {
			safeTemplate.IPAddresses = csrTemplate.IPAddresses
		}
		if profile.CSRWhitelist.EmailAddresses {
			safeTemplate.EmailAddresses = csrTemplate.EmailAddresses
		}
		if profile.CSRWhitelist.URIs {
			safeTemplate.URIs = csrTemplate.URIs
		}
	}

	if req.CRLOverride != "" {
		safeTemplate.CRLDistributionPoints = []string{req.CRLOverride}
	}

	if safeTemplate.IsCA {
		if !profile.CAConstraint.IsCA {
			log.Error("local signer policy disallows issuing CA certificate")
			return nil, cferr.New(cferr.PolicyError, cferr.InvalidRequest)
		}

		if s.ca != nil && s.ca.MaxPathLen > 0 {
			if safeTemplate.MaxPathLen >= s.ca.MaxPathLen {
				log.Error("local signer certificate disallows CA MaxPathLen extending")
				// do not sign a cert with pathlen > current
				return nil, cferr.New(cferr.PolicyError, cferr.InvalidRequest)
			}
		} else if s.ca != nil && s.ca.MaxPathLen == 0 && s.ca.MaxPathLenZero {
			log.Error("local signer certificate disallows issuing CA certificate")
			// signer has pathlen of 0, do not sign more intermediate CAs
			return nil, cferr.New(cferr.PolicyError, cferr.InvalidRequest)
		}
	}

	OverrideHosts(&safeTemplate, req.Hosts)
	safeTemplate.Subject = PopulateSubjectFromCSR(req.Subject, safeTemplate.Subject)

	// If there is a whitelist, ensure that both the Common Name and SAN DNSNames match
	if profile.NameWhitelist != nil {
		if safeTemplate.Subject.CommonName != "" {
			if profile.NameWhitelist.Find([]byte(safeTemplate.Subject.CommonName)) == nil {
				return nil, cferr.New(cferr.PolicyError, cferr.UnmatchedWhitelist)
			}
		}
		for _, name := range safeTemplate.DNSNames {
			if profile.NameWhitelist.Find([]byte(name)) == nil {
				return nil, cferr.New(cferr.PolicyError, cferr.UnmatchedWhitelist)
			}
		}
		for _, name := range safeTemplate.EmailAddresses {
			if profile.NameWhitelist.Find([]byte(name)) == nil {
				return nil, cferr.New(cferr.PolicyError, cferr.UnmatchedWhitelist)
			}
		}
		for _, name := range safeTemplate.URIs {
			if profile.NameWhitelist.Find([]byte(name.String())) == nil {
				return nil, cferr.New(cferr.PolicyError, cferr.UnmatchedWhitelist)
			}
		}
	}

	if profile.ClientProvidesSerialNumbers {
		if req.Serial == nil {
			return nil, cferr.New(cferr.CertificateError, cferr.MissingSerial)
		}
		safeTemplate.SerialNumber = req.Serial
	} else {
		// RFC 5280 4.1.2.2:
		// Certificate users MUST be able to handle serialNumber
		// values up to 20 octets.  Conforming CAs MUST NOT use
		// serialNumber values longer than 20 octets.
		//
		// If CFSSL is providing the serial numbers, it makes
		// sense to use the max supported size.
		serialNumber := make([]byte, 20)
		_, err = io.ReadFull(rand.Reader, serialNumber)
		if err != nil {
			return nil, cferr.Wrap(cferr.CertificateError, cferr.Unknown, err)
		}

		// SetBytes interprets buf as the bytes of a big-endian
		// unsigned integer. The leading byte should be masked
		// off to ensure it isn't negative.
		serialNumber[0] &= 0x7F

		safeTemplate.SerialNumber = new(big.Int).SetBytes(serialNumber)
	}

	if len(req.Extensions) > 0 {
		for _, ext := range req.Extensions {
			oid := asn1.ObjectIdentifier(ext.ID)
			if !profile.ExtensionWhitelist[oid.String()] {
				return nil, cferr.New(cferr.CertificateError, cferr.InvalidRequest)
			}

			rawValue, err := hex.DecodeString(ext.Value)
			if err != nil {
				return nil, cferr.Wrap(cferr.CertificateError, cferr.InvalidRequest, err)
			}

			safeTemplate.ExtraExtensions = append(safeTemplate.ExtraExtensions, pkix.Extension{
				Id:       oid,
				Critical: ext.Critical,
				Value:    rawValue,
			})
		}
	}

	var distPoints = safeTemplate.CRLDistributionPoints
	err = signer.FillTemplate(&safeTemplate, s.policy.Default, profile, req.NotBefore, req.NotAfter)
	if err != nil {
		return nil, err
	}
	if distPoints != nil && len(distPoints) > 0 {
		safeTemplate.CRLDistributionPoints = distPoints
	}

	var certTBS = safeTemplate

	if len(profile.CTLogServers) > 0 || req.ReturnPrecert {
		// Add a poison extension which prevents validation
		var poisonExtension = pkix.Extension{Id: signer.CTPoisonOID, Critical: true, Value: []byte{0x05, 0x00}}
		var poisonedPreCert = certTBS
		poisonedPreCert.ExtraExtensions = append(safeTemplate.ExtraExtensions, poisonExtension)
		cert, err = s.sign(&poisonedPreCert, profile.LintErrLevel, profile.LintRegistry)
		if err != nil {
			return
		}

		if req.ReturnPrecert {
			return cert, nil
		}

		derCert, _ := pem.Decode(cert)
		prechain := []ct.ASN1Cert{{Data: derCert.Bytes}, {Data: s.ca.Raw}}
		var sctList []ct.SignedCertificateTimestamp

		for _, server := range profile.CTLogServers {
			log.Infof("submitting poisoned precertificate to %s", server)
			ctclient, err := client.New(server, nil, jsonclient.Options{})
			if err != nil {
				return nil, cferr.Wrap(cferr.CTError, cferr.PrecertSubmissionFailed, err)
			}
			var resp *ct.SignedCertificateTimestamp
			ctx := context.Background()
			resp, err = ctclient.AddPreChain(ctx, prechain)
			if err != nil {
				return nil, cferr.Wrap(cferr.CTError, cferr.PrecertSubmissionFailed, err)
			}
			sctList = append(sctList, *resp)
		}

		var serializedSCTList []byte
		serializedSCTList, err = helpers.SerializeSCTList(sctList)
		if err != nil {
			return nil, cferr.Wrap(cferr.CTError, cferr.Unknown, err)
		}

		// Serialize again as an octet string before embedding
		serializedSCTList, err = asn1.Marshal(serializedSCTList)
		if err != nil {
			return nil, cferr.Wrap(cferr.CTError, cferr.Unknown, err)
		}

		var SCTListExtension = pkix.Extension{Id: signer.SCTListOID, Critical: false, Value: serializedSCTList}
		certTBS.ExtraExtensions = append(certTBS.ExtraExtensions, SCTListExtension)
	}

	var signedCert []byte
	signedCert, err = s.sign(&certTBS, profile.LintErrLevel, profile.LintRegistry)
	if err != nil {
		return nil, err
	}

	// Get the AKI from signedCert.  This is required to support Go 1.9+.
	// In prior versions of Go, x509.CreateCertificate updated the
	// AuthorityKeyId of certTBS.
	parsedCert, _ := helpers.ParseCertificatePEM(signedCert)

	if s.dbAccessor != nil {
		now := time.Now()
		var certRecord = certdb.CertificateRecord{
			Serial: certTBS.SerialNumber.String(),
			// this relies on the specific behavior of x509.CreateCertificate
			// which sets the AuthorityKeyId from the signer's SubjectKeyId
			AKI:        hex.EncodeToString(parsedCert.AuthorityKeyId),
			CALabel:    req.Label,
			Status:     "good",
			Expiry:     certTBS.NotAfter,
			PEM:        string(signedCert),
			IssuedAt:   &now,
			NotBefore:  &certTBS.NotBefore,
			CommonName: sql.NullString{String: certTBS.Subject.CommonName, Valid: true},
		}

		if err := certRecord.SetMetadata(req.Metadata); err != nil {
			return nil, err
		}
		if err := certRecord.SetSANs(certTBS.DNSNames); err != nil {
			return nil, err
		}

		if err := s.dbAccessor.InsertCertificate(certRecord); err != nil {
			return nil, err
		}
		log.Debug("saved certificate with serial number ", certTBS.SerialNumber)
	}

	return signedCert, nil
}

// SignFromPrecert creates and signs a certificate from an existing precertificate
// that was previously signed by Signer.ca and inserts the provided SCTs into the
// new certificate. The resulting certificate will be a exact copy of the precert
// except for the removal of the poison extension and the addition of the SCT list
// extension. SignFromPrecert does not verify that the contents of the certificate
// still match the signing profile of the signer, it only requires that the precert
// was previously signed by the Signers CA. Similarly, any linting configured
// by the profile used to sign the precert will not be re-applied to the final
// cert and must be done separately by the caller.
func (s *Signer) SignFromPrecert(precert *x509.Certificate, scts []ct.SignedCertificateTimestamp) ([]byte, error) {
	// Verify certificate was signed by s.ca
	if err := precert.CheckSignatureFrom(s.ca); err != nil {
		return nil, err
	}

	// Verify certificate is a precert
	isPrecert := false
	poisonIndex := 0
	for i, ext := range precert.Extensions {
		if ext.Id.Equal(signer.CTPoisonOID) {
			if !ext.Critical {
				return nil, cferr.New(cferr.CTError, cferr.PrecertInvalidPoison)
			}
			// Check extension contains ASN.1 NULL
			if bytes.Compare(ext.Value, []byte{0x05, 0x00}) != 0 {
				return nil, cferr.New(cferr.CTError, cferr.PrecertInvalidPoison)
			}
			isPrecert = true
			poisonIndex = i
			break
		}
	}
	if !isPrecert {
		return nil, cferr.New(cferr.CTError, cferr.PrecertMissingPoison)
	}

	// Serialize SCTs into list format and create extension
	serializedList, err := helpers.SerializeSCTList(scts)
	if err != nil {
		return nil, err
	}
	// Serialize again as an octet string before embedding
	serializedList, err = asn1.Marshal(serializedList)
	if err != nil {
		return nil, cferr.Wrap(cferr.CTError, cferr.Unknown, err)
	}
	sctExt := pkix.Extension{Id: signer.SCTListOID, Critical: false, Value: serializedList}

	// Create the new tbsCert from precert. Do explicit copies of any slices so that we don't
	// use memory that may be altered by us or the caller at a later stage.
	tbsCert := x509.Certificate{
		SignatureAlgorithm:          precert.SignatureAlgorithm,
		PublicKeyAlgorithm:          precert.PublicKeyAlgorithm,
		PublicKey:                   precert.PublicKey,
		Version:                     precert.Version,
		SerialNumber:                precert.SerialNumber,
		Issuer:                      precert.Issuer,
		Subject:                     precert.Subject,
		NotBefore:                   precert.NotBefore,
		NotAfter:                    precert.NotAfter,
		KeyUsage:                    precert.KeyUsage,
		BasicConstraintsValid:       precert.BasicConstraintsValid,
		IsCA:                        precert.IsCA,
		MaxPathLen:                  precert.MaxPathLen,
		MaxPathLenZero:              precert.MaxPathLenZero,
		PermittedDNSDomainsCritical: precert.PermittedDNSDomainsCritical,
	}
	if len(precert.Extensions) > 0 {
		tbsCert.ExtraExtensions = make([]pkix.Extension, len(precert.Extensions))
		copy(tbsCert.ExtraExtensions, precert.Extensions)
	}

	// Remove the poison extension from ExtraExtensions
	tbsCert.ExtraExtensions = append(tbsCert.ExtraExtensions[:poisonIndex], tbsCert.ExtraExtensions[poisonIndex+1:]...)
	// Insert the SCT list extension
	tbsCert.ExtraExtensions = append(tbsCert.ExtraExtensions, sctExt)

	// Sign the tbsCert. Linting is always disabled because there is no way for
	// this API to know the correct lint settings to use because there is no
	// reference to the signing profile of the precert available.
	return s.sign(&tbsCert, 0, nil)
}

// Info return a populated info.Resp struct or an error.
func (s *Signer) Info(req info.Req) (resp *info.Resp, err error) {
	cert, err := s.Certificate(req.Label, req.Profile)
	if err != nil {
		return
	}

	profile, err := signer.Profile(s, req.Profile)
	if err != nil {
		return
	}

	resp = new(info.Resp)
	if cert.Raw != nil {
		resp.Certificate = string(bytes.TrimSpace(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})))
	}
	resp.Usage = profile.Usage
	resp.ExpiryString = profile.ExpiryString

	return
}

// SigAlgo returns the RSA signer's signature algorithm.
func (s *Signer) SigAlgo() x509.SignatureAlgorithm {
	return s.sigAlgo
}

// Certificate returns the signer's certificate.
func (s *Signer) Certificate(label, profile string) (*x509.Certificate, error) {
	cert := *s.ca
	return &cert, nil
}

// SetPolicy sets the signer's signature policy.
func (s *Signer) SetPolicy(policy *config.Signing) {
	s.policy = policy
}

// SetDBAccessor sets the signers' cert db accessor
func (s *Signer) SetDBAccessor(dba certdb.Accessor) {
	s.dbAccessor = dba
}

// GetDBAccessor returns the signers' cert db accessor
func (s *Signer) GetDBAccessor() certdb.Accessor {
	return s.dbAccessor
}

// SetReqModifier does nothing for local
func (s *Signer) SetReqModifier(func(*http.Request, []byte)) {
	// noop
}

// Policy returns the signer's policy.
func (s *Signer) Policy() *config.Signing {
	return s.policy
}
