package ca

import (
	cryptorand "crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	cfconfig "github.com/cloudflare/cfssl/config"
	"github.com/docker/distribution/digest"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/picker"

	"golang.org/x/net/context"
)

const (
	rootCACertFilename  = "swarm-root-ca.crt"
	rootCAKeyFilename   = "swarm-root-ca.key"
	nodeTLSCertFilename = "swarm-node.crt"
	nodeTLSKeyFilename  = "swarm-node.key"
	nodeCSRFilename     = "swarm-node.csr"

	rootCN = "swarm-ca"
	// ManagerRole represents the Manager node type, and is used for authorization to endpoints
	ManagerRole = "swarm-manager"
	// AgentRole represents the Agent node type, and is used for authorization to endpoints
	AgentRole = "swarm-worker"
	// CARole represents the CA node type, and is used for clients attempting to get new certificates issued
	CARole = "swarm-ca"

	generatedSecretEntropyBytes = 16
	joinTokenBase               = 36
	// ceil(log(2^128-1, 36))
	maxGeneratedSecretLength = 25
	// ceil(log(2^256-1, 36))
	base36DigestLen = 50
)

// SecurityConfig is used to represent a node's security configuration. It includes information about
// the RootCA and ServerTLSCreds/ClientTLSCreds transport authenticators to be used for MTLS
type SecurityConfig struct {
	mu sync.Mutex

	rootCA     *RootCA
	externalCA *ExternalCA

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
func NewSecurityConfig(rootCA *RootCA, clientTLSCreds, serverTLSCreds *MutableTLSCreds) *SecurityConfig {
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

// DefaultPolicy is the default policy used by the signers to ensure that the only fields
// from the remote CSRs we trust are: PublicKey, PublicKeyAlgorithm and SignatureAlgorithm.
func DefaultPolicy() *cfconfig.Signing {
	return SigningPolicy(DefaultNodeCertExpiration)
}

// SigningPolicy creates a policy used by the signer to ensure that the only fields
// from the remote CSRs we trust are: PublicKey, PublicKeyAlgorithm and SignatureAlgorithm.
// It receives the duration a certificate will be valid for
func SigningPolicy(certExpiry time.Duration) *cfconfig.Signing {
	// Force the minimum Certificate expiration to be fifteen minutes
	if certExpiry < MinNodeCertExpiration {
		certExpiry = DefaultNodeCertExpiration
	}

	return &cfconfig.Signing{
		Default: &cfconfig.SigningProfile{
			Usage:  []string{"signing", "key encipherment", "server auth", "client auth"},
			Expiry: certExpiry,
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
	if len(split) != 4 || split[0] != "SWMTKN" || split[1] != "1" {
		return "", errors.New("invalid join token")
	}

	var digestInt big.Int
	digestInt.SetString(split[2], joinTokenBase)

	return digest.ParseDigest(fmt.Sprintf("sha256:%0[1]*s", 64, digestInt.Text(16)))
}

// LoadOrCreateSecurityConfig encapsulates the security logic behind joining a cluster.
// Every node requires at least a set of TLS certificates with which to join the cluster with.
// In the case of a manager, these certificates will be used both for client and server credentials.
func LoadOrCreateSecurityConfig(ctx context.Context, baseCertDir, token, proposedRole string, picker *picker.Picker, nodeInfo chan<- api.IssueNodeCertificateResponse) (*SecurityConfig, error) {
	paths := NewConfigPaths(baseCertDir)

	var (
		rootCA                         RootCA
		serverTLSCreds, clientTLSCreds *MutableTLSCreds
		err                            error
	)

	// Check if we already have a CA certificate on disk. We need a CA to have a valid SecurityConfig
	rootCA, err = GetLocalRootCA(baseCertDir)
	switch err {
	case nil:
		log.Debugf("loaded local CA certificate: %s.", paths.RootCA.Cert)
	case ErrNoLocalRootCA:
		log.Debugf("no valid local CA certificate found: %v", err)

		// Get a digest for the optional CA hash string that we've been provided
		// If we were provided a non-empty string, and it is an invalid hash, return
		// otherwise, allow the invalid digest through.
		var d digest.Digest
		if token != "" {
			d, err = getCAHashFromToken(token)
			if err != nil {
				return nil, err
			}
		}

		// Get the remote CA certificate, verify integrity with the
		// hash provided. Retry up to 5 times, in case the manager we
		// first try to contact is not responding properly (it may have
		// just been demoted, for example).

		for i := 0; i != 5; i++ {
			rootCA, err = GetRemoteCA(ctx, d, picker)
			if err == nil {
				break
			}
			log.Warningf("failed to retrieve remote root CA certificate: %v", err)
		}
		if err != nil {
			return nil, err
		}

		// Save root CA certificate to disk
		if err = saveRootCA(rootCA, paths.RootCA); err != nil {
			return nil, err
		}

		log.Debugf("downloaded remote CA certificate.")
	default:
		return nil, err
	}

	// At this point we've successfully loaded the CA details from disk, or
	// successfully downloaded them remotely. The next step is to try to
	// load our certificates.
	clientTLSCreds, serverTLSCreds, err = LoadTLSCreds(rootCA, paths.Node)
	if err != nil {
		log.Debugf("no valid local TLS credentials found: %v", err)

		var (
			tlsKeyPair *tls.Certificate
			err        error
		)

		if rootCA.CanSign() {
			// Create a new random ID for this certificate
			cn := identity.NewID()
			org := identity.NewID()

			if nodeInfo != nil {
				nodeInfo <- api.IssueNodeCertificateResponse{
					NodeID:         cn,
					NodeMembership: api.NodeMembershipAccepted,
				}
			}
			tlsKeyPair, err = rootCA.IssueAndSaveNewCertificates(paths.Node, cn, proposedRole, org)
			if err != nil {
				return nil, err
			}
		} else {
			// There was an error loading our Credentials, let's get a new certificate issued
			// Last argument is nil because at this point we don't have any valid TLS creds
			tlsKeyPair, err = rootCA.RequestAndSaveNewCertificates(ctx, paths.Node, token, picker, nil, nodeInfo)
			if err != nil {
				return nil, err
			}
		}
		// Create the Server TLS Credentials for this node. These will not be used by agents.
		serverTLSCreds, err = rootCA.NewServerTLSCredentials(tlsKeyPair)
		if err != nil {
			return nil, err
		}

		// Create a TLSConfig to be used when this node connects as a client to another remote node.
		// We're using ManagerRole as remote serverName for TLS host verification
		clientTLSCreds, err = rootCA.NewClientTLSCredentials(tlsKeyPair, ManagerRole)
		if err != nil {
			return nil, err
		}
		log.Debugf("new TLS credentials generated: %s.", paths.Node.Cert)
	} else {
		if nodeInfo != nil {
			nodeInfo <- api.IssueNodeCertificateResponse{
				NodeID:         clientTLSCreds.NodeID(),
				NodeMembership: api.NodeMembershipAccepted,
			}
		}
		log.Debugf("loaded local TLS credentials: %s.", paths.Node.Cert)
	}

	return NewSecurityConfig(&rootCA, clientTLSCreds, serverTLSCreds), nil
}

// RenewTLSConfig will continuously monitor for the necessity of renewing the local certificates, either by
// issuing them locally if key-material is available, or requesting them from a remote CA.
func RenewTLSConfig(ctx context.Context, s *SecurityConfig, baseCertDir string, picker *picker.Picker, renew <-chan struct{}) <-chan CertificateUpdate {
	paths := NewConfigPaths(baseCertDir)
	updates := make(chan CertificateUpdate)

	go func() {
		var retry time.Duration
		defer close(updates)
		for {
			// Our starting default will be 5 minutes
			retry = 5 * time.Minute

			// Since the expiration of the certificate is managed remotely we should update our
			// retry timer on every iteration of this loop.
			// Retrieve the time until the certificate expires.
			expiresIn, err := readCertExpiration(paths.Node)
			if err != nil {
				// We failed to read the expiration, let's stick with the starting default
				log.Errorf("failed to read the expiration of the TLS certificate in: %s", paths.Node.Cert)
				updates <- CertificateUpdate{Err: fmt.Errorf("failed to read certificate expiration")}
			} else {
				// If we have an expired certificate, we let's stick with the starting default in
				// the hope that this is a temporary clock skew.
				if expiresIn.Minutes() < 0 {
					log.Debugf("failed to create a new client TLS config: %v", err)
					updates <- CertificateUpdate{Err: fmt.Errorf("TLS Certificate is expired")}
				} else {
					// Random retry time between 50% and 80% of the total time to expiration
					retry = calculateRandomExpiry(expiresIn)
				}
			}

			select {
			case <-time.After(retry):
			case <-renew:
			case <-ctx.Done():
				return
			}
			log.Infof("Renewing TLS Certificate.")

			// Let's request new certs. Renewals don't require a token.
			rootCA := s.RootCA()
			tlsKeyPair, err := rootCA.RequestAndSaveNewCertificates(ctx,
				paths.Node,
				"",
				picker,
				s.ClientTLSCreds,
				nil)
			if err != nil {
				log.Debugf("failed to renew the TLS Certificate: %v", err)
				updates <- CertificateUpdate{Err: err}
				continue
			}

			clientTLSConfig, err := NewClientTLSConfig(tlsKeyPair, rootCA.Pool, CARole)
			if err != nil {
				log.Debugf("failed to create a new client TLS config: %v", err)
				updates <- CertificateUpdate{Err: err}
			}
			serverTLSConfig, err := NewServerTLSConfig(tlsKeyPair, rootCA.Pool)
			if err != nil {
				log.Debugf("failed to create a new server TLS config: %v", err)
				updates <- CertificateUpdate{Err: err}
			}

			err = s.ClientTLSCreds.LoadNewTLSConfig(clientTLSConfig)
			if err != nil {
				log.Debugf("failed to update the client TLS credentials: %v", err)
				updates <- CertificateUpdate{Err: err}
			}

			// Update the external CA to use the new client TLS
			// config using a copy without a serverName specified.
			s.externalCA.UpdateTLSConfig(&tls.Config{
				Certificates: clientTLSConfig.Certificates,
				RootCAs:      clientTLSConfig.RootCAs,
				MinVersion:   tls.VersionTLS12,
			})

			err = s.ServerTLSCreds.LoadNewTLSConfig(serverTLSConfig)
			if err != nil {
				log.Debugf("failed to update the server TLS credentials: %v", err)
				updates <- CertificateUpdate{Err: err}
			}

			updates <- CertificateUpdate{Role: s.ClientTLSCreds.Role()}
		}
	}()

	return updates
}

// calculateRandomExpiry returns a random duration between 50% and 80% of the original
// duration
func calculateRandomExpiry(expiresIn time.Duration) time.Duration {
	if expiresIn.Minutes() < 1 {
		return time.Second
	}

	var randomExpiry int
	// Our lower bound of renewal will be half of the total expiration time
	minValidity := int(expiresIn.Minutes() * CertLowerRotationRange)
	// Our upper bound of renewal will be 80% of the total expiration time
	maxValidity := int(expiresIn.Minutes() * CertUpperRotationRange)
	// Let's select a random number of minutes between min and max, and set our retry for that
	// Using randomly selected rotation allows us to avoid certificate thundering herds.
	if maxValidity-minValidity < 1 {
		randomExpiry = minValidity
	} else {
		randomExpiry = rand.Intn(maxValidity-minValidity) + int(minValidity)
	}

	return time.Duration(randomExpiry) * time.Minute
}

// LoadTLSCreds loads tls credentials from the specified path and verifies that
// thay are valid for the RootCA.
func LoadTLSCreds(rootCA RootCA, paths CertPaths) (*MutableTLSCreds, *MutableTLSCreds, error) {
	// Read both the Cert and Key from disk
	cert, err := ioutil.ReadFile(paths.Cert)
	if err != nil {
		return nil, nil, err
	}
	key, err := ioutil.ReadFile(paths.Key)
	if err != nil {
		return nil, nil, err
	}

	// Create an x509 certificate out of the contents on disk
	certBlock, _ := pem.Decode([]byte(cert))
	if certBlock == nil {
		return nil, nil, fmt.Errorf("failed to parse certificate PEM")
	}

	// Create an X509Cert so we can .Verify()
	X509Cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	// Include our root pool
	opts := x509.VerifyOptions{
		Roots: rootCA.Pool,
	}

	// Check to see if this certificate was signed by our CA, and isn't expired
	if _, err := X509Cert.Verify(opts); err != nil {
		return nil, nil, err
	}

	// Now that we know this certificate is valid, create a TLS Certificate for our
	// credentials
	var (
		keyPair tls.Certificate
		newErr  error
	)
	keyPair, err = tls.X509KeyPair(cert, key)
	if err != nil {
		// This current keypair isn't valid. It's possible we crashed before we
		// overwrote the current key. Let's try loading it from disk.
		tempPaths := genTempPaths(paths)
		key, newErr = ioutil.ReadFile(tempPaths.Key)
		if newErr != nil {
			return nil, nil, err
		}

		keyPair, newErr = tls.X509KeyPair(cert, key)
		if newErr != nil {
			return nil, nil, err
		}
	}

	// Load the Certificates as server credentials
	serverTLSCreds, err := rootCA.NewServerTLSCredentials(&keyPair)
	if err != nil {
		return nil, nil, err
	}

	// Load the Certificates also as client credentials.
	// Both Agents and Managers always connect to remote Managers,
	// so ServerName is always set to ManagerRole here.
	clientTLSCreds, err := rootCA.NewClientTLSCredentials(&keyPair, ManagerRole)
	if err != nil {
		return nil, nil, err
	}

	return clientTLSCreds, serverTLSCreds, nil
}

func genTempPaths(path CertPaths) CertPaths {
	return CertPaths{
		Key:  filepath.Join(filepath.Dir(path.Key), "."+filepath.Base(path.Key)),
		Cert: filepath.Join(filepath.Dir(path.Cert), "."+filepath.Base(path.Cert)),
	}
}

// NewServerTLSConfig returns a tls.Config configured for a TLS Server, given a tls.Certificate
// and the PEM-encoded root CA Certificate
func NewServerTLSConfig(cert *tls.Certificate, rootCAPool *x509.CertPool) (*tls.Config, error) {
	if rootCAPool == nil {
		return nil, fmt.Errorf("valid root CA pool required")
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
		return nil, fmt.Errorf("valid root CA pool required")
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
func (rca *RootCA) NewClientTLSCredentials(cert *tls.Certificate, serverName string) (*MutableTLSCreds, error) {
	tlsConfig, err := NewClientTLSConfig(cert, rca.Pool, serverName)
	if err != nil {
		return nil, err
	}

	mtls, err := NewMutableTLS(tlsConfig)

	return mtls, err
}

// NewServerTLSCredentials returns GRPC credentials for a TLS GRPC client, given a tls.Certificate
// a PEM-Encoded root CA Certificate, and the name of the remote server the client wants to connect to.
func (rca *RootCA) NewServerTLSCredentials(cert *tls.Certificate) (*MutableTLSCreds, error) {
	tlsConfig, err := NewServerTLSConfig(cert, rca.Pool)
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
		return AgentRole, nil
	default:
		return "", fmt.Errorf("failed to parse api role: %v", apiRole)
	}
}

// FormatRole parses an internal role string into an apiRole
func FormatRole(role string) (api.NodeRole, error) {
	switch strings.ToLower(role) {
	case strings.ToLower(ManagerRole):
		return api.NodeRoleManager, nil
	case strings.ToLower(AgentRole):
		return api.NodeRoleWorker, nil
	default:
		return 0, fmt.Errorf("failed to parse role: %s", role)
	}
}
