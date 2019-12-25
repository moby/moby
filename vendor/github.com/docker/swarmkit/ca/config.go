package ca

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math/big"
	"math/rand"
	"path/filepath"
	"strings"
	"sync"
	"time"

	cfconfig "github.com/cloudflare/cfssl/config"
	events "github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/connectionbroker"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/watch"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/credentials"
)

const (
	rootCACertFilename  = "swarm-root-ca.crt"
	rootCAKeyFilename   = "swarm-root-ca.key"
	nodeTLSCertFilename = "swarm-node.crt"
	nodeTLSKeyFilename  = "swarm-node.key"

	// DefaultRootCN represents the root CN that we should create roots CAs with by default
	DefaultRootCN = "swarm-ca"
	// ManagerRole represents the Manager node type, and is used for authorization to endpoints
	ManagerRole = "swarm-manager"
	// WorkerRole represents the Worker node type, and is used for authorization to endpoints
	WorkerRole = "swarm-worker"
	// CARole represents the CA node type, and is used for clients attempting to get new certificates issued
	CARole = "swarm-ca"

	generatedSecretEntropyBytes = 16
	joinTokenBase               = 36
	// ceil(log(2^128-1, 36))
	maxGeneratedSecretLength = 25
	// ceil(log(2^256-1, 36))
	base36DigestLen = 50
)

var (
	// GetCertRetryInterval is how long to wait before retrying a node
	// certificate or root certificate request.
	GetCertRetryInterval = 2 * time.Second

	// errInvalidJoinToken is returned when attempting to parse an invalid join
	// token (e.g. when attempting to get the version, fipsness, or the root ca
	// digest)
	errInvalidJoinToken = errors.New("invalid join token")
)

// SecurityConfig is used to represent a node's security configuration. It includes information about
// the RootCA and ServerTLSCreds/ClientTLSCreds transport authenticators to be used for MTLS
type SecurityConfig struct {
	// mu protects against concurrent access to fields inside the structure.
	mu sync.Mutex

	// renewalMu makes sure only one certificate renewal attempt happens at
	// a time. It should never be locked after mu is already locked.
	renewalMu sync.Mutex

	rootCA        *RootCA
	keyReadWriter *KeyReadWriter

	certificate *tls.Certificate
	issuerInfo  *IssuerInfo

	ServerTLSCreds *MutableTLSCreds
	ClientTLSCreds *MutableTLSCreds

	// An optional queue for anyone interested in subscribing to SecurityConfig updates
	queue *watch.Queue
}

// CertificateUpdate represents a change in the underlying TLS configuration being returned by
// a certificate renewal event.
type CertificateUpdate struct {
	Role string
	Err  error
}

// ParsedJoinToken is the data from a join token, once parsed
type ParsedJoinToken struct {
	// Version is the version of the join token that is being parsed
	Version int

	// RootDigest is the digest of the root CA certificate of the cluster, which
	// is always part of the join token so that the root CA can be verified
	// upon initial node join
	RootDigest digest.Digest

	// Secret is the randomly-generated secret part of the join token - when
	// rotating a join token, this is the value that is changed unless some other
	// property of the cluster (like the root CA) is changed.
	Secret string

	// FIPS indicates whether the join token specifies that the cluster mandates
	// that all nodes must have FIPS mode enabled.
	FIPS bool
}

// ParseJoinToken parses a join token.  Current format is v2, but this is currently used only if the cluster requires
// mandatory FIPS, in order to facilitate mixed version clusters.
// v1: SWMTKN-1-<SHA256 digest of root CA cert in base 36, 0-left-padded to 50 chars>-<16-byte secret in base 36 0-left-padded to 25 chars>
// v2: SWMTKN-2-<0/1 whether its FIPS or not>-<same rest of data as v1>
func ParseJoinToken(token string) (*ParsedJoinToken, error) {
	split := strings.Split(token, "-")
	numParts := len(split)

	// v1 has 4, v2 has 5
	if numParts < 4 || split[0] != "SWMTKN" {
		return nil, errInvalidJoinToken
	}

	var (
		version int
		fips    bool
	)

	switch split[1] {
	case "1":
		if numParts != 4 {
			return nil, errInvalidJoinToken
		}
		version = 1
	case "2":
		if numParts != 5 || (split[2] != "0" && split[2] != "1") {
			return nil, errInvalidJoinToken
		}
		version = 2
		fips = split[2] == "1"
	default:
		return nil, errInvalidJoinToken
	}

	secret := split[numParts-1]
	rootDigest := split[numParts-2]
	if len(rootDigest) != base36DigestLen || len(secret) != maxGeneratedSecretLength {
		return nil, errInvalidJoinToken
	}

	var digestInt big.Int
	digestInt.SetString(rootDigest, joinTokenBase)

	d, err := digest.Parse(fmt.Sprintf("sha256:%0[1]*s", 64, digestInt.Text(16)))
	if err != nil {
		return nil, err
	}
	return &ParsedJoinToken{
		Version:    version,
		RootDigest: d,
		Secret:     secret,
		FIPS:       fips,
	}, nil
}

func validateRootCAAndTLSCert(rootCA *RootCA, tlsKeyPair *tls.Certificate) error {
	var (
		leafCert         *x509.Certificate
		intermediatePool *x509.CertPool
	)
	for i, derBytes := range tlsKeyPair.Certificate {
		parsed, err := x509.ParseCertificate(derBytes)
		if err != nil {
			return errors.Wrap(err, "could not validate new root certificates due to parse error")
		}
		if i == 0 {
			leafCert = parsed
		} else {
			if intermediatePool == nil {
				intermediatePool = x509.NewCertPool()
			}
			intermediatePool.AddCert(parsed)
		}
	}
	opts := x509.VerifyOptions{
		Roots:         rootCA.Pool,
		Intermediates: intermediatePool,
	}
	if _, err := leafCert.Verify(opts); err != nil {
		return errors.Wrap(err, "new root CA does not match existing TLS credentials")
	}
	return nil
}

// NewSecurityConfig initializes and returns a new SecurityConfig.
func NewSecurityConfig(rootCA *RootCA, krw *KeyReadWriter, tlsKeyPair *tls.Certificate, issuerInfo *IssuerInfo) (*SecurityConfig, func() error, error) {
	// Create the Server TLS Credentials for this node. These will not be used by workers.
	serverTLSCreds, err := rootCA.NewServerTLSCredentials(tlsKeyPair)
	if err != nil {
		return nil, nil, err
	}

	// Create a TLSConfig to be used when this node connects as a client to another remote node.
	// We're using ManagerRole as remote serverName for TLS host verification because both workers
	// and managers always connect to remote managers.
	clientTLSCreds, err := rootCA.NewClientTLSCredentials(tlsKeyPair, ManagerRole)
	if err != nil {
		return nil, nil, err
	}

	q := watch.NewQueue()
	return &SecurityConfig{
		rootCA:        rootCA,
		keyReadWriter: krw,

		certificate: tlsKeyPair,
		issuerInfo:  issuerInfo,
		queue:       q,

		ClientTLSCreds: clientTLSCreds,
		ServerTLSCreds: serverTLSCreds,
	}, q.Close, nil
}

// RootCA returns the root CA.
func (s *SecurityConfig) RootCA() *RootCA {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.rootCA
}

// KeyWriter returns the object that can write keys to disk
func (s *SecurityConfig) KeyWriter() KeyWriter {
	return s.keyReadWriter
}

// KeyReader returns the object that can read keys from disk
func (s *SecurityConfig) KeyReader() KeyReader {
	return s.keyReadWriter
}

// UpdateRootCA replaces the root CA with a new root CA
func (s *SecurityConfig) UpdateRootCA(rootCA *RootCA) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// refuse to update the root CA if the current TLS credentials do not validate against it
	if err := validateRootCAAndTLSCert(rootCA, s.certificate); err != nil {
		return err
	}

	s.rootCA = rootCA
	return s.updateTLSCredentials(s.certificate, s.issuerInfo)
}

// Watch allows you to set a watch on the security config, in order to be notified of any changes
func (s *SecurityConfig) Watch() (chan events.Event, func()) {
	return s.queue.Watch()
}

// IssuerInfo returns the issuer subject and issuer public key
func (s *SecurityConfig) IssuerInfo() *IssuerInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.issuerInfo
}

// This function expects something else to have taken out a lock on the SecurityConfig.
func (s *SecurityConfig) updateTLSCredentials(certificate *tls.Certificate, issuerInfo *IssuerInfo) error {
	certs := []tls.Certificate{*certificate}
	clientConfig, err := NewClientTLSConfig(certs, s.rootCA.Pool, ManagerRole)
	if err != nil {
		return errors.Wrap(err, "failed to create a new client config using the new root CA")
	}

	serverConfig, err := NewServerTLSConfig(certs, s.rootCA.Pool)
	if err != nil {
		return errors.Wrap(err, "failed to create a new server config using the new root CA")
	}

	if err := s.ClientTLSCreds.loadNewTLSConfig(clientConfig); err != nil {
		return errors.Wrap(err, "failed to update the client credentials")
	}

	if err := s.ServerTLSCreds.loadNewTLSConfig(serverConfig); err != nil {
		return errors.Wrap(err, "failed to update the server TLS credentials")
	}

	s.certificate = certificate
	s.issuerInfo = issuerInfo
	if s.queue != nil {
		s.queue.Publish(&api.NodeTLSInfo{
			TrustRoot:           s.rootCA.Certs,
			CertIssuerPublicKey: s.issuerInfo.PublicKey,
			CertIssuerSubject:   s.issuerInfo.Subject,
		})
	}
	return nil
}

// UpdateTLSCredentials updates the security config with an updated TLS certificate and issuer info
func (s *SecurityConfig) UpdateTLSCredentials(certificate *tls.Certificate, issuerInfo *IssuerInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updateTLSCredentials(certificate, issuerInfo)
}

// SigningPolicy creates a policy used by the signer to ensure that the only fields
// from the remote CSRs we trust are: PublicKey, PublicKeyAlgorithm and SignatureAlgorithm.
// It receives the duration a certificate will be valid for
func SigningPolicy(certExpiry time.Duration) *cfconfig.Signing {
	// Force the minimum Certificate expiration to be fifteen minutes
	if certExpiry < MinNodeCertExpiration {
		certExpiry = DefaultNodeCertExpiration
	}

	// Add the backdate
	certExpiry = certExpiry + CertBackdate

	return &cfconfig.Signing{
		Default: &cfconfig.SigningProfile{
			Usage:    []string{"signing", "key encipherment", "server auth", "client auth"},
			Expiry:   certExpiry,
			Backdate: CertBackdate,
			// Only trust the key components from the CSR. Everything else should
			// come directly from API call params.
			CSRWhitelist: &cfconfig.CSRWhitelist{
				PublicKey:          true,
				PublicKeyAlgorithm: true,
				SignatureAlgorithm: true,
			},
		},
	}
}

// SecurityConfigPaths is used as a helper to hold all the paths of security relevant files
type SecurityConfigPaths struct {
	Node, RootCA CertPaths
}

// NewConfigPaths returns the absolute paths to all of the different types of files
func NewConfigPaths(baseCertDir string) *SecurityConfigPaths {
	return &SecurityConfigPaths{
		Node: CertPaths{
			Cert: filepath.Join(baseCertDir, nodeTLSCertFilename),
			Key:  filepath.Join(baseCertDir, nodeTLSKeyFilename)},
		RootCA: CertPaths{
			Cert: filepath.Join(baseCertDir, rootCACertFilename),
			Key:  filepath.Join(baseCertDir, rootCAKeyFilename)},
	}
}

// GenerateJoinToken creates a new join token. Current format is v2, but this is
// currently used only if the cluster requires mandatory FIPS, in order to
// facilitate mixed version clusters (the `fips` parameter is set to true).
// Otherwise, v1 is used so as to maintain compatibility in mixed version
// non-FIPS clusters.
// v1: SWMTKN-1-<SHA256 digest of root CA cert in base 36, 0-left-padded to 50 chars>-<16-byte secret in base 36 0-left-padded to 25 chars>
// v2: SWMTKN-2-<0/1 whether its FIPS or not>-<same rest of data as v1>
func GenerateJoinToken(rootCA *RootCA, fips bool) string {
	var secretBytes [generatedSecretEntropyBytes]byte

	if _, err := cryptorand.Read(secretBytes[:]); err != nil {
		panic(fmt.Errorf("failed to read random bytes: %v", err))
	}

	var nn, digest big.Int
	nn.SetBytes(secretBytes[:])
	digest.SetString(rootCA.Digest.Hex(), 16)

	fmtString := "SWMTKN-1-%0[1]*s-%0[3]*s"
	if fips {
		fmtString = "SWMTKN-2-1-%0[1]*s-%0[3]*s"
	}
	return fmt.Sprintf(fmtString, base36DigestLen,
		digest.Text(joinTokenBase), maxGeneratedSecretLength, nn.Text(joinTokenBase))
}

// DownloadRootCA tries to retrieve a remote root CA and matches the digest against the provided token.
func DownloadRootCA(ctx context.Context, paths CertPaths, token string, connBroker *connectionbroker.Broker) (RootCA, error) {
	var rootCA RootCA
	// Get a digest for the optional CA hash string that we've been provided
	// If we were provided a non-empty string, and it is an invalid hash, return
	// otherwise, allow the invalid digest through.
	var (
		d   digest.Digest
		err error
	)
	if token != "" {
		parsed, err := ParseJoinToken(token)
		if err != nil {
			return RootCA{}, err
		}
		d = parsed.RootDigest
	}
	// Get the remote CA certificate, verify integrity with the
	// hash provided. Retry up to 5 times, in case the manager we
	// first try to contact is not responding properly (it may have
	// just been demoted, for example).

	for i := 0; i != 5; i++ {
		rootCA, err = GetRemoteCA(ctx, d, connBroker)
		if err == nil {
			break
		}
		log.G(ctx).WithError(err).Errorf("failed to retrieve remote root CA certificate")

		select {
		case <-time.After(GetCertRetryInterval):
		case <-ctx.Done():
			return RootCA{}, ctx.Err()
		}
	}
	if err != nil {
		return RootCA{}, err
	}

	// Save root CA certificate to disk
	if err = SaveRootCA(rootCA, paths); err != nil {
		return RootCA{}, err
	}

	log.G(ctx).Debugf("retrieved remote CA certificate: %s", paths.Cert)
	return rootCA, nil
}

// LoadSecurityConfig loads TLS credentials from disk, or returns an error if
// these credentials do not exist or are unusable.
func LoadSecurityConfig(ctx context.Context, rootCA RootCA, krw *KeyReadWriter, allowExpired bool) (*SecurityConfig, func() error, error) {
	ctx = log.WithModule(ctx, "tls")

	// At this point we've successfully loaded the CA details from disk, or
	// successfully downloaded them remotely. The next step is to try to
	// load our certificates.

	// Read both the Cert and Key from disk
	cert, key, err := krw.Read()
	if err != nil {
		return nil, nil, err
	}

	// Check to see if this certificate was signed by our CA, and isn't expired
	_, chains, err := ValidateCertChain(rootCA.Pool, cert, allowExpired)
	if err != nil {
		return nil, nil, err
	}
	// ValidateChain, if successful, will always return at least 1 chain containing
	// at least 2 certificates:  the leaf and the root.
	issuer := chains[0][1]

	// Now that we know this certificate is valid, create a TLS Certificate for our
	// credentials
	keyPair, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, nil, err
	}

	secConfig, cleanup, err := NewSecurityConfig(&rootCA, krw, &keyPair, &IssuerInfo{
		Subject:   issuer.RawSubject,
		PublicKey: issuer.RawSubjectPublicKeyInfo,
	})
	if err == nil {
		log.G(ctx).WithFields(logrus.Fields{
			"node.id":   secConfig.ClientTLSCreds.NodeID(),
			"node.role": secConfig.ClientTLSCreds.Role(),
		}).Debug("loaded node credentials")
	}
	return secConfig, cleanup, err
}

// CertificateRequestConfig contains the information needed to request a
// certificate from a remote CA.
type CertificateRequestConfig struct {
	// Token is the join token that authenticates us with the CA.
	Token string
	// Availability allows a user to control the current scheduling status of a node
	Availability api.NodeSpec_Availability
	// ConnBroker provides connections to CAs.
	ConnBroker *connectionbroker.Broker
	// Credentials provides transport credentials for communicating with the
	// remote server.
	Credentials credentials.TransportCredentials
	// ForceRemote specifies that only a remote (TCP) connection should
	// be used to request the certificate. This may be necessary in cases
	// where the local node is running a manager, but is in the process of
	// being demoted.
	ForceRemote bool
	// NodeCertificateStatusRequestTimeout determines how long to wait for a node
	// status RPC result.  If not provided (zero value), will default to 5 seconds.
	NodeCertificateStatusRequestTimeout time.Duration
	// RetryInterval specifies how long to delay between retries, if non-zero.
	RetryInterval time.Duration
	// Organization is the organization to use for a TLS certificate when creating
	// a security config from scratch.  If not provided, a random ID is generated.
	// For swarm certificates, the organization is the cluster ID.
	Organization string
}

// CreateSecurityConfig creates a new key and cert for this node, either locally
// or via a remote CA.
func (rootCA RootCA) CreateSecurityConfig(ctx context.Context, krw *KeyReadWriter, config CertificateRequestConfig) (*SecurityConfig, func() error, error) {
	ctx = log.WithModule(ctx, "tls")

	// Create a new random ID for this certificate
	cn := identity.NewID()
	org := config.Organization
	if config.Organization == "" {
		org = identity.NewID()
	}

	proposedRole := ManagerRole
	tlsKeyPair, issuerInfo, err := rootCA.IssueAndSaveNewCertificates(krw, cn, proposedRole, org)
	switch errors.Cause(err) {
	case ErrNoValidSigner:
		config.RetryInterval = GetCertRetryInterval
		// Request certificate issuance from a remote CA.
		// Last argument is nil because at this point we don't have any valid TLS creds
		tlsKeyPair, issuerInfo, err = rootCA.RequestAndSaveNewCertificates(ctx, krw, config)
		if err != nil {
			log.G(ctx).WithError(err).Error("failed to request and save new certificate")
			return nil, nil, err
		}
	case nil:
		log.G(ctx).WithFields(logrus.Fields{
			"node.id":   cn,
			"node.role": proposedRole,
		}).Debug("issued new TLS certificate")
	default:
		log.G(ctx).WithFields(logrus.Fields{
			"node.id":   cn,
			"node.role": proposedRole,
		}).WithError(err).Errorf("failed to issue and save new certificate")
		return nil, nil, err
	}

	secConfig, cleanup, err := NewSecurityConfig(&rootCA, krw, tlsKeyPair, issuerInfo)
	if err == nil {
		log.G(ctx).WithFields(logrus.Fields{
			"node.id":   secConfig.ClientTLSCreds.NodeID(),
			"node.role": secConfig.ClientTLSCreds.Role(),
		}).Debugf("new node credentials generated: %s", krw.Target())
	}
	return secConfig, cleanup, err
}

// TODO(cyli): currently we have to only update if it's a worker role - if we have a single root CA update path for
// both managers and workers, we won't need to check any more.
func updateRootThenUpdateCert(ctx context.Context, s *SecurityConfig, connBroker *connectionbroker.Broker, rootPaths CertPaths, failedCert *x509.Certificate) (*tls.Certificate, *IssuerInfo, error) {
	if len(failedCert.Subject.OrganizationalUnit) == 0 || failedCert.Subject.OrganizationalUnit[0] != WorkerRole {
		return nil, nil, errors.New("cannot update root CA since this is not a worker")
	}
	// try downloading a new root CA if it's an unknown authority issue, in case there was a root rotation completion
	// and we just didn't get the new root
	rootCA, err := GetRemoteCA(ctx, "", connBroker)
	if err != nil {
		return nil, nil, err
	}
	// validate against the existing security config creds
	if err := s.UpdateRootCA(&rootCA); err != nil {
		return nil, nil, err
	}
	if err := SaveRootCA(rootCA, rootPaths); err != nil {
		return nil, nil, err
	}
	return rootCA.RequestAndSaveNewCertificates(ctx, s.KeyWriter(),
		CertificateRequestConfig{
			ConnBroker:  connBroker,
			Credentials: s.ClientTLSCreds,
		})
}

// RenewTLSConfigNow gets a new TLS cert and key, and updates the security config if provided.  This is similar to
// RenewTLSConfig, except while that monitors for expiry, and periodically renews, this renews once and is blocking
func RenewTLSConfigNow(ctx context.Context, s *SecurityConfig, connBroker *connectionbroker.Broker, rootPaths CertPaths) error {
	s.renewalMu.Lock()
	defer s.renewalMu.Unlock()

	ctx = log.WithModule(ctx, "tls")
	log := log.G(ctx).WithFields(logrus.Fields{
		"node.id":   s.ClientTLSCreds.NodeID(),
		"node.role": s.ClientTLSCreds.Role(),
	})

	// Let's request new certs. Renewals don't require a token.
	rootCA := s.RootCA()
	tlsKeyPair, issuerInfo, err := rootCA.RequestAndSaveNewCertificates(ctx,
		s.KeyWriter(),
		CertificateRequestConfig{
			ConnBroker:  connBroker,
			Credentials: s.ClientTLSCreds,
		})
	if wrappedError, ok := err.(x509UnknownAuthError); ok {
		var newErr error
		tlsKeyPair, issuerInfo, newErr = updateRootThenUpdateCert(ctx, s, connBroker, rootPaths, wrappedError.failedLeafCert)
		if newErr != nil {
			err = wrappedError.error
		} else {
			err = nil
		}
	}
	if err != nil {
		log.WithError(err).Errorf("failed to renew the certificate")
		return err
	}

	return s.UpdateTLSCredentials(tlsKeyPair, issuerInfo)
}

// calculateRandomExpiry returns a random duration between 50% and 80% of the
// original validity period
func calculateRandomExpiry(validFrom, validUntil time.Time) time.Duration {
	duration := validUntil.Sub(validFrom)

	var randomExpiry int
	// Our lower bound of renewal will be half of the total expiration time
	minValidity := int(duration.Minutes() * CertLowerRotationRange)
	// Our upper bound of renewal will be 80% of the total expiration time
	maxValidity := int(duration.Minutes() * CertUpperRotationRange)
	// Let's select a random number of minutes between min and max, and set our retry for that
	// Using randomly selected rotation allows us to avoid certificate thundering herds.
	if maxValidity-minValidity < 1 {
		randomExpiry = minValidity
	} else {
		randomExpiry = rand.Intn(maxValidity-minValidity) + minValidity
	}

	expiry := time.Until(validFrom.Add(time.Duration(randomExpiry) * time.Minute))
	if expiry < 0 {
		return 0
	}
	return expiry
}

// NewServerTLSConfig returns a tls.Config configured for a TLS Server, given a tls.Certificate
// and the PEM-encoded root CA Certificate
func NewServerTLSConfig(certs []tls.Certificate, rootCAPool *x509.CertPool) (*tls.Config, error) {
	if rootCAPool == nil {
		return nil, errors.New("valid root CA pool required")
	}

	return &tls.Config{
		Certificates: certs,
		// Since we're using the same CA server to issue Certificates to new nodes, we can't
		// use tls.RequireAndVerifyClientCert
		ClientAuth:               tls.VerifyClientCertIfGiven,
		RootCAs:                  rootCAPool,
		ClientCAs:                rootCAPool,
		PreferServerCipherSuites: true,
		MinVersion:               tls.VersionTLS12,
	}, nil
}

// NewClientTLSConfig returns a tls.Config configured for a TLS Client, given a tls.Certificate
// the PEM-encoded root CA Certificate, and the name of the remote server the client wants to connect to.
func NewClientTLSConfig(certs []tls.Certificate, rootCAPool *x509.CertPool, serverName string) (*tls.Config, error) {
	if rootCAPool == nil {
		return nil, errors.New("valid root CA pool required")
	}

	return &tls.Config{
		ServerName:   serverName,
		Certificates: certs,
		RootCAs:      rootCAPool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// NewClientTLSCredentials returns GRPC credentials for a TLS GRPC client, given a tls.Certificate
// a PEM-Encoded root CA Certificate, and the name of the remote server the client wants to connect to.
func (rootCA *RootCA) NewClientTLSCredentials(cert *tls.Certificate, serverName string) (*MutableTLSCreds, error) {
	tlsConfig, err := NewClientTLSConfig([]tls.Certificate{*cert}, rootCA.Pool, serverName)
	if err != nil {
		return nil, err
	}

	mtls, err := NewMutableTLS(tlsConfig)

	return mtls, err
}

// NewServerTLSCredentials returns GRPC credentials for a TLS GRPC client, given a tls.Certificate
// a PEM-Encoded root CA Certificate, and the name of the remote server the client wants to connect to.
func (rootCA *RootCA) NewServerTLSCredentials(cert *tls.Certificate) (*MutableTLSCreds, error) {
	tlsConfig, err := NewServerTLSConfig([]tls.Certificate{*cert}, rootCA.Pool)
	if err != nil {
		return nil, err
	}

	mtls, err := NewMutableTLS(tlsConfig)

	return mtls, err
}

// ParseRole parses an apiRole into an internal role string
func ParseRole(apiRole api.NodeRole) (string, error) {
	switch apiRole {
	case api.NodeRoleManager:
		return ManagerRole, nil
	case api.NodeRoleWorker:
		return WorkerRole, nil
	default:
		return "", errors.Errorf("failed to parse api role: %v", apiRole)
	}
}

// FormatRole parses an internal role string into an apiRole
func FormatRole(role string) (api.NodeRole, error) {
	switch strings.ToLower(role) {
	case strings.ToLower(ManagerRole):
		return api.NodeRoleManager, nil
	case strings.ToLower(WorkerRole):
		return api.NodeRoleWorker, nil
	default:
		return 0, errors.Errorf("failed to parse role: %s", role)
	}
}
