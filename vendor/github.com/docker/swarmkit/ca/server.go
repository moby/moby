package ca

import (
	"bytes"
	"crypto/subtle"
	"crypto/x509"
	"sync"
	"time"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/api/equality"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/state/store"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	defaultReconciliationRetryInterval = 10 * time.Second
	defaultRootReconciliationInterval  = 3 * time.Second
)

// Server is the CA and NodeCA API gRPC server.
// TODO(aaronl): At some point we may want to have separate implementations of
// CA, NodeCA, and other hypothetical future CA services. At the moment,
// breaking it apart doesn't seem worth it.
type Server struct {
	mu                          sync.Mutex
	wg                          sync.WaitGroup
	ctx                         context.Context
	cancel                      func()
	store                       *store.MemoryStore
	securityConfig              *SecurityConfig
	clusterID                   string
	localRootCA                 *RootCA
	externalCA                  *ExternalCA
	externalCAPool              *x509.CertPool
	joinTokens                  *api.JoinTokens
	reconciliationRetryInterval time.Duration

	// pending is a map of nodes with pending certificates issuance or
	// renewal. They are indexed by node ID.
	pending map[string]*api.Node

	// started is a channel which gets closed once the server is running
	// and able to service RPCs.
	started chan struct{}

	// these are cached values to ensure we only update the security config when
	// the cluster root CA and external CAs have changed - the cluster object
	// can change for other reasons, and it would not be necessary to update
	// the security config as a result
	lastSeenClusterRootCA *api.RootCA
	lastSeenExternalCAs   []*api.ExternalCA

	// This mutex protects the components of the CA server used to issue new certificates
	// (and any attributes used to update those components): `lastSeenClusterRootCA` and
	// `lastSeenExternalCA`, which are used to update `externalCA` and the `rootCA` object
	// of the SecurityConfig
	signingMu sync.Mutex

	// lets us monitor and finish root rotations
	rootReconciler                  *rootRotationReconciler
	rootReconciliationRetryInterval time.Duration
}

// DefaultCAConfig returns the default CA Config, with a default expiration.
func DefaultCAConfig() api.CAConfig {
	return api.CAConfig{
		NodeCertExpiry: gogotypes.DurationProto(DefaultNodeCertExpiration),
	}
}

// NewServer creates a CA API server.
func NewServer(store *store.MemoryStore, securityConfig *SecurityConfig) *Server {
	return &Server{
		store:                           store,
		securityConfig:                  securityConfig,
		localRootCA:                     securityConfig.RootCA(),
		externalCA:                      NewExternalCA(nil, nil),
		pending:                         make(map[string]*api.Node),
		started:                         make(chan struct{}),
		reconciliationRetryInterval:     defaultReconciliationRetryInterval,
		rootReconciliationRetryInterval: defaultRootReconciliationInterval,
		clusterID:                       securityConfig.ClientTLSCreds.Organization(),
	}
}

// ExternalCA returns the current external CA - this is exposed to support unit testing only, and the external CA
// should really be a private field
func (s *Server) ExternalCA() *ExternalCA {
	s.signingMu.Lock()
	defer s.signingMu.Unlock()
	return s.externalCA
}

// RootCA returns the current local root CA - this is exposed to support unit testing only, and the root CA
// should really be a private field
func (s *Server) RootCA() *RootCA {
	s.signingMu.Lock()
	defer s.signingMu.Unlock()
	return s.localRootCA
}

// SetReconciliationRetryInterval changes the time interval between
// reconciliation attempts. This function must be called before Run.
func (s *Server) SetReconciliationRetryInterval(reconciliationRetryInterval time.Duration) {
	s.reconciliationRetryInterval = reconciliationRetryInterval
}

// SetRootReconciliationInterval changes the time interval between root rotation
// reconciliation attempts.  This function must be called before Run.
func (s *Server) SetRootReconciliationInterval(interval time.Duration) {
	s.rootReconciliationRetryInterval = interval
}

// GetUnlockKey is responsible for returning the current unlock key used for encrypting TLS private keys and
// other at rest data.  Access to this RPC call should only be allowed via mutual TLS from managers.
func (s *Server) GetUnlockKey(ctx context.Context, request *api.GetUnlockKeyRequest) (*api.GetUnlockKeyResponse, error) {
	// This directly queries the store, rather than storing the unlock key and version on
	// the `Server` object and updating it `updateCluster` is called, because we need this
	// API to return the latest version of the key.  Otherwise, there might be a slight delay
	// between when the cluster gets updated, and when this function returns the latest key.
	// This delay is currently unacceptable because this RPC call is the only way, after a
	// cluster update, to get the actual value of the unlock key, and we don't want to return
	// a cached value.
	resp := api.GetUnlockKeyResponse{}
	s.store.View(func(tx store.ReadTx) {
		cluster := store.GetCluster(tx, s.clusterID)
		resp.Version = cluster.Meta.Version
		if cluster.Spec.EncryptionConfig.AutoLockManagers {
			for _, encryptionKey := range cluster.UnlockKeys {
				if encryptionKey.Subsystem == ManagerRole {
					resp.UnlockKey = encryptionKey.Key
					return
				}
			}
		}
	})

	return &resp, nil
}

// NodeCertificateStatus returns the current issuance status of an issuance request identified by the nodeID
func (s *Server) NodeCertificateStatus(ctx context.Context, request *api.NodeCertificateStatusRequest) (*api.NodeCertificateStatusResponse, error) {
	if request.NodeID == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, codes.InvalidArgument.String())
	}

	serverCtx, err := s.isRunningLocked()
	if err != nil {
		return nil, err
	}

	var node *api.Node

	event := api.EventUpdateNode{
		Node:   &api.Node{ID: request.NodeID},
		Checks: []api.NodeCheckFunc{api.NodeCheckID},
	}

	// Retrieve the current value of the certificate with this token, and create a watcher
	updates, cancel, err := store.ViewAndWatch(
		s.store,
		func(tx store.ReadTx) error {
			node = store.GetNode(tx, request.NodeID)
			return nil
		},
		event,
	)
	if err != nil {
		return nil, err
	}
	defer cancel()

	// This node ID doesn't exist
	if node == nil {
		return nil, grpc.Errorf(codes.NotFound, codes.NotFound.String())
	}

	log.G(ctx).WithFields(logrus.Fields{
		"node.id": node.ID,
		"status":  node.Certificate.Status,
		"method":  "NodeCertificateStatus",
	})

	// If this certificate has a final state, return it immediately (both pending and renew are transition states)
	if isFinalState(node.Certificate.Status) {
		return &api.NodeCertificateStatusResponse{
			Status:      &node.Certificate.Status,
			Certificate: &node.Certificate,
		}, nil
	}

	log.G(ctx).WithFields(logrus.Fields{
		"node.id": node.ID,
		"status":  node.Certificate.Status,
		"method":  "NodeCertificateStatus",
	}).Debugf("started watching for certificate updates")

	// Certificate is Pending or in an Unknown state, let's wait for changes.
	for {
		select {
		case event := <-updates:
			switch v := event.(type) {
			case api.EventUpdateNode:
				// We got an update on the certificate record. If the status is a final state,
				// return the certificate.
				if isFinalState(v.Node.Certificate.Status) {
					cert := v.Node.Certificate.Copy()
					return &api.NodeCertificateStatusResponse{
						Status:      &cert.Status,
						Certificate: cert,
					}, nil
				}
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-serverCtx.Done():
			return nil, s.ctx.Err()
		}
	}
}

// IssueNodeCertificate is responsible for gatekeeping both certificate requests from new nodes in the swarm,
// and authorizing certificate renewals.
// If a node presented a valid certificate, the corresponding certificate is set in a RENEW state.
// If a node failed to present a valid certificate, we check for a valid join token and set the
// role accordingly. A new random node ID is generated, and the corresponding node entry is created.
// IssueNodeCertificate is the only place where new node entries to raft should be created.
func (s *Server) IssueNodeCertificate(ctx context.Context, request *api.IssueNodeCertificateRequest) (*api.IssueNodeCertificateResponse, error) {
	// First, let's see if the remote node is presenting a non-empty CSR
	if len(request.CSR) == 0 {
		return nil, grpc.Errorf(codes.InvalidArgument, codes.InvalidArgument.String())
	}

	if err := s.isReadyLocked(); err != nil {
		return nil, err
	}

	var (
		blacklistedCerts map[string]*api.BlacklistedCertificate
		clusters         []*api.Cluster
		err              error
	)

	s.store.View(func(readTx store.ReadTx) {
		clusters, err = store.FindClusters(readTx, store.ByName(store.DefaultClusterName))
	})

	// Not having a cluster object yet means we can't check
	// the blacklist.
	if err == nil && len(clusters) == 1 {
		blacklistedCerts = clusters[0].BlacklistedCertificates
	}

	// Renewing the cert with a local (unix socket) is always valid.
	localNodeInfo := ctx.Value(LocalRequestKey)
	if localNodeInfo != nil {
		nodeInfo, ok := localNodeInfo.(RemoteNodeInfo)
		if ok && nodeInfo.NodeID != "" {
			return s.issueRenewCertificate(ctx, nodeInfo.NodeID, request.CSR)
		}
	}

	// If the remote node is a worker (either forwarded by a manager, or calling directly),
	// issue a renew worker certificate entry with the correct ID
	nodeID, err := AuthorizeForwardedRoleAndOrg(ctx, []string{WorkerRole}, []string{ManagerRole}, s.clusterID, blacklistedCerts)
	if err == nil {
		return s.issueRenewCertificate(ctx, nodeID, request.CSR)
	}

	// If the remote node is a manager (either forwarded by another manager, or calling directly),
	// issue a renew certificate entry with the correct ID
	nodeID, err = AuthorizeForwardedRoleAndOrg(ctx, []string{ManagerRole}, []string{ManagerRole}, s.clusterID, blacklistedCerts)
	if err == nil {
		return s.issueRenewCertificate(ctx, nodeID, request.CSR)
	}

	// The remote node didn't successfully present a valid MTLS certificate, let's issue a
	// certificate with a new random ID
	role := api.NodeRole(-1)

	s.mu.Lock()
	if subtle.ConstantTimeCompare([]byte(s.joinTokens.Manager), []byte(request.Token)) == 1 {
		role = api.NodeRoleManager
	} else if subtle.ConstantTimeCompare([]byte(s.joinTokens.Worker), []byte(request.Token)) == 1 {
		role = api.NodeRoleWorker
	}
	s.mu.Unlock()

	if role < 0 {
		return nil, grpc.Errorf(codes.InvalidArgument, "A valid join token is necessary to join this cluster")
	}

	// Max number of collisions of ID or CN to tolerate before giving up
	maxRetries := 3
	// Generate a random ID for this new node
	for i := 0; ; i++ {
		nodeID = identity.NewID()

		// Create a new node
		err := s.store.Update(func(tx store.Tx) error {
			node := &api.Node{
				Role: role,
				ID:   nodeID,
				Certificate: api.Certificate{
					CSR:  request.CSR,
					CN:   nodeID,
					Role: role,
					Status: api.IssuanceStatus{
						State: api.IssuanceStatePending,
					},
				},
				Spec: api.NodeSpec{
					DesiredRole:  role,
					Membership:   api.NodeMembershipAccepted,
					Availability: request.Availability,
				},
			}

			return store.CreateNode(tx, node)
		})
		if err == nil {
			log.G(ctx).WithFields(logrus.Fields{
				"node.id":   nodeID,
				"node.role": role,
				"method":    "IssueNodeCertificate",
			}).Debugf("new certificate entry added")
			break
		}
		if err != store.ErrExist {
			return nil, err
		}
		if i == maxRetries {
			return nil, err
		}
		log.G(ctx).WithFields(logrus.Fields{
			"node.id":   nodeID,
			"node.role": role,
			"method":    "IssueNodeCertificate",
		}).Errorf("randomly generated node ID collided with an existing one - retrying")
	}

	return &api.IssueNodeCertificateResponse{
		NodeID:         nodeID,
		NodeMembership: api.NodeMembershipAccepted,
	}, nil
}

// issueRenewCertificate receives a nodeID and a CSR and modifies the node's certificate entry with the new CSR
// and changes the state to RENEW, so it can be picked up and signed by the signing reconciliation loop
func (s *Server) issueRenewCertificate(ctx context.Context, nodeID string, csr []byte) (*api.IssueNodeCertificateResponse, error) {
	var (
		cert api.Certificate
		node *api.Node
	)
	err := s.store.Update(func(tx store.Tx) error {
		// Attempt to retrieve the node with nodeID
		node = store.GetNode(tx, nodeID)
		if node == nil {
			log.G(ctx).WithFields(logrus.Fields{
				"node.id": nodeID,
				"method":  "issueRenewCertificate",
			}).Warnf("node does not exist")
			// If this node doesn't exist, we shouldn't be renewing a certificate for it
			return grpc.Errorf(codes.NotFound, "node %s not found when attempting to renew certificate", nodeID)
		}

		// Create a new Certificate entry for this node with the new CSR and a RENEW state
		cert = api.Certificate{
			CSR:  csr,
			CN:   node.ID,
			Role: node.Role,
			Status: api.IssuanceStatus{
				State: api.IssuanceStateRenew,
			},
		}

		node.Certificate = cert
		return store.UpdateNode(tx, node)
	})
	if err != nil {
		return nil, err
	}

	log.G(ctx).WithFields(logrus.Fields{
		"cert.cn":   cert.CN,
		"cert.role": cert.Role,
		"method":    "issueRenewCertificate",
	}).Debugf("node certificate updated")

	return &api.IssueNodeCertificateResponse{
		NodeID:         nodeID,
		NodeMembership: node.Spec.Membership,
	}, nil
}

// GetRootCACertificate returns the certificate of the Root CA. It is used as a convenience for distributing
// the root of trust for the swarm. Clients should be using the CA hash to verify if they weren't target to
// a MiTM. If they fail to do so, node bootstrap works with TOFU semantics.
func (s *Server) GetRootCACertificate(ctx context.Context, request *api.GetRootCACertificateRequest) (*api.GetRootCACertificateResponse, error) {
	log.G(ctx).WithFields(logrus.Fields{
		"method": "GetRootCACertificate",
	})

	s.signingMu.Lock()
	defer s.signingMu.Unlock()

	return &api.GetRootCACertificateResponse{
		Certificate: s.localRootCA.Certs,
	}, nil
}

// Run runs the CA signer main loop.
// The CA signer can be stopped with cancelling ctx or calling Stop().
func (s *Server) Run(ctx context.Context) error {
	s.mu.Lock()
	if s.isRunning() {
		s.mu.Unlock()
		return errors.New("CA signer is already running")
	}
	s.wg.Add(1)
	s.ctx, s.cancel = context.WithCancel(log.WithModule(ctx, "ca"))
	ctx = s.ctx
	s.mu.Unlock()
	defer s.wg.Done()
	defer func() {
		s.mu.Lock()
		s.mu.Unlock()
	}()

	// Retrieve the channels to keep track of changes in the cluster
	// Retrieve all the currently registered nodes
	var (
		nodes   []*api.Node
		cluster *api.Cluster
		err     error
	)
	updates, cancel, err := store.ViewAndWatch(
		s.store,
		func(readTx store.ReadTx) error {
			cluster = store.GetCluster(readTx, s.clusterID)
			if cluster == nil {
				return errors.New("could not find cluster object")
			}
			nodes, err = store.FindNodes(readTx, store.All)
			return err
		},
		api.EventCreateNode{},
		api.EventUpdateNode{},
		api.EventDeleteNode{},
		api.EventUpdateCluster{
			Cluster: &api.Cluster{ID: s.clusterID},
			Checks:  []api.ClusterCheckFunc{api.ClusterCheckID},
		},
	)

	// call once to ensure that the join tokens and local/external CA signer are always set
	rootReconciler := &rootRotationReconciler{
		ctx:                 log.WithField(ctx, "method", "(*Server).rootRotationReconciler"),
		clusterID:           s.clusterID,
		store:               s.store,
		batchUpdateInterval: s.rootReconciliationRetryInterval,
	}

	s.UpdateRootCA(ctx, cluster, rootReconciler)

	// Do this after updateCluster has been called, so Ready() and isRunning never returns true without
	// the join tokens and external CA/security config's root CA being set correctly
	s.mu.Lock()
	close(s.started)
	s.mu.Unlock()

	if err != nil {
		log.G(ctx).WithFields(logrus.Fields{
			"method": "(*Server).Run",
		}).WithError(err).Errorf("snapshot store view failed")
		return err
	}
	defer cancel()

	// We might have missed some updates if there was a leader election,
	// so let's pick up the slack.
	if err := s.reconcileNodeCertificates(ctx, nodes); err != nil {
		// We don't return here because that means the Run loop would
		// never run. Log an error instead.
		log.G(ctx).WithFields(logrus.Fields{
			"method": "(*Server).Run",
		}).WithError(err).Errorf("error attempting to reconcile certificates")
	}

	ticker := time.NewTicker(s.reconciliationRetryInterval)
	defer ticker.Stop()

	externalTLSCredsChange, externalTLSWatchCancel := s.securityConfig.Watch()
	defer externalTLSWatchCancel()

	// Watch for new nodes being created, new nodes being updated, and changes
	// to the cluster
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		select {
		case event := <-updates:
			switch v := event.(type) {
			case api.EventCreateNode:
				s.evaluateAndSignNodeCert(ctx, v.Node)
				rootReconciler.UpdateNode(v.Node)
			case api.EventUpdateNode:
				// If this certificate is already at a final state
				// no need to evaluate and sign it.
				if !isFinalState(v.Node.Certificate.Status) {
					s.evaluateAndSignNodeCert(ctx, v.Node)
				}
				rootReconciler.UpdateNode(v.Node)
			case api.EventDeleteNode:
				rootReconciler.DeleteNode(v.Node)
			case api.EventUpdateCluster:
				if v.Cluster.ID == s.clusterID {
					s.UpdateRootCA(ctx, v.Cluster, rootReconciler)
				}
			}
		case <-externalTLSCredsChange:
			// The TLS certificates can rotate independently of the root CA (and hence which roots the
			// external CA trusts) and external CA URLs.  It's possible that the root CA update is received
			// before the external TLS cred change notification.  During that period, it is possible that
			// the TLS creds will expire or otherwise fail to authorize against external CAs.  However, in
			// that case signing will just fail with a recoverable connectivity error - the state of the
			// certificate issuance is left as pending, and on the next tick, the server will try to sign
			// all nodes with pending certs again (by which time the TLS cred change will have been
			// received).

			// Note that if the external CA changes, the new external CA *MUST* trust the current server's
			// certificate issuer, and this server's certificates should not be extremely close to expiry,
			// otherwise this server would not be able to get new TLS certificates and will no longer be
			// able to function.
			s.signingMu.Lock()
			s.externalCA.UpdateTLSConfig(NewExternalCATLSConfig(
				s.securityConfig.ClientTLSCreds.Config().Certificates, s.externalCAPool))
			s.signingMu.Unlock()
		case <-ticker.C:
			for _, node := range s.pending {
				if err := s.evaluateAndSignNodeCert(ctx, node); err != nil {
					// If this sign operation did not succeed, the rest are
					// unlikely to. Yield so that we don't hammer an external CA.
					// Since the map iteration order is randomized, there is no
					// risk of getting stuck on a problematic CSR.
					break
				}
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// Stop stops the CA and closes all grpc streams.
func (s *Server) Stop() error {
	s.mu.Lock()

	if !s.isRunning() {
		s.mu.Unlock()
		return errors.New("CA signer is already stopped")
	}
	s.cancel()
	s.started = make(chan struct{})
	s.joinTokens = nil
	s.mu.Unlock()

	// Wait for Run to complete
	s.wg.Wait()

	return nil
}

// Ready waits on the ready channel and returns when the server is ready to serve.
func (s *Server) Ready() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started
}

func (s *Server) isRunningLocked() (context.Context, error) {
	s.mu.Lock()
	if !s.isRunning() {
		s.mu.Unlock()
		return nil, grpc.Errorf(codes.Aborted, "CA signer is stopped")
	}
	ctx := s.ctx
	s.mu.Unlock()
	return ctx, nil
}

func (s *Server) isReadyLocked() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isRunning() {
		return grpc.Errorf(codes.Aborted, "CA signer is stopped")
	}
	if s.joinTokens == nil {
		return grpc.Errorf(codes.Aborted, "CA signer is still starting")
	}
	return nil
}

func (s *Server) isRunning() bool {
	if s.ctx == nil {
		return false
	}
	select {
	case <-s.ctx.Done():
		return false
	default:
	}
	return true
}

// filterExternalCAURLS returns a list of external CA urls filtered by the desired cert.
func filterExternalCAURLS(ctx context.Context, desiredCert, defaultCert []byte, apiExternalCAs []*api.ExternalCA) (urls []string) {
	desiredCert = NormalizePEMs(desiredCert)

	// TODO(aaronl): In the future, this will be abstracted with an ExternalCA interface that has different
	// implementations for different CA types. At the moment, only CFSSL is supported.
	for i, extCA := range apiExternalCAs {
		// We want to support old external CA specifications which did not have a CA cert.  If there is no cert specified,
		// we assume it's the old cert
		certForExtCA := extCA.CACert
		if len(certForExtCA) == 0 {
			certForExtCA = defaultCert
		}
		certForExtCA = NormalizePEMs(certForExtCA)
		if extCA.Protocol != api.ExternalCA_CAProtocolCFSSL {
			log.G(ctx).Debugf("skipping external CA %d (url: %s) due to unknown protocol type", i, extCA.URL)
			continue
		}
		if !bytes.Equal(certForExtCA, desiredCert) {
			log.G(ctx).Debugf("skipping external CA %d (url: %s) because it has the wrong CA cert", i, extCA.URL)
			continue
		}
		urls = append(urls, extCA.URL)
	}
	return
}

// UpdateRootCA is called when there are cluster changes, and it ensures that the local RootCA is
// always aware of changes in clusterExpiry and the Root CA key material - this can be called by
// anything to update the root CA material
func (s *Server) UpdateRootCA(ctx context.Context, cluster *api.Cluster, reconciler *rootRotationReconciler) error {
	s.mu.Lock()
	s.joinTokens = cluster.RootCA.JoinTokens.Copy()
	s.mu.Unlock()
	rCA := cluster.RootCA.Copy()
	if reconciler != nil {
		reconciler.UpdateRootCA(rCA)
	}

	s.signingMu.Lock()
	defer s.signingMu.Unlock()
	firstSeenCluster := s.lastSeenClusterRootCA == nil && s.lastSeenExternalCAs == nil
	rootCAChanged := len(rCA.CACert) != 0 && !equality.RootCAEqualStable(s.lastSeenClusterRootCA, rCA)
	externalCAChanged := !equality.ExternalCAsEqualStable(s.lastSeenExternalCAs, cluster.Spec.CAConfig.ExternalCAs)
	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(logrus.Fields{
		"cluster.id": cluster.ID,
		"method":     "(*Server).UpdateRootCA",
	}))

	if rootCAChanged {
		setOrUpdate := "set"
		if !firstSeenCluster {
			log.G(ctx).Debug("Updating signing root CA and external CA due to change in cluster Root CA")
			setOrUpdate = "updated"
		}
		expiry := DefaultNodeCertExpiration
		if cluster.Spec.CAConfig.NodeCertExpiry != nil {
			// NodeCertExpiry exists, let's try to parse the duration out of it
			clusterExpiry, err := gogotypes.DurationFromProto(cluster.Spec.CAConfig.NodeCertExpiry)
			if err != nil {
				log.G(ctx).WithError(err).Warn("failed to parse certificate expiration, using default")
			} else {
				// We were able to successfully parse the expiration out of the cluster.
				expiry = clusterExpiry
			}
		} else {
			// NodeCertExpiry seems to be nil
			log.G(ctx).Warn("no certificate expiration specified, using default")
		}
		// Attempt to update our local RootCA with the new parameters
		updatedRootCA, err := RootCAFromAPI(ctx, rCA, expiry)
		if err != nil {
			return errors.Wrap(err, "invalid Root CA object in cluster")
		}

		s.localRootCA = &updatedRootCA
		s.externalCAPool = updatedRootCA.Pool
		externalCACert := rCA.CACert
		if rCA.RootRotation != nil {
			externalCACert = rCA.RootRotation.CACert
			// the external CA has to trust the new CA cert
			s.externalCAPool = x509.NewCertPool()
			s.externalCAPool.AppendCertsFromPEM(rCA.CACert)
			s.externalCAPool.AppendCertsFromPEM(rCA.RootRotation.CACert)
		}
		s.lastSeenExternalCAs = cluster.Spec.CAConfig.Copy().ExternalCAs
		urls := filterExternalCAURLS(ctx, externalCACert, rCA.CACert, s.lastSeenExternalCAs)
		// Replace the external CA with the relevant intermediates, URLS, and TLS config
		s.externalCA = NewExternalCA(updatedRootCA.Intermediates,
			NewExternalCATLSConfig(s.securityConfig.ClientTLSCreds.Config().Certificates, s.externalCAPool), urls...)

		// only update the server cache if we've successfully updated the root CA
		log.G(ctx).Debugf("Root CA %s successfully", setOrUpdate)
		s.lastSeenClusterRootCA = rCA
	} else if externalCAChanged {
		// we want to update only if the external CA URLS have changed, since if the root CA has changed we already
		// run similar logic
		if !firstSeenCluster {
			log.G(ctx).Debug("Updating security config external CA URLs due to change in cluster spec's list of external CAs")
		}
		wantedExternalCACert := rCA.CACert // we want to only add external CA URLs that use this cert
		if rCA.RootRotation != nil {
			// we're rotating to a new root, so we only want external CAs with the new root cert
			wantedExternalCACert = rCA.RootRotation.CACert
		}
		// Update our external CA with the list of External CA URLs from the new cluster state
		s.lastSeenExternalCAs = cluster.Spec.CAConfig.Copy().ExternalCAs
		urls := filterExternalCAURLS(ctx, wantedExternalCACert, rCA.CACert, s.lastSeenExternalCAs)
		s.externalCA.UpdateURLs(urls...)
	}
	return nil
}

// evaluateAndSignNodeCert implements the logic of which certificates to sign
func (s *Server) evaluateAndSignNodeCert(ctx context.Context, node *api.Node) error {
	// If the desired membership and actual state are in sync, there's
	// nothing to do.
	certState := node.Certificate.Status.State
	if node.Spec.Membership == api.NodeMembershipAccepted &&
		(certState == api.IssuanceStateIssued || certState == api.IssuanceStateRotate) {
		return nil
	}

	// If the certificate state is renew, then it is a server-sided accepted cert (cert renewals)
	if certState == api.IssuanceStateRenew {
		return s.signNodeCert(ctx, node)
	}

	// Sign this certificate if a user explicitly changed it to Accepted, and
	// the certificate is in pending state
	if node.Spec.Membership == api.NodeMembershipAccepted && certState == api.IssuanceStatePending {
		return s.signNodeCert(ctx, node)
	}

	return nil
}

// signNodeCert does the bulk of the work for signing a certificate
func (s *Server) signNodeCert(ctx context.Context, node *api.Node) error {
	s.signingMu.Lock()
	rootCA := s.localRootCA
	externalCA := s.externalCA
	s.signingMu.Unlock()

	node = node.Copy()
	nodeID := node.ID
	// Convert the role from proto format
	role, err := ParseRole(node.Certificate.Role)
	if err != nil {
		log.G(ctx).WithFields(logrus.Fields{
			"node.id": node.ID,
			"method":  "(*Server).signNodeCert",
		}).WithError(err).Errorf("failed to parse role")
		return errors.New("failed to parse role")
	}

	s.pending[node.ID] = node

	// Attempt to sign the CSR
	var (
		rawCSR = node.Certificate.CSR
		cn     = node.Certificate.CN
		ou     = role
		org    = s.clusterID
	)

	// Try using the external CA first.
	cert, err := externalCA.Sign(ctx, PrepareCSR(rawCSR, cn, ou, org))
	if err == ErrNoExternalCAURLs {
		// No external CA servers configured. Try using the local CA.
		cert, err = rootCA.ParseValidateAndSignCSR(rawCSR, cn, ou, org)
	}

	if err != nil {
		log.G(ctx).WithFields(logrus.Fields{
			"node.id": node.ID,
			"method":  "(*Server).signNodeCert",
		}).WithError(err).Errorf("failed to sign CSR")

		// If the current state is already Failed, no need to change it
		if node.Certificate.Status.State == api.IssuanceStateFailed {
			delete(s.pending, node.ID)
			return errors.New("failed to sign CSR")
		}

		if _, ok := err.(recoverableErr); ok {
			// Return without changing the state of the certificate. We may
			// retry signing it in the future.
			return errors.New("failed to sign CSR")
		}

		// We failed to sign this CSR, change the state to FAILED
		err = s.store.Update(func(tx store.Tx) error {
			node := store.GetNode(tx, nodeID)
			if node == nil {
				return errors.Errorf("node %s not found", nodeID)
			}

			node.Certificate.Status = api.IssuanceStatus{
				State: api.IssuanceStateFailed,
				Err:   err.Error(),
			}

			return store.UpdateNode(tx, node)
		})
		if err != nil {
			log.G(ctx).WithFields(logrus.Fields{
				"node.id": nodeID,
				"method":  "(*Server).signNodeCert",
			}).WithError(err).Errorf("transaction failed when setting state to FAILED")
		}

		delete(s.pending, node.ID)
		return errors.New("failed to sign CSR")
	}

	// We were able to successfully sign the new CSR. Let's try to update the nodeStore
	for {
		err = s.store.Update(func(tx store.Tx) error {
			node.Certificate.Certificate = cert
			node.Certificate.Status = api.IssuanceStatus{
				State: api.IssuanceStateIssued,
			}

			err := store.UpdateNode(tx, node)
			if err != nil {
				node = store.GetNode(tx, nodeID)
				if node == nil {
					err = errors.Errorf("node %s does not exist", nodeID)
				}
			}
			return err
		})
		if err == nil {
			log.G(ctx).WithFields(logrus.Fields{
				"node.id":   node.ID,
				"node.role": node.Certificate.Role,
				"method":    "(*Server).signNodeCert",
			}).Debugf("certificate issued")
			delete(s.pending, node.ID)
			break
		}
		if err == store.ErrSequenceConflict {
			continue
		}

		log.G(ctx).WithFields(logrus.Fields{
			"node.id": nodeID,
			"method":  "(*Server).signNodeCert",
		}).WithError(err).Errorf("transaction failed")
		return errors.New("transaction failed")
	}
	return nil
}

// reconcileNodeCertificates is a helper method that calls evaluateAndSignNodeCert on all the
// nodes.
func (s *Server) reconcileNodeCertificates(ctx context.Context, nodes []*api.Node) error {
	for _, node := range nodes {
		s.evaluateAndSignNodeCert(ctx, node)
	}

	return nil
}

// A successfully issued certificate and a failed certificate are our current final states
func isFinalState(status api.IssuanceStatus) bool {
	if status.State == api.IssuanceStateIssued || status.State == api.IssuanceStateFailed ||
		status.State == api.IssuanceStateRotate {
		return true
	}

	return false
}

// RootCAFromAPI creates a RootCA object from an api.RootCA object
func RootCAFromAPI(ctx context.Context, apiRootCA *api.RootCA, expiry time.Duration) (RootCA, error) {
	var intermediates []byte
	signingCert := apiRootCA.CACert
	signingKey := apiRootCA.CAKey
	if apiRootCA.RootRotation != nil {
		signingCert = apiRootCA.RootRotation.CrossSignedCACert
		signingKey = apiRootCA.RootRotation.CAKey
		intermediates = apiRootCA.RootRotation.CrossSignedCACert
	}
	if signingKey == nil {
		signingCert = nil
	}
	return NewRootCA(apiRootCA.CACert, signingCert, signingKey, expiry, intermediates)
}
