package ca

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	cfcsr "github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/initca"
	cflog "github.com/cloudflare/cfssl/log"
	cfsigner "github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"
	"github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/connectionbroker"
	"github.com/docker/swarmkit/ioutils"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
)

const (
	// Security Strength Equivalence
	//-----------------------------------
	//| ECC  |  DH/DSA/RSA  |
	//| 256  |     3072     |
	//| 384  |     7680     |
	//-----------------------------------

	// RootKeySize is the default size of the root CA key
	// It would be ideal for the root key to use P-384, but in P-384 is not optimized in go yet :(
	RootKeySize = 256
	// RootKeyAlgo defines the default algorithm for the root CA Key
	RootKeyAlgo = "ecdsa"
	// PassphraseENVVar defines the environment variable to look for the
	// root CA private key material encryption key
	PassphraseENVVar = "SWARM_ROOT_CA_PASSPHRASE"
	// PassphraseENVVarPrev defines the alternate environment variable to look for the
	// root CA private key material encryption key. It can be used for seamless
	// KEK rotations.
	PassphraseENVVarPrev = "SWARM_ROOT_CA_PASSPHRASE_PREV"
	// RootCAExpiration represents the default expiration for the root CA in seconds (20 years)
	RootCAExpiration = "630720000s"
	// DefaultNodeCertExpiration represents the default expiration for node certificates (3 months)
	DefaultNodeCertExpiration = 2160 * time.Hour
	// CertBackdate represents the amount of time each certificate is backdated to try to avoid
	// clock drift issues.
	CertBackdate = 1 * time.Hour
	// CertLowerRotationRange represents the minimum fraction of time that we will wait when randomly
	// choosing our next certificate rotation
	CertLowerRotationRange = 0.5
	// CertUpperRotationRange represents the maximum fraction of time that we will wait when randomly
	// choosing our next certificate rotation
	CertUpperRotationRange = 0.8
	// MinNodeCertExpiration represents the minimum expiration for node certificates
	MinNodeCertExpiration = 1 * time.Hour
)

// BasicConstraintsOID is the ASN1 Object ID indicating a basic constraints extension
var BasicConstraintsOID = asn1.ObjectIdentifier{2, 5, 29, 19}

// A recoverableErr is a non-fatal error encountered signing a certificate,
// which means that the certificate issuance may be retried at a later time.
type recoverableErr struct {
	err error
}

func (r recoverableErr) Error() string {
	return r.err.Error()
}

// ErrNoLocalRootCA is an error type used to indicate that the local root CA
// certificate file does not exist.
var ErrNoLocalRootCA = errors.New("local root CA certificate does not exist")

// ErrNoValidSigner is an error type used to indicate that our RootCA doesn't have the ability to
// sign certificates.
var ErrNoValidSigner = recoverableErr{err: errors.New("no valid signer found")}

func init() {
	cflog.Level = 5
}

// CertPaths is a helper struct that keeps track of the paths of a
// Cert and corresponding Key
type CertPaths struct {
	Cert, Key string
}

// IssuerInfo contains the subject and public key of the issuer of a certificate
type IssuerInfo struct {
	Subject   []byte
	PublicKey []byte
}

// LocalSigner is a signer that can sign CSRs
type LocalSigner struct {
	cfsigner.Signer

	// Key will only be used by the original manager to put the private
	// key-material in raft, no signing operations depend on it.
	Key []byte

	// Cert is one PEM encoded Certificate used as the signing CA.  It must correspond to the key.
	Cert []byte

	// just cached parsed values for validation, etc.
	parsedCert   *x509.Certificate
	cryptoSigner crypto.Signer
}

// RootCA is the representation of everything we need to sign certificates and/or to verify certificates
//
// RootCA.Cert:          [CA cert1][CA cert2]
// RootCA.Intermediates: [intermediate CA1][intermediate CA2][intermediate CA3]
// RootCA.signer.Cert:   [signing CA cert]
// RootCA.signer.Key:    [signing CA key]
//
// Requirements:
//
// - [signing CA key] must be the private key for [signing CA cert], and either both or none must be provided
//
// - [intermediate CA1] must have the same public key and subject as [signing CA cert], because otherwise when
//   appended to a leaf certificate, the intermediates will not form a chain (because [intermediate CA1] won't because
//   the signer of the leaf certificate)
// - [intermediate CA1] must be signed by [intermediate CA2], which must be signed by [intermediate CA3]
//
// - When we issue a certificate, the intermediates will be appended so that the certificate looks like:
//   [leaf signed by signing CA cert][intermediate CA1][intermediate CA2][intermediate CA3]
// - [leaf signed by signing CA cert][intermediate CA1][intermediate CA2][intermediate CA3] is guaranteed to form a
//   valid chain from [leaf signed by signing CA cert] to one of the root certs ([signing CA cert], [CA cert1], [CA cert2])
//   using zero or more of the intermediate certs ([intermediate CA1][intermediate CA2][intermediate CA3]) as intermediates
//
// Example 1:  Simple root rotation
// - Initial state:
// 	 - RootCA.Cert:          [Root CA1 self-signed]
// 	 - RootCA.Intermediates: []
// 	 - RootCA.signer.Cert:   [Root CA1 self-signed]
// 	 - Issued TLS cert:      [leaf signed by Root CA1]
//
// - Intermediate state (during root rotation):
//   - RootCA.Cert:          [Root CA1 self-signed]
//   - RootCA.Intermediates: [Root CA2 signed by Root CA1]
//   - RootCA.signer.Cert:   [Root CA2 signed by Root CA1]
//   - Issued TLS cert:      [leaf signed by Root CA2][Root CA2 signed by Root CA1]
//
// - Final state:
//   - RootCA.Cert:          [Root CA2 self-signed]
//   - RootCA.Intermediates: []
//   - RootCA.signer.Cert:   [Root CA2 self-signed]
//   - Issued TLS cert:      [leaf signed by Root CA2]
//
type RootCA struct {
	// Certs contains a bundle of self-signed, PEM encoded certificates for the Root CA to be used
	// as the root of trust.
	Certs []byte

	// Intermediates contains a bundle of PEM encoded intermediate CA certificates to append to any
	// issued TLS (leaf) certificates. The first one must have the same public key and subject as the
	// signing root certificate, and the rest must form a chain, each one certifying the one above it,
	// as per RFC5246 section 7.4.2.
	Intermediates []byte

	// Pool is the root pool used to validate TLS certificates
	Pool *x509.CertPool

	// Digest of the serialized bytes of the certificate(s)
	Digest digest.Digest

	// This signer will be nil if the node doesn't have the appropriate key material
	signer *LocalSigner
}

// Signer is an accessor for the local signer that returns an error if this root cannot sign.
func (rca *RootCA) Signer() (*LocalSigner, error) {
	if rca.Pool == nil || rca.signer == nil || len(rca.signer.Cert) == 0 || rca.signer.Signer == nil {
		return nil, ErrNoValidSigner
	}

	return rca.signer, nil
}

// IssueAndSaveNewCertificates generates a new key-pair, signs it with the local root-ca, and returns a
// TLS certificate and the issuer information for the certificate.
func (rca *RootCA) IssueAndSaveNewCertificates(kw KeyWriter, cn, ou, org string) (*tls.Certificate, *IssuerInfo, error) {
	csr, key, err := GenerateNewCSR()
	if err != nil {
		return nil, nil, errors.Wrap(err, "error when generating new node certs")
	}

	// Obtain a signed Certificate
	certChain, err := rca.ParseValidateAndSignCSR(csr, cn, ou, org)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to sign node certificate")
	}
	signer, err := rca.Signer()
	if err != nil { // should never happen, since if ParseValidateAndSignCSR did not fail this root CA must have a signer
		return nil, nil, err
	}

	// Create a valid TLSKeyPair out of the PEM encoded private key and certificate
	tlsKeyPair, err := tls.X509KeyPair(certChain, key)
	if err != nil {
		return nil, nil, err
	}

	if err := kw.Write(NormalizePEMs(certChain), key, nil); err != nil {
		return nil, nil, err
	}

	return &tlsKeyPair, &IssuerInfo{
		PublicKey: signer.parsedCert.RawSubjectPublicKeyInfo,
		Subject:   signer.parsedCert.RawSubject,
	}, nil
}

// RequestAndSaveNewCertificates gets new certificates issued, either by signing them locally if a signer is
// available, or by requesting them from the remote server at remoteAddr.  This function returns the TLS
// certificate and the issuer information for the certificate.
func (rca *RootCA) RequestAndSaveNewCertificates(ctx context.Context, kw KeyWriter, config CertificateRequestConfig) (*tls.Certificate, *IssuerInfo, error) {
	// Create a new key/pair and CSR
	csr, key, err := GenerateNewCSR()
	if err != nil {
		return nil, nil, errors.Wrap(err, "error when generating new node certs")
	}

	// Get the remote manager to issue a CA signed certificate for this node
	// Retry up to 5 times in case the manager we first try to contact isn't
	// responding properly (for example, it may have just been demoted).
	var signedCert []byte
	for i := 0; i != 5; i++ {
		signedCert, err = GetRemoteSignedCertificate(ctx, csr, rca.Pool, config)
		if err == nil {
			break
		}

		// If the first attempt fails, we should try a remote
		// connection. The local node may be a manager that was
		// demoted, so the local connection (which is preferred) may
		// not work. If we are successful in renewing the certificate,
		// the local connection will not be returned by the connection
		// broker anymore.
		config.ForceRemote = true

		// Wait a moment, in case a leader election was taking place.
		select {
		case <-time.After(config.RetryInterval):
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}
	if err != nil {
		return nil, nil, err
	}

	// Доверяй, но проверяй.
	// Before we overwrite our local key + certificate, let's make sure the server gave us one that is valid
	// Create an X509Cert so we can .Verify()
	// Check to see if this certificate was signed by our CA, and isn't expired
	parsedCerts, chains, err := ValidateCertChain(rca.Pool, signedCert, false)
	if err != nil {
		return nil, nil, err
	}

	// ValidateChain, if successful, will always return at least 1 parsed cert and at least 1 chain containing
	// at least 2 certificates:  the leaf and the root.
	leafCert := parsedCerts[0]
	issuer := chains[0][1]

	// Create a valid TLSKeyPair out of the PEM encoded private key and certificate
	tlsKeyPair, err := tls.X509KeyPair(signedCert, key)
	if err != nil {
		return nil, nil, err
	}

	var kekUpdate *KEKData
	for i := 0; i < 5; i++ {
		// ValidateCertChain will always return at least 1 cert, so indexing at 0 is safe
		kekUpdate, err = rca.getKEKUpdate(ctx, leafCert, tlsKeyPair, config)
		if err == nil {
			break
		}

		config.ForceRemote = true

		// Wait a moment, in case a leader election was taking place.
		select {
		case <-time.After(config.RetryInterval):
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}
	if err != nil {
		return nil, nil, err
	}

	if err := kw.Write(NormalizePEMs(signedCert), key, kekUpdate); err != nil {
		return nil, nil, err
	}

	return &tlsKeyPair, &IssuerInfo{
		PublicKey: issuer.RawSubjectPublicKeyInfo,
		Subject:   issuer.RawSubject,
	}, nil
}

func (rca *RootCA) getKEKUpdate(ctx context.Context, leafCert *x509.Certificate, keypair tls.Certificate, config CertificateRequestConfig) (*KEKData, error) {
	var managerRole bool
	for _, ou := range leafCert.Subject.OrganizationalUnit {
		if ou == ManagerRole {
			managerRole = true
			break
		}
	}

	if managerRole {
		mtlsCreds := credentials.NewTLS(&tls.Config{ServerName: CARole, RootCAs: rca.Pool, Certificates: []tls.Certificate{keypair}})
		conn, err := getGRPCConnection(mtlsCreds, config.ConnBroker, config.ForceRemote)
		if err != nil {
			return nil, err
		}

		client := api.NewCAClient(conn.ClientConn)
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		response, err := client.GetUnlockKey(ctx, &api.GetUnlockKeyRequest{})
		if err != nil {
			if grpc.Code(err) == codes.Unimplemented { // if the server does not support keks, return as if no encryption key was specified
				conn.Close(true)
				return &KEKData{}, nil
			}

			conn.Close(false)
			return nil, err
		}
		conn.Close(true)
		return &KEKData{KEK: response.UnlockKey, Version: response.Version.Index}, nil
	}

	// If this is a worker, set to never encrypt. We always want to set to the lock key to nil,
	// in case this was a manager that was demoted to a worker.
	return &KEKData{}, nil
}

// PrepareCSR creates a CFSSL Sign Request based on the given raw CSR and
// overrides the Subject and Hosts with the given extra args.
func PrepareCSR(csrBytes []byte, cn, ou, org string) cfsigner.SignRequest {
	// All managers get added the subject-alt-name of CA, so they can be
	// used for cert issuance.
	hosts := []string{ou, cn}
	if ou == ManagerRole {
		hosts = append(hosts, CARole)
	}

	return cfsigner.SignRequest{
		Request: string(csrBytes),
		// OU is used for Authentication of the node type. The CN has the random
		// node ID.
		Subject: &cfsigner.Subject{CN: cn, Names: []cfcsr.Name{{OU: ou, O: org}}},
		// Adding ou as DNS alt name, so clients can connect to ManagerRole and CARole
		Hosts: hosts,
	}
}

// ParseValidateAndSignCSR returns a signed certificate from a particular rootCA and a CSR.
func (rca *RootCA) ParseValidateAndSignCSR(csrBytes []byte, cn, ou, org string) ([]byte, error) {
	signRequest := PrepareCSR(csrBytes, cn, ou, org)
	signer, err := rca.Signer()
	if err != nil {
		return nil, err
	}
	cert, err := signer.Sign(signRequest)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign node certificate")
	}

	return append(cert, rca.Intermediates...), nil
}

// CrossSignCACertificate takes a CA root certificate and generates an intermediate CA from it signed with the current root signer
func (rca *RootCA) CrossSignCACertificate(otherCAPEM []byte) ([]byte, error) {
	signer, err := rca.Signer()
	if err != nil {
		return nil, err
	}

	// create a new cert with exactly the same parameters, including the public key and exact NotBefore and NotAfter
	template, err := helpers.ParseCertificatePEM(otherCAPEM)
	if err != nil {
		return nil, errors.New("could not parse new CA certificate")
	}

	if !template.IsCA {
		return nil, errors.New("certificate not a CA")
	}

	template.SignatureAlgorithm = signer.parsedCert.SignatureAlgorithm // make sure we can sign with the signer key
	derBytes, err := x509.CreateCertificate(cryptorand.Reader, template, signer.parsedCert, template.PublicKey, signer.cryptoSigner)
	if err != nil {
		return nil, errors.Wrap(err, "could not cross-sign new CA certificate using old CA material")
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	}), nil
}

func validateSignatureAlgorithm(cert *x509.Certificate) error {
	switch cert.SignatureAlgorithm {
	case x509.SHA256WithRSA, x509.SHA384WithRSA, x509.SHA512WithRSA, x509.ECDSAWithSHA256, x509.ECDSAWithSHA384, x509.ECDSAWithSHA512:
		return nil
	default:
		return fmt.Errorf("unsupported signature algorithm: %s", cert.SignatureAlgorithm.String())
	}
}

// NewRootCA creates a new RootCA object from unparsed PEM cert bundle and key byte
// slices. key may be nil, and in this case NewRootCA will return a RootCA
// without a signer.
func NewRootCA(rootCertBytes, signCertBytes, signKeyBytes []byte, certExpiry time.Duration, intermediates []byte) (RootCA, error) {
	// Parse all the certificates in the cert bundle
	parsedCerts, err := helpers.ParseCertificatesPEM(rootCertBytes)
	if err != nil {
		return RootCA{}, errors.Wrap(err, "invalid root certificates")
	}
	// Check to see if we have at least one valid cert
	if len(parsedCerts) < 1 {
		return RootCA{}, errors.New("no valid root CA certificates found")
	}

	// Create a Pool with all of the certificates found
	pool := x509.NewCertPool()
	for _, cert := range parsedCerts {
		if err := validateSignatureAlgorithm(cert); err != nil {
			return RootCA{}, err
		}
		// Check to see if all of the certificates are valid, self-signed root CA certs
		selfpool := x509.NewCertPool()
		selfpool.AddCert(cert)
		if _, err := cert.Verify(x509.VerifyOptions{Roots: selfpool}); err != nil {
			return RootCA{}, errors.Wrap(err, "error while validating Root CA Certificate")
		}
		pool.AddCert(cert)
	}

	// Calculate the digest for our Root CA bundle
	digest := digest.FromBytes(rootCertBytes)

	// The intermediates supplied must be able to chain up to the root certificates, so that when they are appended to
	// a leaf certificate, the leaf certificate can be validated through the intermediates to the root certificates.
	var intermediatePool *x509.CertPool
	var parsedIntermediates []*x509.Certificate
	if len(intermediates) > 0 {
		parsedIntermediates, _, err = ValidateCertChain(pool, intermediates, false)
		if err != nil {
			return RootCA{}, errors.Wrap(err, "invalid intermediate chain")
		}
		intermediatePool = x509.NewCertPool()
		for _, cert := range parsedIntermediates {
			intermediatePool.AddCert(cert)
		}
	}

	var localSigner *LocalSigner
	if len(signKeyBytes) != 0 || len(signCertBytes) != 0 {
		localSigner, err = newLocalSigner(signKeyBytes, signCertBytes, certExpiry, pool, intermediatePool)
		if err != nil {
			return RootCA{}, err
		}

		// If a signer is provided and there are intermediates, then either the first intermediate would be the signer CA
		// certificate (in which case it'd have the same subject and public key), or it would be a cross-signed
		// intermediate with the same subject and public key as our signing CA certificate (which could be either an
		// intermediate cert or a self-signed root cert).
		if len(parsedIntermediates) > 0 && (!bytes.Equal(parsedIntermediates[0].RawSubject, localSigner.parsedCert.RawSubject) ||
			!bytes.Equal(parsedIntermediates[0].RawSubjectPublicKeyInfo, localSigner.parsedCert.RawSubjectPublicKeyInfo)) {
			return RootCA{}, errors.New(
				"invalid intermediate chain - the first intermediate must have the same subject and public key as the signing cert")
		}
	}

	return RootCA{signer: localSigner, Intermediates: intermediates, Digest: digest, Certs: rootCertBytes, Pool: pool}, nil
}

// ValidateCertChain checks checks that the certificates provided chain up to the root pool provided.  In addition
// it also enforces that every cert in the bundle certificates form a chain, each one certifying the one above,
// as per RFC5246 section 7.4.2, and that every certificate (whether or not it is necessary to form a chain to the root
// pool) is currently valid and not yet expired (unless allowExpiry is set to true).
// This is additional validation not required by go's Certificate.Verify (which allows invalid certs in the
// intermediate pool), because this function is intended to be used when reading certs from untrusted locations such as
// from disk or over a network when a CSR is signed, so it is extra pedantic.
// This function always returns all the parsed certificates in the bundle in order, which means there will always be
// at least 1 certificate if there is no error, and the valid chains found by Certificate.Verify
func ValidateCertChain(rootPool *x509.CertPool, certs []byte, allowExpired bool) ([]*x509.Certificate, [][]*x509.Certificate, error) {
	// Parse all the certificates in the cert bundle
	parsedCerts, err := helpers.ParseCertificatesPEM(certs)
	if err != nil {
		return nil, nil, err
	}
	if len(parsedCerts) == 0 {
		return nil, nil, errors.New("no certificates to validate")
	}
	now := time.Now()
	// ensure that they form a chain, each one being signed by the one after it
	var intermediatePool *x509.CertPool
	for i, cert := range parsedCerts {
		// Manual expiry validation because we want more information on which certificate in the chain is expired, and
		// because this is an easier way to allow expired certs.
		if now.Before(cert.NotBefore) {
			return nil, nil, errors.Wrapf(
				x509.CertificateInvalidError{
					Cert:   cert,
					Reason: x509.Expired,
				},
				"certificate (%d - %s) not valid before %s, and it is currently %s",
				i+1, cert.Subject.CommonName, cert.NotBefore.UTC().Format(time.RFC1123), now.Format(time.RFC1123))
		}
		if !allowExpired && now.After(cert.NotAfter) {
			return nil, nil, errors.Wrapf(
				x509.CertificateInvalidError{
					Cert:   cert,
					Reason: x509.Expired,
				},
				"certificate (%d - %s) not valid after %s, and it is currently %s",
				i+1, cert.Subject.CommonName, cert.NotAfter.UTC().Format(time.RFC1123), now.Format(time.RFC1123))
		}

		if i > 0 {
			// check that the previous cert was signed by this cert
			prevCert := parsedCerts[i-1]
			if err := prevCert.CheckSignatureFrom(cert); err != nil {
				return nil, nil, errors.Wrapf(err, "certificates do not form a chain: (%d - %s) is not signed by (%d - %s)",
					i, prevCert.Subject.CommonName, i+1, cert.Subject.CommonName)
			}

			if intermediatePool == nil {
				intermediatePool = x509.NewCertPool()
			}
			intermediatePool.AddCert(cert)

		}
	}

	verifyOpts := x509.VerifyOptions{
		Roots:         rootPool,
		Intermediates: intermediatePool,
		CurrentTime:   now,
	}

	var chains [][]*x509.Certificate

	// If we accept expired certs, try to build a valid cert chain using some subset of the certs.  We start off using the
	// first certificate's NotAfter as the current time, thus ensuring that the first cert is not expired. If the chain
	// still fails to validate due to expiry issues, continue iterating over the rest of the certs.
	// If any of the other certs has an earlier NotAfter time, use that time as the current time instead. This insures that
	// particular cert, and any that came before it, are not expired.  Note that the root that the certs chain up to
	// should also not be expired at that "current" time.
	if allowExpired {
		verifyOpts.CurrentTime = parsedCerts[0].NotAfter.Add(time.Hour)
		for _, cert := range parsedCerts {
			if !cert.NotAfter.Before(verifyOpts.CurrentTime) {
				continue
			}
			verifyOpts.CurrentTime = cert.NotAfter

			chains, err = parsedCerts[0].Verify(verifyOpts)
			if err == nil {
				return parsedCerts, chains, nil
			}
		}
		if invalid, ok := err.(x509.CertificateInvalidError); ok && invalid.Reason == x509.Expired {
			return nil, nil, errors.New("there is no time span for which all of the certificates, including a root, are valid")
		}
		return nil, nil, err
	}

	chains, err = parsedCerts[0].Verify(verifyOpts)
	if err != nil {
		return nil, nil, err
	}
	return parsedCerts, chains, nil
}

// newLocalSigner validates the signing cert and signing key to create a local signer, which accepts a crypto signer and a cert
func newLocalSigner(keyBytes, certBytes []byte, certExpiry time.Duration, rootPool, intermediatePool *x509.CertPool) (*LocalSigner, error) {
	if len(keyBytes) == 0 || len(certBytes) == 0 {
		return nil, errors.New("must provide both a signing key and a signing cert, or neither")
	}

	parsedCerts, err := helpers.ParseCertificatesPEM(certBytes)
	if err != nil {
		return nil, errors.Wrap(err, "invalid signing CA cert")
	}
	if len(parsedCerts) == 0 {
		return nil, errors.New("no valid signing CA certificates found")
	}
	if err := validateSignatureAlgorithm(parsedCerts[0]); err != nil {
		return nil, err
	}
	opts := x509.VerifyOptions{
		Roots:         rootPool,
		Intermediates: intermediatePool,
	}
	if _, err := parsedCerts[0].Verify(opts); err != nil {
		return nil, errors.Wrap(err, "error while validating signing CA certificate against roots and intermediates")
	}

	var (
		passphraseStr              string
		passphrase, passphrasePrev []byte
		priv                       crypto.Signer
	)

	// Attempt two distinct passphrases, so we can do a hitless passphrase rotation
	if passphraseStr = os.Getenv(PassphraseENVVar); passphraseStr != "" {
		passphrase = []byte(passphraseStr)
	}

	if p := os.Getenv(PassphraseENVVarPrev); p != "" {
		passphrasePrev = []byte(p)
	}

	// Attempt to decrypt the current private-key with the passphrases provided
	priv, err = helpers.ParsePrivateKeyPEMWithPassword(keyBytes, passphrase)
	if err != nil {
		priv, err = helpers.ParsePrivateKeyPEMWithPassword(keyBytes, passphrasePrev)
		if err != nil {
			return nil, errors.Wrap(err, "malformed private key")
		}
	}

	// We will always use the first certificate inside of the root bundle as the active one
	if err := ensureCertKeyMatch(parsedCerts[0], priv.Public()); err != nil {
		return nil, err
	}

	signer, err := local.NewSigner(priv, parsedCerts[0], cfsigner.DefaultSigAlgo(priv), SigningPolicy(certExpiry))
	if err != nil {
		return nil, err
	}

	// If the key was loaded from disk unencrypted, but there is a passphrase set,
	// ensure it is encrypted, so it doesn't hit raft in plain-text
	// we don't have to check for nil, because if we couldn't pem-decode the bytes, then parsing above would have failed
	keyBlock, _ := pem.Decode(keyBytes)
	if passphraseStr != "" && !x509.IsEncryptedPEMBlock(keyBlock) {
		keyBytes, err = EncryptECPrivateKey(keyBytes, passphraseStr)
		if err != nil {
			return nil, errors.Wrap(err, "unable to encrypt signing CA key material")
		}
	}

	return &LocalSigner{Cert: certBytes, Key: keyBytes, Signer: signer, parsedCert: parsedCerts[0], cryptoSigner: priv}, nil
}

func ensureCertKeyMatch(cert *x509.Certificate, key crypto.PublicKey) error {
	switch certPub := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		if certPub.N.BitLen() < 2048 || certPub.E == 1 {
			return errors.New("unsupported RSA key parameters")
		}
		rsaKey, ok := key.(*rsa.PublicKey)
		if ok && certPub.E == rsaKey.E && certPub.N.Cmp(rsaKey.N) == 0 {
			return nil
		}
	case *ecdsa.PublicKey:
		switch certPub.Curve {
		case elliptic.P256(), elliptic.P384(), elliptic.P521():
			break
		default:
			return errors.New("unsupported ECDSA key parameters")
		}

		ecKey, ok := key.(*ecdsa.PublicKey)
		if ok && certPub.X.Cmp(ecKey.X) == 0 && certPub.Y.Cmp(ecKey.Y) == 0 {
			return nil
		}
	default:
		return errors.New("unknown or unsupported certificate public key algorithm")
	}

	return errors.New("certificate key mismatch")
}

// GetLocalRootCA validates if the contents of the file are a valid self-signed
// CA certificate, and returns the PEM-encoded Certificate if so
func GetLocalRootCA(paths CertPaths) (RootCA, error) {
	// Check if we have a Certificate file
	cert, err := ioutil.ReadFile(paths.Cert)
	if err != nil {
		if os.IsNotExist(err) {
			err = ErrNoLocalRootCA
		}

		return RootCA{}, err
	}
	signingCert := cert

	key, err := ioutil.ReadFile(paths.Key)
	if err != nil {
		if !os.IsNotExist(err) {
			return RootCA{}, err
		}
		// There may not be a local key. It's okay to pass in a nil
		// key. We'll get a root CA without a signer.
		key = nil
		signingCert = nil
	}

	return NewRootCA(cert, signingCert, key, DefaultNodeCertExpiration, nil)
}

func getGRPCConnection(creds credentials.TransportCredentials, connBroker *connectionbroker.Broker, forceRemote bool) (*connectionbroker.Conn, error) {
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithTimeout(5 * time.Second),
		grpc.WithBackoffMaxDelay(5 * time.Second),
	}
	if forceRemote {
		return connBroker.SelectRemote(dialOpts...)
	}
	return connBroker.Select(dialOpts...)
}

// GetRemoteCA returns the remote endpoint's CA certificate bundle
func GetRemoteCA(ctx context.Context, d digest.Digest, connBroker *connectionbroker.Broker) (RootCA, error) {
	// This TLS Config is intentionally using InsecureSkipVerify. We use the
	// digest instead to check the integrity of the CA certificate.
	insecureCreds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
	conn, err := getGRPCConnection(insecureCreds, connBroker, false)
	if err != nil {
		return RootCA{}, err
	}

	client := api.NewCAClient(conn.ClientConn)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	defer func() {
		conn.Close(err == nil)
	}()
	response, err := client.GetRootCACertificate(ctx, &api.GetRootCACertificateRequest{})
	if err != nil {
		return RootCA{}, err
	}

	// If a bundle of certificates are provided, the digest covers the entire bundle and not just
	// one of the certificates in the bundle.  Otherwise, a node can be MITMed while joining if
	// the MITM CA provides a single certificate which matches the digest, and providing arbitrary
	// other non-verified root certs that the manager certificate actually chains up to.
	if d != "" {
		verifier := d.Verifier()
		if err != nil {
			return RootCA{}, errors.Wrap(err, "unexpected error getting digest verifier")
		}

		io.Copy(verifier, bytes.NewReader(response.Certificate))

		if !verifier.Verified() {
			return RootCA{}, errors.Errorf("remote CA does not match fingerprint. Expected: %s", d.Hex())
		}
	}

	// NewRootCA will validate that the certificates are otherwise valid and create a RootCA object.
	// Since there is no key, the certificate expiry does not matter and will not be used.
	return NewRootCA(response.Certificate, nil, nil, DefaultNodeCertExpiration, nil)
}

// CreateRootCA creates a Certificate authority for a new Swarm Cluster, potentially
// overwriting any existing CAs.
func CreateRootCA(rootCN string) (RootCA, error) {
	// Create a simple CSR for the CA using the default CA validator and policy
	req := cfcsr.CertificateRequest{
		CN:         rootCN,
		KeyRequest: &cfcsr.BasicKeyRequest{A: RootKeyAlgo, S: RootKeySize},
		CA:         &cfcsr.CAConfig{Expiry: RootCAExpiration},
	}

	// Generate the CA and get the certificate and private key
	cert, _, key, err := initca.New(&req)
	if err != nil {
		return RootCA{}, err
	}

	rootCA, err := NewRootCA(cert, cert, key, DefaultNodeCertExpiration, nil)
	if err != nil {
		return RootCA{}, err
	}

	return rootCA, nil
}

// GetRemoteSignedCertificate submits a CSR to a remote CA server address,
// and that is part of a CA identified by a specific certificate pool.
func GetRemoteSignedCertificate(ctx context.Context, csr []byte, rootCAPool *x509.CertPool, config CertificateRequestConfig) ([]byte, error) {
	if rootCAPool == nil {
		return nil, errors.New("valid root CA pool required")
	}
	creds := config.Credentials

	if creds == nil {
		// This is our only non-MTLS request, and it happens when we are boostraping our TLS certs
		// We're using CARole as server name, so an external CA doesn't also have to have ManagerRole in the cert SANs
		creds = credentials.NewTLS(&tls.Config{ServerName: CARole, RootCAs: rootCAPool})
	}

	conn, err := getGRPCConnection(creds, config.ConnBroker, config.ForceRemote)
	if err != nil {
		return nil, err
	}

	// Create a CAClient to retrieve a new Certificate
	caClient := api.NewNodeCAClient(conn.ClientConn)

	issueCtx, issueCancel := context.WithTimeout(ctx, 5*time.Second)
	defer issueCancel()

	// Send the Request and retrieve the request token
	issueRequest := &api.IssueNodeCertificateRequest{CSR: csr, Token: config.Token, Availability: config.Availability}
	issueResponse, err := caClient.IssueNodeCertificate(issueCtx, issueRequest)
	if err != nil {
		conn.Close(false)
		return nil, err
	}

	statusRequest := &api.NodeCertificateStatusRequest{NodeID: issueResponse.NodeID}
	expBackoff := events.NewExponentialBackoff(events.ExponentialBackoffConfig{
		Base:   time.Second,
		Factor: time.Second,
		Max:    30 * time.Second,
	})

	// Exponential backoff with Max of 30 seconds to wait for a new retry
	for {
		timeout := 5 * time.Second
		if config.NodeCertificateStatusRequestTimeout > 0 {
			timeout = config.NodeCertificateStatusRequestTimeout
		}
		// Send the Request and retrieve the certificate
		stateCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		statusResponse, err := caClient.NodeCertificateStatus(stateCtx, statusRequest)
		switch {
		case err != nil && grpc.Code(err) != codes.DeadlineExceeded:
			conn.Close(false)
			// Because IssueNodeCertificate succeeded, if this call failed likely it is due to an issue with this
			// particular connection, so we need to get another.  We should try a remote connection - the local node
			// may be a manager that was demoted, so the local connection (which is preferred) may not work.
			config.ForceRemote = true
			conn, err = getGRPCConnection(creds, config.ConnBroker, config.ForceRemote)
			if err != nil {
				return nil, err
			}
			caClient = api.NewNodeCAClient(conn.ClientConn)

		// If there was no deadline exceeded error, and the certificate was issued, return
		case err == nil && statusResponse.Status.State == api.IssuanceStateIssued:
			if statusResponse.Certificate == nil {
				conn.Close(false)
				return nil, errors.New("no certificate in CertificateStatus response")
			}

			// The certificate in the response must match the CSR
			// we submitted. If we are getting a response for a
			// certificate that was previously issued, we need to
			// retry until the certificate gets updated per our
			// current request.
			if bytes.Equal(statusResponse.Certificate.CSR, csr) {
				conn.Close(true)
				return statusResponse.Certificate.Certificate, nil
			}
		}

		// If NodeCertificateStatus timed out, we're still pending, the issuance failed, or
		// the state is unknown let's continue trying after an exponential backoff
		expBackoff.Failure(nil, nil)
		select {
		case <-ctx.Done():
			conn.Close(true)
			return nil, err
		case <-time.After(expBackoff.Proceed(nil)):
		}
	}
}

// readCertValidity returns the certificate issue and expiration time
func readCertValidity(kr KeyReader) (time.Time, time.Time, error) {
	var zeroTime time.Time
	// Read the Cert
	cert, _, err := kr.Read()
	if err != nil {
		return zeroTime, zeroTime, err
	}

	// Create an x509 certificate out of the contents on disk
	certBlock, _ := pem.Decode(cert)
	if certBlock == nil {
		return zeroTime, zeroTime, errors.New("failed to decode certificate block")
	}
	X509Cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return zeroTime, zeroTime, err
	}

	return X509Cert.NotBefore, X509Cert.NotAfter, nil

}

// SaveRootCA saves a RootCA object to disk
func SaveRootCA(rootCA RootCA, paths CertPaths) error {
	// Make sure the necessary dirs exist and they are writable
	err := os.MkdirAll(filepath.Dir(paths.Cert), 0755)
	if err != nil {
		return err
	}

	// If the root certificate got returned successfully, save the rootCA to disk.
	return ioutils.AtomicWriteFile(paths.Cert, rootCA.Certs, 0644)
}

// GenerateNewCSR returns a newly generated key and CSR signed with said key
func GenerateNewCSR() ([]byte, []byte, error) {
	req := &cfcsr.CertificateRequest{
		KeyRequest: cfcsr.NewBasicKeyRequest(),
	}
	return cfcsr.ParseRequest(req)
}

// EncryptECPrivateKey receives a PEM encoded private key and returns an encrypted
// AES256 version using a passphrase
// TODO: Make this method generic to handle RSA keys
func EncryptECPrivateKey(key []byte, passphraseStr string) ([]byte, error) {
	passphrase := []byte(passphraseStr)
	cipherType := x509.PEMCipherAES256

	keyBlock, _ := pem.Decode(key)
	if keyBlock == nil {
		// This RootCA does not have a valid signer.
		return nil, errors.New("error while decoding PEM key")
	}

	encryptedPEMBlock, err := x509.EncryptPEMBlock(cryptorand.Reader,
		"EC PRIVATE KEY",
		keyBlock.Bytes,
		passphrase,
		cipherType)
	if err != nil {
		return nil, err
	}

	if encryptedPEMBlock.Headers == nil {
		return nil, errors.New("unable to encrypt key - invalid PEM file produced")
	}

	return pem.EncodeToMemory(encryptedPEMBlock), nil
}

// NormalizePEMs takes a bundle of PEM-encoded certificates in a certificate bundle,
// decodes them, removes headers, and re-encodes them to make sure that they have
// consistent whitespace.  Note that this is intended to normalize x509 certificates
// in PEM format, hence the stripping out of headers.
func NormalizePEMs(certs []byte) []byte {
	var (
		results  []byte
		pemBlock *pem.Block
	)
	for {
		pemBlock, certs = pem.Decode(bytes.TrimSpace(certs))
		if pemBlock == nil {
			return results
		}
		pemBlock.Headers = nil
		results = append(results, pem.EncodeToMemory(pemBlock)...)
	}
}
