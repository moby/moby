package ca

import (
	cryptorand "crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"math/rand"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	cfconfig "github.com/cloudflare/cfssl/config"
	events "github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/connectionbroker"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/log"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"google.golang.org/grpc/credentials"

	"golang.org/x/net/context"
)

const (
	rootCACertFilename  = "swarm-root-ca.crt"
	rootCAKeyFilename   = "swarm-root-ca.key"
	nodeTLSCertFilename = "swarm-node.crt"
	nodeTLSKeyFilename  = "swarm-node.key"
	nodeCSRFilename     = "swarm-node.csr"

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

// RenewTLSExponentialBackoff sets the exponential backoff when trying to renew TLS certificates that have expired
var RenewTLSExponentialBackoff = events.ExponentialBackoffConfig{
	Base:   time.Second * 5,
	Factor: time.Minute,
	Max:    1 * time.Hour,
}

// SecurityConfig is used to represent a node's security configuration. It includes information about
// the RootCA and ServerTLSCreds/ClientTLSCreds transport authenticators to be used for MTLS
type SecurityConfig struct {
	// mu protects against concurrent access to fields inside the structure.
	mu sync.Mutex

	// renewalMu makes sure only one certificate renewal attempt happens at
	// a time. It should never be locked after mu is already locked.
	renewalMu sync.Mutex

	rootCA        *RootCA
	externalCA    *ExternalCA
	keyReadWriter *KeyReadWriter

	ServerTLSCreds *MutableTLSCreds
	ClientTLSCreds *MutableTLSCreds
}

// CertificateUpdate represents a change in the underlying TLS configuration being returned by
// a certificate renewal event.
type CertificateUpdate struct {
	Role string
	Err  error
}

// NewSecurityConfig initializes and returns a new SecurityConfig.
func NewSecurityConfig(rootCA *RootCA, krw *KeyReadWriter, clientTLSCreds, serverTLSCreds *MutableTLSCreds) *SecurityConfig {
	// Make a new TLS config for the external CA client without a
	// ServerName value set.
	clientTLSConfig := clientTLSCreds.Config()

	externalCATLSConfig := &tls.Config{
		Certificates: clientTLSConfig.Certificates,
		RootCAs:      clientTLSConfig.RootCAs,
		MinVersion:   tls.VersionTLS12,
	}

	return &SecurityConfig{
		rootCA:         rootCA,
		keyReadWriter:  krw,
		externalCA:     NewExternalCA(rootCA, externalCATLSConfig),
		ClientTLSCreds: clientTLSCreds,
		ServerTLSCreds: serverTLSCreds,
	}
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

// UpdateRootCA replaces the root CA with a new root CA based on the specified
// certificate, key, and the number of hours the certificates issue should last.
func (s *SecurityConfig) UpdateRootCA(cert, key []byte, certExpiry time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rootCA, err := NewRootCA(cert, key, certExpiry)
	if err == nil {
		s.rootCA = &rootCA
	}

	return err
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

// GenerateJoinToken creates a new join token.
func GenerateJoinToken(rootCA *RootCA) string {
	var secretBytes [generatedSecretEntropyBytes]byte

	if _, err := cryptorand.Read(secretBytes[:]); err != nil {
		panic(fmt.Errorf("failed to read random bytes: %v", err))
	}

	var nn, digest big.Int
	nn.SetBytes(secretBytes[:])
	digest.SetString(rootCA.Digest.Hex(), 16)
	return fmt.Sprintf("SWMTKN-1-%0[1]*s-%0[3]*s", base36DigestLen, digest.Text(joinTokenBase), maxGeneratedSecretLength, nn.Text(joinTokenBase))
}

func getCAHashFromToken(token string) (digest.Digest, error) {
	split := strings.Split(token, "-")
	if len(split) != 4 || split[0] != "SWMTKN" || split[1] != "1" || len(split[2]) != base36DigestLen || len(split[3]) != maxGeneratedSecretLength {
		return "", errors.New("invalid join token")
	}

	var digestInt big.Int
	digestInt.SetString(split[2], joinTokenBase)

	return digest.Parse(fmt.Sprintf("sha256:%0[1]*s", 64, digestInt.Text(16)))
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
		d, err = getCAHashFromToken(token)
		if err != nil {
			return RootCA{}, err
		}
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
	}
	if err != nil {
		return RootCA{}, err
	}

	// Save root CA certificate to disk
	if err = saveRootCA(rootCA, paths); err != nil {
		return RootCA{}, err
	}

	log.G(ctx).Debugf("retrieved remote CA certificate: %s", paths.Cert)
	return rootCA, nil
}

// LoadSecurityConfig loads TLS credentials from disk, or returns an error if
// these credentials do not exist or are unusable.
func LoadSecurityConfig(ctx context.Context, rootCA RootCA, krw *KeyReadWriter, allowExpired bool) (*SecurityConfig, error) {
	ctx = log.WithModule(ctx, "tls")

	// At this point we've successfully loaded the CA details from disk, or
	// successfully downloaded them remotely. The next step is to try to
	// load our certificates.

	// Read both the Cert and Key from disk
	cert, key, err := krw.Read()
	if err != nil {
		return nil, err
	}

	// Create an x509 certificate out of the contents on disk
	certBlock, _ := pem.Decode([]byte(cert))
	if certBlock == nil {
		return nil, errors.New("failed to parse certificate PEM")
	}

	// Create an X509Cert so we can .Verify()
	X509Cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, err
	}

	// Include our root pool
	opts := x509.VerifyOptions{
		Roots: rootCA.Pool,
	}

	// Check to see if this certificate was signed by our CA, and isn't expired
	if err := verifyCertificate(X509Cert, opts, allowExpired); err != nil {
		return nil, err
	}

	// Now that we know this certificate is valid, create a TLS Certificate for our
	// credentials
	keyPair, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}

	// Load the Certificates as server credentials
	serverTLSCreds, err := rootCA.NewServerTLSCredentials(&keyPair)
	if err != nil {
		return nil, err
	}

	// Load the Certificates also as client credentials.
	// Both workers and managers always connect to remote managers,
	// so ServerName is always set to ManagerRole here.
	clientTLSCreds, err := rootCA.NewClientTLSCredentials(&keyPair, ManagerRole)
	if err != nil {
		return nil, err
	}

	log.G(ctx).WithFields(logrus.Fields{
		"node.id":   clientTLSCreds.NodeID(),
		"node.role": clientTLSCreds.Role(),
	}).Debug("loaded node credentials")

	return NewSecurityConfig(&rootCA, krw, clientTLSCreds, serverTLSCreds), nil
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
}

// CreateSecurityConfig creates a new key and cert for this node, either locally
// or via a remote CA.
func (rootCA RootCA) CreateSecurityConfig(ctx context.Context, krw *KeyReadWriter, config CertificateRequestConfig) (*SecurityConfig, error) {
	ctx = log.WithModule(ctx, "tls")

	var (
		tlsKeyPair *tls.Certificate
		err        error
	)

	if rootCA.CanSign() {
		// Create a new random ID for this certificate
		cn := identity.NewID()
		org := identity.NewID()

		proposedRole := ManagerRole
		tlsKeyPair, err = rootCA.IssueAndSaveNewCertificates(krw, cn, proposedRole, org)
		if err != nil {
			log.G(ctx).WithFields(logrus.Fields{
				"node.id":   cn,
				"node.role": proposedRole,
			}).WithError(err).Errorf("failed to issue and save new certificate")
			return nil, err
		}

		log.G(ctx).WithFields(logrus.Fields{
			"node.id":   cn,
			"node.role": proposedRole,
		}).Debug("issued new TLS certificate")
	} else {
		// Request certificate issuance from a remote CA.
		// Last argument is nil because at this point we don't have any valid TLS creds
		tlsKeyPair, err = rootCA.RequestAndSaveNewCertificates(ctx, krw, config)
		if err != nil {
			log.G(ctx).WithError(err).Error("failed to request save new certificate")
			return nil, err
		}
	}
	// Create the Server TLS Credentials for this node. These will not be used by workers.
	serverTLSCreds, err := rootCA.NewServerTLSCredentials(tlsKeyPair)
	if err != nil {
		return nil, err
	}

	// Create a TLSConfig to be used when this node connects as a client to another remote node.
	// We're using ManagerRole as remote serverName for TLS host verification
	clientTLSCreds, err := rootCA.NewClientTLSCredentials(tlsKeyPair, ManagerRole)
	if err != nil {
		return nil, err
	}
	log.G(ctx).WithFields(logrus.Fields{
		"node.id":   clientTLSCreds.NodeID(),
		"node.role": clientTLSCreds.Role(),
	}).Debugf("new node credentials generated: %s", krw.Target())

	return NewSecurityConfig(&rootCA, krw, clientTLSCreds, serverTLSCreds), nil
}

// RenewTLSConfigNow gets a new TLS cert and key, and updates the security config if provided.  This is similar to
// RenewTLSConfig, except while that monitors for expiry, and periodically renews, this renews once and is blocking
func RenewTLSConfigNow(ctx context.Context, s *SecurityConfig, connBroker *connectionbroker.Broker) error {
	s.renewalMu.Lock()
	defer s.renewalMu.Unlock()

	ctx = log.WithModule(ctx, "tls")
	log := log.G(ctx).WithFields(logrus.Fields{
		"node.id":   s.ClientTLSCreds.NodeID(),
		"node.role": s.ClientTLSCreds.Role(),
	})

	// Let's request new certs. Renewals don't require a token.
	rootCA := s.RootCA()
	tlsKeyPair, err := rootCA.RequestAndSaveNewCertificates(ctx,
		s.KeyWriter(),
		CertificateRequestConfig{
			ConnBroker:  connBroker,
			Credentials: s.ClientTLSCreds,
		})
	if err != nil {
		log.WithError(err).Errorf("failed to renew the certificate")
		return err
	}

	clientTLSConfig, err := NewClientTLSConfig(tlsKeyPair, rootCA.Pool, CARole)
	if err != nil {
		log.WithError(err).Errorf("failed to create a new client config")
		return err
	}
	serverTLSConfig, err := NewServerTLSConfig(tlsKeyPair, rootCA.Pool)
	if err != nil {
		log.WithError(err).Errorf("failed to create a new server config")
		return err
	}

	if err = s.ClientTLSCreds.LoadNewTLSConfig(clientTLSConfig); err != nil {
		log.WithError(err).Errorf("failed to update the client credentials")
		return err
	}

	// Update the external CA to use the new client TLS
	// config using a copy without a serverName specified.
	s.externalCA.UpdateTLSConfig(&tls.Config{
		Certificates: clientTLSConfig.Certificates,
		RootCAs:      clientTLSConfig.RootCAs,
		MinVersion:   tls.VersionTLS12,
	})

	if err = s.ServerTLSCreds.LoadNewTLSConfig(serverTLSConfig); err != nil {
		log.WithError(err).Errorf("failed to update the server TLS credentials")
		return err
	}

	return nil
}

// RenewTLSConfig will continuously monitor for the necessity of renewing the local certificates, either by
// issuing them locally if key-material is available, or requesting them from a remote CA.
func RenewTLSConfig(ctx context.Context, s *SecurityConfig, connBroker *connectionbroker.Broker, renew <-chan struct{}) <-chan CertificateUpdate {
	updates := make(chan CertificateUpdate)

	go func() {
		var retry time.Duration
		expBackoff := events.NewExponentialBackoff(RenewTLSExponentialBackoff)
		defer close(updates)
		for {
			ctx = log.WithModule(ctx, "tls")
			log := log.G(ctx).WithFields(logrus.Fields{
				"node.id":   s.ClientTLSCreds.NodeID(),
				"node.role": s.ClientTLSCreds.Role(),
			})
			// Our starting default will be 5 minutes
			retry = 5 * time.Minute

			// Since the expiration of the certificate is managed remotely we should update our
			// retry timer on every iteration of this loop.
			// Retrieve the current certificate expiration information.
			validFrom, validUntil, err := readCertValidity(s.KeyReader())
			if err != nil {
				// We failed to read the expiration, let's stick with the starting default
				log.Errorf("failed to read the expiration of the TLS certificate in: %s", s.KeyReader().Target())

				select {
				case updates <- CertificateUpdate{Err: errors.New("failed to read certificate expiration")}:
				case <-ctx.Done():
					log.Info("shutting down certificate renewal routine")
					return
				}
			} else {
				// If we have an expired certificate, try to renew immediately: the hope that this is a temporary clock skew, or
				// we can issue our own TLS certs.
				if validUntil.Before(time.Now()) {
					log.Warn("the current TLS certificate is expired, so an attempt to renew it will be made immediately")
					// retry immediately(ish) with exponential backoff
					retry = expBackoff.Proceed(nil)
				} else {
					// Random retry time between 50% and 80% of the total time to expiration
					retry = calculateRandomExpiry(validFrom, validUntil)
				}
			}

			log.WithFields(logrus.Fields{
				"time": time.Now().Add(retry),
			}).Debugf("next certificate renewal scheduled for %v from now", retry)

			select {
			case <-time.After(retry):
				log.Info("renewing certificate")
			case <-renew:
				log.Info("forced certificate renewal")
			case <-ctx.Done():
				log.Info("shutting down certificate renewal routine")
				return
			}

			// ignore errors - it will just try again later
			var certUpdate CertificateUpdate
			if err := RenewTLSConfigNow(ctx, s, connBroker); err != nil {
				certUpdate.Err = err
				expBackoff.Failure(nil, nil)
			} else {
				certUpdate.Role = s.ClientTLSCreds.Role()
				expBackoff = events.NewExponentialBackoff(RenewTLSExponentialBackoff)
			}

			select {
			case updates <- certUpdate:
			case <-ctx.Done():
				log.Info("shutting down certificate renewal routine")
				return
			}
		}
	}()

	return updates
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
		randomExpiry = rand.Intn(maxValidity-minValidity) + int(minValidity)
	}

	expiry := validFrom.Add(time.Duration(randomExpiry) * time.Minute).Sub(time.Now())
	if expiry < 0 {
		return 0
	}
	return expiry
}

// NewServerTLSConfig returns a tls.Config configured for a TLS Server, given a tls.Certificate
// and the PEM-encoded root CA Certificate
func NewServerTLSConfig(cert *tls.Certificate, rootCAPool *x509.CertPool) (*tls.Config, error) {
	if rootCAPool == nil {
		return nil, errors.New("valid root CA pool required")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{*cert},
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
func NewClientTLSConfig(cert *tls.Certificate, rootCAPool *x509.CertPool, serverName string) (*tls.Config, error) {
	if rootCAPool == nil {
		return nil, errors.New("valid root CA pool required")
	}

	return &tls.Config{
		ServerName:   serverName,
		Certificates: []tls.Certificate{*cert},
		RootCAs:      rootCAPool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// NewClientTLSCredentials returns GRPC credentials for a TLS GRPC client, given a tls.Certificate
// a PEM-Encoded root CA Certificate, and the name of the remote server the client wants to connect to.
func (rootCA *RootCA) NewClientTLSCredentials(cert *tls.Certificate, serverName string) (*MutableTLSCreds, error) {
	tlsConfig, err := NewClientTLSConfig(cert, rootCA.Pool, serverName)
	if err != nil {
		return nil, err
	}

	mtls, err := NewMutableTLS(tlsConfig)

	return mtls, err
}

// NewServerTLSCredentials returns GRPC credentials for a TLS GRPC client, given a tls.Certificate
// a PEM-Encoded root CA Certificate, and the name of the remote server the client wants to connect to.
func (rootCA *RootCA) NewServerTLSCredentials(cert *tls.Certificate) (*MutableTLSCreds, error) {
	tlsConfig, err := NewServerTLSConfig(cert, rootCA.Pool)
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
