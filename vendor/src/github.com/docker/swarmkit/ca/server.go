package ca

import (
	"crypto/subtle"
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/state"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/docker/swarmkit/protobuf/ptypes"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// Server is the CA and NodeCA API gRPC server.
// TODO(aaronl): At some point we may want to have separate implementations of
// CA, NodeCA, and other hypothetical future CA services. At the moment,
// breaking it apart doesn't seem worth it.
type Server struct {
	mu               sync.Mutex
	wg               sync.WaitGroup
	ctx              context.Context
	cancel           func()
	store            *store.MemoryStore
	securityConfig   *SecurityConfig
	acceptancePolicy *api.AcceptancePolicy
}

// DefaultAcceptancePolicy returns the default acceptance policy.
func DefaultAcceptancePolicy() api.AcceptancePolicy {
	return api.AcceptancePolicy{
		Policies: []*api.AcceptancePolicy_RoleAdmissionPolicy{
			{
				Role:       api.NodeRoleWorker,
				Autoaccept: true,
			},
			{
				Role:       api.NodeRoleManager,
				Autoaccept: false,
			},
		},
	}
}

// DefaultCAConfig returns the default CA Config, with a default expiration.
func DefaultCAConfig() api.CAConfig {
	return api.CAConfig{
		NodeCertExpiry: ptypes.DurationProto(DefaultNodeCertExpiration),
	}
}

// NewServer creates a CA API server.
func NewServer(store *store.MemoryStore, securityConfig *SecurityConfig) *Server {
	return &Server{
		store:          store,
		securityConfig: securityConfig,
	}
}

// NodeCertificateStatus returns the current issuance status of an issuance request identified by the nodeID
func (s *Server) NodeCertificateStatus(ctx context.Context, request *api.NodeCertificateStatusRequest) (*api.NodeCertificateStatusResponse, error) {
	if request.NodeID == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, codes.InvalidArgument.String())
	}

	if err := s.addTask(); err != nil {
		return nil, err
	}
	defer s.doneTask()

	var node *api.Node

	event := state.EventUpdateNode{
		Node:   &api.Node{ID: request.NodeID},
		Checks: []state.NodeCheckFunc{state.NodeCheckID},
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
			case state.EventUpdateNode:
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
		case <-s.ctx.Done():
			return nil, s.ctx.Err()
		}
	}
}

// IssueNodeCertificate receives requests from a remote client indicating a node type and a CSR,
// returning a certificate chain signed by the local CA, if available.
func (s *Server) IssueNodeCertificate(ctx context.Context, request *api.IssueNodeCertificateRequest) (*api.IssueNodeCertificateResponse, error) {
	// First, let's see if the remote node is proposing to be added as a valid node, and with a valid CSR
	if len(request.CSR) == 0 || (request.Role != api.NodeRoleWorker && request.Role != api.NodeRoleManager) {
		return nil, grpc.Errorf(codes.InvalidArgument, codes.InvalidArgument.String())
	}

	if err := s.addTask(); err != nil {
		return nil, err
	}
	defer s.doneTask()

	// If the remote node is an Agent (either forwarded by a manager, or calling directly),
	// issue a renew agent certificate entry with the correct ID
	nodeID, err := AuthorizeForwardedRoleAndOrg(ctx, []string{AgentRole}, []string{ManagerRole}, s.securityConfig.ClientTLSCreds.Organization())
	if err == nil {
		return s.issueRenewCertificate(ctx, nodeID, request.CSR)
	}

	// If the remote node is a Manager, issue a renew certificate entry with the correct ID
	nodeID, err = AuthorizeForwardedRoleAndOrg(ctx, []string{ManagerRole}, []string{ManagerRole}, s.securityConfig.ClientTLSCreds.Organization())
	if err == nil {
		return s.issueRenewCertificate(ctx, nodeID, request.CSR)
	}

	// The remote node didn't successfully present a valid MTLS certificate, let's issue
	// a pending certificate with a new ID
	// By default all nodes start out as PENDING
	nodeMembership := api.NodeMembershipPending
	// Attempt to retrieve a policy for the role
	policy := s.getRolePolicy(request.Role)
	if policy != nil {
		// If we have a secret configured, constant time compare!
		if policy.Secret != "" &&
			subtle.ConstantTimeCompare([]byte(request.Secret), []byte(policy.Secret)) != 1 {
			return nil, grpc.Errorf(codes.InvalidArgument, "A valid secret token is necessary to join this cluster")
		}
		// Check to see if our autoacceptance policy allows this node to be issued without manual intervention
		if policy.Autoaccept {
			nodeMembership = api.NodeMembershipAccepted
		}
	}

	// Max number of collisions of ID or CN to tolerate before giving up
	maxRetries := 3
	// Generate a random ID for this new node
	for i := 0; ; i++ {
		nodeID = identity.NewNodeID()

		err := s.store.Update(func(tx store.Tx) error {
			node := &api.Node{
				ID: nodeID,
				Certificate: api.Certificate{
					CSR:  request.CSR,
					CN:   nodeID,
					Role: request.Role,
					Status: api.IssuanceStatus{
						State: api.IssuanceStatePending,
					},
				},
				Spec: api.NodeSpec{
					Role:       request.Role,
					Membership: nodeMembership,
				},
			}

			return store.CreateNode(tx, node)
		})
		if err == nil {
			log.G(ctx).WithFields(logrus.Fields{
				"node.id":   nodeID,
				"node.role": request.Role,
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
			"node.role": request.Role,
			"method":    "IssueNodeCertificate",
		}).Errorf("randomly generated node ID collided with an existing one - retrying")
	}

	return &api.IssueNodeCertificateResponse{
		NodeID: nodeID,
	}, nil
}

func (s *Server) getRolePolicy(role api.NodeRole) *api.AcceptancePolicy_RoleAdmissionPolicy {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.acceptancePolicy != nil && len(s.acceptancePolicy.Policies) > 0 {
		// Let's go through all the configured policies and try to find one for this role
		for _, p := range s.acceptancePolicy.Policies {
			if role == p.Role {
				return p
			}
		}
	}

	return nil
}

func (s *Server) issueRenewCertificate(ctx context.Context, nodeID string, csr []byte) (*api.IssueNodeCertificateResponse, error) {
	var cert api.Certificate
	err := s.store.Update(func(tx store.Tx) error {

		node := store.GetNode(tx, nodeID)
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
			Role: node.Spec.Role,
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
		NodeID: nodeID,
	}, nil
}

// GetRootCACertificate returns the certificate of the Root CA.
func (s *Server) GetRootCACertificate(ctx context.Context, request *api.GetRootCACertificateRequest) (*api.GetRootCACertificateResponse, error) {
	log.G(ctx).WithFields(logrus.Fields{
		"method": "GetRootCACertificate",
	})

	return &api.GetRootCACertificateResponse{
		Certificate: s.securityConfig.RootCA().Cert,
	}, nil
}

// Run runs the CA signer main loop.
// The CA signer can be stopped with cancelling ctx or calling Stop().
func (s *Server) Run(ctx context.Context) error {
	s.mu.Lock()
	if s.isRunning() {
		s.mu.Unlock()
		return fmt.Errorf("CA signer is stopped")
	}
	s.wg.Add(1)
	defer s.wg.Done()
	logger := log.G(ctx).WithField("module", "ca")
	ctx = log.WithLogger(ctx, logger)
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()

	var nodes []*api.Node
	updates, cancel, err := store.ViewAndWatch(
		s.store,
		func(readTx store.ReadTx) error {
			clusters, err := store.FindClusters(readTx, store.ByName(store.DefaultClusterName))
			if err != nil {
				return err
			}
			if len(clusters) != 1 {
				return fmt.Errorf("could not find cluster object")
			}
			s.updateCluster(ctx, clusters[0])

			nodes, err = store.FindNodes(readTx, store.All)
			return err
		},
		state.EventCreateNode{},
		state.EventUpdateNode{},
		state.EventUpdateCluster{},
	)
	if err != nil {
		log.G(ctx).WithFields(logrus.Fields{
			"method": "(*Server).Run",
		}).WithError(err).Errorf("snapshot store view failed")
		return err
	}
	defer cancel()

	if err := s.reconcileNodeCertificates(ctx, nodes); err != nil {
		// We don't return here because that means the Run loop would
		// never run. Log an error instead.
		log.G(ctx).WithFields(logrus.Fields{
			"method": "(*Server).Run",
		}).WithError(err).Errorf("error attempting to reconcile certificates")
	}

	// Watch for changes.
	for {
		select {
		case event := <-updates:
			switch v := event.(type) {
			case state.EventCreateNode:
				s.evaluateAndSignNodeCert(ctx, v.Node)
			case state.EventUpdateNode:
				s.evaluateAndSignNodeCert(ctx, v.Node)
			case state.EventUpdateCluster:
				s.updateCluster(ctx, v.Cluster)
			}

		case <-ctx.Done():
			return ctx.Err()
		case <-s.ctx.Done():
			return nil
		}
	}
}

// Stop stops the CA and closes all grpc streams.
func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.isRunning() {
		return fmt.Errorf("CA signer is already stopped")
	}
	s.cancel()
	s.mu.Unlock()
	// wait for all handlers to finish their CA deals,
	s.wg.Wait()
	return nil
}

func (s *Server) addTask() error {
	s.mu.Lock()
	if !s.isRunning() {
		s.mu.Unlock()
		return grpc.Errorf(codes.Aborted, "CA signer is stopped")
	}
	s.wg.Add(1)
	s.mu.Unlock()
	return nil
}

func (s *Server) doneTask() {
	s.wg.Done()
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

func (s *Server) updateCluster(ctx context.Context, cluster *api.Cluster) {
	s.mu.Lock()
	s.acceptancePolicy = cluster.Spec.AcceptancePolicy.Copy()
	s.mu.Unlock()
	var err error
	// If the cluster has a RootCA, let's try to update our SecurityConfig to reflect the latest values
	if cluster.RootCA != nil && len(cluster.RootCA.CACert) != 0 && len(cluster.RootCA.CAKey) != 0 {
		expiry := DefaultNodeCertExpiration
		if cluster.Spec.CAConfig.NodeCertExpiry != nil {
			// NodeCertExpiry exists, let's try to parse the duration out of it
			clusterExpiry, err := ptypes.Duration(cluster.Spec.CAConfig.NodeCertExpiry)
			if err != nil {
				log.G(ctx).WithFields(logrus.Fields{
					"cluster.id": cluster.ID,
					"method":     "(*Server).updateCluster",
				}).WithError(err).Warn("failed to parse certificate expiration, using default")
			} else {
				// We were able to successfully parse the expiration out of the cluster.
				expiry = clusterExpiry
			}
		} else {
			// NodeCertExpiry seems to be nil
			log.G(ctx).WithFields(logrus.Fields{
				"cluster.id": cluster.ID,
				"method":     "(*Server).updateCluster",
			}).WithError(err).Warn("failed to parse certificate expiration, using default")

		}
		rCA := cluster.RootCA
		err = s.securityConfig.UpdateRootCA(rCA.CACert, rCA.CAKey, expiry)
		if err != nil {
			log.G(ctx).WithFields(logrus.Fields{
				"cluster.id": cluster.ID,
				"method":     "(*Server).updateCluster",
			}).WithError(err).Error("updating root key failed")
		} else {
			log.G(ctx).WithFields(logrus.Fields{
				"cluster.id": cluster.ID,
				"method":     "(*Server).updateCluster",
			}).Debugf("root CA updated successfully")
		}
	}
}

func (s *Server) setNodeCertState(node *api.Node, state api.IssuanceStatus_State) error {
	return s.store.Update(func(tx store.Tx) error {
		latestNode := store.GetNode(tx, node.ID)
		if latestNode == nil {
			return store.ErrNotExist
		}

		// Remote users are expecting a full certificate chain, not just a signed certificate
		latestNode.Certificate.Status = api.IssuanceStatus{
			State: state,
		}

		return store.UpdateNode(tx, latestNode)
	})
}

func (s *Server) evaluateAndSignNodeCert(ctx context.Context, node *api.Node) {
	// If the desired membership and actual state are in sync, there's
	// nothing to do.
	if node.Spec.Membership == api.NodeMembershipAccepted && node.Certificate.Status.State == api.IssuanceStateIssued {
		return
	}
	if node.Spec.Membership == api.NodeMembershipRejected && node.Certificate.Status.State == api.IssuanceStateRejected {
		return
	}

	// If the desired membership was set to rejected, we should
	// act on that right away, and that is all that should be done.
	if node.Spec.Membership == api.NodeMembershipRejected {
		err := s.setNodeCertState(node, api.IssuanceStateRejected)
		if err != nil {
			log.G(ctx).WithFields(logrus.Fields{
				"node.id":   node.ID,
				"node.role": node.Certificate.Role,
				"method":    "(*Server).evaluateAndSignCert",
			}).WithError(err).Errorf("failed to change certificate state")
		}
		return
	}

	// If the certificate state is renew, then it is a server-sided accepted cert (cert renewals)
	if node.Certificate.Status.State == api.IssuanceStateRenew {
		s.signNodeCert(ctx, node)
		return
	}

	// If the certificate state is not pending at this point, we are in an unknown state, return
	if node.Certificate.Status.State != api.IssuanceStatePending {
		return
	}

	// Only issue this node if the admin explicitly changed it to Accepted
	if node.Spec.Membership == api.NodeMembershipAccepted {
		// Cert was approved by admin
		s.signNodeCert(ctx, node)
	}
}

func (s *Server) signNodeCert(ctx context.Context, node *api.Node) {
	if !s.securityConfig.RootCA().CanSign() {
		log.G(ctx).Error("no valid signer found")
		return
	}

	node = node.Copy()
	nodeID := node.ID
	// Convert the role from proto format
	role, err := ParseRole(node.Certificate.Role)
	if err != nil {
		log.G(ctx).WithFields(logrus.Fields{
			"node.id": node.ID,
			"method":  "(*Server).signNodeCert",
		}).WithError(err).Errorf("failed to parse role")
		return
	}
	// Attempt to sign the CSR
	cert, err := s.securityConfig.RootCA().ParseValidateAndSignCSR(node.Certificate.CSR, node.Certificate.CN, role, s.securityConfig.ClientTLSCreds.Organization())
	if err != nil {
		log.G(ctx).WithFields(logrus.Fields{
			"node.id": node.ID,
			"method":  "(*Server).signNodeCert",
		}).WithError(err).Errorf("failed to parse CSR")
		return
	}

	// We were able to successfully sign the new CSR. Let's try to update the nodeStore
	for {
		err = s.store.Update(func(tx store.Tx) error {
			// Remote nodes are expecting a full certificate chain, not just a signed certificate
			node.Certificate.Certificate = append(cert, s.securityConfig.RootCA().Cert...)
			node.Certificate.Status = api.IssuanceStatus{
				State: api.IssuanceStateIssued,
			}

			err := store.UpdateNode(tx, node)
			if err != nil {
				node = store.GetNode(tx, nodeID)
				if node == nil {
					err = fmt.Errorf("node %s does not exist", nodeID)
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
			break
		}
		if err == store.ErrSequenceConflict {
			continue
		}

		log.G(ctx).WithFields(logrus.Fields{
			"node.id": nodeID,
			"method":  "(*Server).signNodeCert",
		}).WithError(err).Errorf("transaction failed")
		return
	}

}

func (s *Server) reconcileNodeCertificates(ctx context.Context, nodes []*api.Node) error {
	for _, node := range nodes {
		s.evaluateAndSignNodeCert(ctx, node)
	}

	return nil
}

func isFinalState(status api.IssuanceStatus) bool {
	if status.State != api.IssuanceStatePending &&
		status.State != api.IssuanceStateRenew {
		return true
	}

	return false
}
