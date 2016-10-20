package node

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/boltdb/bolt"
	"github.com/docker/swarmkit/agent"
	"github.com/docker/swarmkit/agent/exec"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/ca"
	"github.com/docker/swarmkit/ioutils"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager"
	"github.com/docker/swarmkit/remotes"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const stateFilename = "state.json"

var (
	errNodeStarted    = errors.New("node: already started")
	errNodeNotStarted = errors.New("node: not started")
)

// Config provides values for a Node.
type Config struct {
	// Hostname is the name of host for agent instance.
	Hostname string

	// JoinAddr specifies node that should be used for the initial connection to
	// other manager in cluster. This should be only one address and optional,
	// the actual remotes come from the stored state.
	JoinAddr string

	// StateDir specifies the directory the node uses to keep the state of the
	// remote managers and certificates.
	StateDir string

	// JoinToken is the token to be used on the first certificate request.
	JoinToken string

	// ExternalCAs is a list of CAs to which a manager node
	// will make certificate signing requests for node certificates.
	ExternalCAs []*api.ExternalCA

	// ForceNewCluster creates a new cluster from current raft state.
	ForceNewCluster bool

	// ListenControlAPI specifies address the control API should listen on.
	ListenControlAPI string

	// ListenRemoteAPI specifies the address for the remote API that agents
	// and raft members connect to.
	ListenRemoteAPI string

	// AdvertiseRemoteAPI specifies the address that should be advertised
	// for connections to the remote API (including the raft service).
	AdvertiseRemoteAPI string

	// Executor specifies the executor to use for the agent.
	Executor exec.Executor

	// ElectionTick defines the amount of ticks needed without
	// leader to trigger a new election
	ElectionTick uint32

	// HeartbeatTick defines the amount of ticks between each
	// heartbeat sent to other members for health-check purposes
	HeartbeatTick uint32
}

// Node implements the primary node functionality for a member of a swarm
// cluster. Node handles workloads and may also run as a manager.
type Node struct {
	sync.RWMutex
	config               *Config
	remotes              *persistentRemotes
	role                 string
	roleCond             *sync.Cond
	conn                 *grpc.ClientConn
	connCond             *sync.Cond
	nodeID               string
	nodeMembership       api.NodeSpec_Membership
	started              chan struct{}
	startOnce            sync.Once
	stopped              chan struct{}
	stopOnce             sync.Once
	ready                chan struct{} // closed when agent has completed registration and manager(if enabled) is ready to receive control requests
	certificateRequested chan struct{} // closed when certificate issue request has been sent by node
	closed               chan struct{}
	err                  error
	agent                *agent.Agent
	manager              *manager.Manager
	roleChangeReq        chan api.NodeRole // used to send role updates from the dispatcher api on promotion/demotion
}

// RemoteAPIAddr returns address on which remote manager api listens.
// Returns nil if node is not manager.
func (n *Node) RemoteAPIAddr() (string, error) {
	n.RLock()
	defer n.RUnlock()
	if n.manager == nil {
		return "", errors.Errorf("node is not manager")
	}
	addr := n.manager.Addr()
	if addr == nil {
		return "", errors.Errorf("manager addr is not set")
	}
	return addr.String(), nil
}

// New returns new Node instance.
func New(c *Config) (*Node, error) {
	if err := os.MkdirAll(c.StateDir, 0700); err != nil {
		return nil, err
	}
	stateFile := filepath.Join(c.StateDir, stateFilename)
	dt, err := ioutil.ReadFile(stateFile)
	var p []api.Peer
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil {
		if err := json.Unmarshal(dt, &p); err != nil {
			return nil, err
		}
	}

	n := &Node{
		remotes:              newPersistentRemotes(stateFile, p...),
		role:                 ca.WorkerRole,
		config:               c,
		started:              make(chan struct{}),
		stopped:              make(chan struct{}),
		closed:               make(chan struct{}),
		ready:                make(chan struct{}),
		certificateRequested: make(chan struct{}),
		roleChangeReq:        make(chan api.NodeRole, 1),
	}
	n.roleCond = sync.NewCond(n.RLocker())
	n.connCond = sync.NewCond(n.RLocker())
	if err := n.loadCertificates(); err != nil {
		return nil, err
	}
	return n, nil
}

// Start starts a node instance.
func (n *Node) Start(ctx context.Context) error {
	err := errNodeStarted

	n.startOnce.Do(func() {
		close(n.started)
		go n.run(ctx)
		err = nil // clear error above, only once.
	})

	return err
}

func (n *Node) run(ctx context.Context) (err error) {
	defer func() {
		n.err = err
		close(n.closed)
	}()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx = log.WithModule(ctx, "node")

	go func() {
		select {
		case <-ctx.Done():
		case <-n.stopped:
			cancel()
		}
	}()

	// NOTE: When this node is created by NewNode(), our nodeID is set if
	// n.loadCertificates() succeeded in loading TLS credentials.
	if n.config.JoinAddr == "" && n.nodeID == "" {
		if err := n.bootstrapCA(); err != nil {
			return err
		}
	}

	if n.config.JoinAddr != "" || n.config.ForceNewCluster {
		n.remotes = newPersistentRemotes(filepath.Join(n.config.StateDir, stateFilename))
		if n.config.JoinAddr != "" {
			n.remotes.Observe(api.Peer{Addr: n.config.JoinAddr}, remotes.DefaultObservationWeight)
		}
	}

	// Obtain new certs and setup TLS certificates renewal for this node:
	// - We call LoadOrCreateSecurityConfig which blocks until a valid certificate has been issued
	// - We retrieve the nodeID from LoadOrCreateSecurityConfig through the info channel. This allows
	// us to display the ID before the certificate gets issued (for potential approval).
	// - We wait for LoadOrCreateSecurityConfig to finish since we need a certificate to operate.
	// - Given a valid certificate, spin a renewal go-routine that will ensure that certificates stay
	// up to date.
	issueResponseChan := make(chan api.IssueNodeCertificateResponse, 1)
	go func() {
		select {
		case <-ctx.Done():
		case resp := <-issueResponseChan:
			log.G(log.WithModule(ctx, "tls")).WithFields(logrus.Fields{
				"node.id": resp.NodeID,
			}).Debugf("requesting certificate")
			n.Lock()
			n.nodeID = resp.NodeID
			n.nodeMembership = resp.NodeMembership
			n.Unlock()
			close(n.certificateRequested)
		}
	}()

	certDir := filepath.Join(n.config.StateDir, "certificates")
	securityConfig, err := ca.LoadOrCreateSecurityConfig(ctx, certDir, n.config.JoinToken, ca.ManagerRole, n.remotes, issueResponseChan)
	if err != nil {
		return err
	}

	taskDBPath := filepath.Join(n.config.StateDir, "worker/tasks.db")
	if err := os.MkdirAll(filepath.Dir(taskDBPath), 0777); err != nil {
		return err
	}

	db, err := bolt.Open(taskDBPath, 0666, nil)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := n.loadCertificates(); err != nil {
		return err
	}

	forceCertRenewal := make(chan struct{})
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case apirole := <-n.roleChangeReq:
				n.Lock()
				lastRole := n.role
				role := ca.WorkerRole
				if apirole == api.NodeRoleManager {
					role = ca.ManagerRole
				}
				if lastRole == role {
					n.Unlock()
					continue
				}
				// switch role to agent immediately to shutdown manager early
				if role == ca.WorkerRole {
					n.role = role
					n.roleCond.Broadcast()
				}
				n.Unlock()
				select {
				case forceCertRenewal <- struct{}{}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	updates := ca.RenewTLSConfig(ctx, securityConfig, certDir, n.remotes, forceCertRenewal)
	go func() {
		for {
			select {
			case certUpdate := <-updates:
				if certUpdate.Err != nil {
					logrus.Warnf("error renewing TLS certificate: %v", certUpdate.Err)
					continue
				}
				n.Lock()
				n.role = certUpdate.Role
				n.roleCond.Broadcast()
				n.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	role := n.role

	managerReady := make(chan struct{})
	agentReady := make(chan struct{})
	var managerErr error
	var agentErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		managerErr = n.runManager(ctx, securityConfig, managerReady) // store err and loop
		wg.Done()
		cancel()
	}()
	go func() {
		agentErr = n.runAgent(ctx, db, securityConfig.ClientTLSCreds, agentReady)
		wg.Done()
		cancel()
	}()

	go func() {
		<-agentReady
		if role == ca.ManagerRole {
			<-managerReady
		}
		close(n.ready)
	}()

	wg.Wait()
	if managerErr != nil && managerErr != context.Canceled {
		return managerErr
	}
	if agentErr != nil && agentErr != context.Canceled {
		return agentErr
	}
	return err
}

// Stop stops node execution
func (n *Node) Stop(ctx context.Context) error {
	select {
	case <-n.started:
	default:
		return errNodeNotStarted
	}

	n.stopOnce.Do(func() {
		close(n.stopped)
	})

	select {
	case <-n.closed:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Err returns the error that caused the node to shutdown or nil. Err blocks
// until the node has fully shut down.
func (n *Node) Err(ctx context.Context) error {
	select {
	case <-n.closed:
		return n.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (n *Node) runAgent(ctx context.Context, db *bolt.DB, creds credentials.TransportCredentials, ready chan<- struct{}) error {
	select {
	case <-ctx.Done():
	case <-n.remotes.WaitSelect(ctx):
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	a, err := agent.New(&agent.Config{
		Hostname:         n.config.Hostname,
		Managers:         n.remotes,
		Executor:         n.config.Executor,
		DB:               db,
		NotifyRoleChange: n.roleChangeReq,
		Credentials:      creds,
	})
	if err != nil {
		return err
	}
	if err := a.Start(ctx); err != nil {
		return err
	}

	n.Lock()
	n.agent = a
	n.Unlock()

	defer func() {
		n.Lock()
		n.agent = nil
		n.Unlock()
	}()

	go func() {
		<-a.Ready()
		close(ready)
	}()

	// todo: manually call stop on context cancellation?

	return a.Err(context.Background())
}

// Ready returns a channel that is closed after node's initialization has
// completes for the first time.
func (n *Node) Ready() <-chan struct{} {
	return n.ready
}

// CertificateRequested returns a channel that is closed after node has
// requested a certificate. After this call a caller can expect calls to
// NodeID() and `NodeMembership()` to succeed.
func (n *Node) CertificateRequested() <-chan struct{} {
	return n.certificateRequested
}

func (n *Node) setControlSocket(conn *grpc.ClientConn) {
	n.Lock()
	if n.conn != nil {
		n.conn.Close()
	}
	n.conn = conn
	n.connCond.Broadcast()
	n.Unlock()
}

// ListenControlSocket listens changes of a connection for managing the
// cluster control api
func (n *Node) ListenControlSocket(ctx context.Context) <-chan *grpc.ClientConn {
	c := make(chan *grpc.ClientConn, 1)
	n.RLock()
	conn := n.conn
	c <- conn
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			n.connCond.Broadcast()
		case <-done:
		}
	}()
	go func() {
		defer close(c)
		defer close(done)
		defer n.RUnlock()
		for {
			if ctx.Err() != nil {
				return
			}
			if conn == n.conn {
				n.connCond.Wait()
				continue
			}
			conn = n.conn
			c <- conn
		}
	}()
	return c
}

// NodeID returns current node's ID. May be empty if not set.
func (n *Node) NodeID() string {
	n.RLock()
	defer n.RUnlock()
	return n.nodeID
}

// NodeMembership returns current node's membership. May be empty if not set.
func (n *Node) NodeMembership() api.NodeSpec_Membership {
	n.RLock()
	defer n.RUnlock()
	return n.nodeMembership
}

// Manager returns manager instance started by node. May be nil.
func (n *Node) Manager() *manager.Manager {
	n.RLock()
	defer n.RUnlock()
	return n.manager
}

// Agent returns agent instance started by node. May be nil.
func (n *Node) Agent() *agent.Agent {
	n.RLock()
	defer n.RUnlock()
	return n.agent
}

// Remotes returns a list of known peers known to node.
func (n *Node) Remotes() []api.Peer {
	weights := n.remotes.Weights()
	remotes := make([]api.Peer, 0, len(weights))
	for p := range weights {
		remotes = append(remotes, p)
	}
	return remotes
}

func (n *Node) loadCertificates() error {
	certDir := filepath.Join(n.config.StateDir, "certificates")
	rootCA, err := ca.GetLocalRootCA(certDir)
	if err != nil {
		if err == ca.ErrNoLocalRootCA {
			return nil
		}
		return err
	}
	configPaths := ca.NewConfigPaths(certDir)
	clientTLSCreds, _, err := ca.LoadTLSCreds(rootCA, configPaths.Node)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return errors.Wrapf(err, "error while loading TLS Certificate in %s", configPaths.Node.Cert)
	}
	// todo: try csr if no cert or store nodeID/role in some other way
	n.Lock()
	n.role = clientTLSCreds.Role()
	n.nodeID = clientTLSCreds.NodeID()
	n.nodeMembership = api.NodeMembershipAccepted
	n.roleCond.Broadcast()
	n.Unlock()

	return nil
}

func (n *Node) bootstrapCA() error {
	if err := ca.BootstrapCluster(filepath.Join(n.config.StateDir, "certificates")); err != nil {
		return err
	}
	return n.loadCertificates()
}

func (n *Node) initManagerConnection(ctx context.Context, ready chan<- struct{}) error {
	opts := []grpc.DialOption{}
	insecureCreds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
	opts = append(opts, grpc.WithTransportCredentials(insecureCreds))
	// Using listen address instead of advertised address because this is a
	// local connection.
	addr := n.config.ListenControlAPI
	opts = append(opts, grpc.WithDialer(
		func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))
	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		return err
	}
	client := api.NewHealthClient(conn)
	for {
		resp, err := client.Check(ctx, &api.HealthCheckRequest{Service: "ControlAPI"})
		if err != nil {
			return err
		}
		if resp.Status == api.HealthCheckResponse_SERVING {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	n.setControlSocket(conn)
	if ready != nil {
		close(ready)
	}
	return nil
}

func (n *Node) waitRole(ctx context.Context, role string) error {
	n.roleCond.L.Lock()
	if role == n.role {
		n.roleCond.L.Unlock()
		return nil
	}
	finishCh := make(chan struct{})
	defer close(finishCh)
	go func() {
		select {
		case <-finishCh:
		case <-ctx.Done():
			// call broadcast to shutdown this function
			n.roleCond.Broadcast()
		}
	}()
	defer n.roleCond.L.Unlock()
	for role != n.role {
		n.roleCond.Wait()
		select {
		case <-ctx.Done():
			if ctx.Err() != nil {
				return ctx.Err()
			}
		default:
		}
	}

	return nil
}

func (n *Node) runManager(ctx context.Context, securityConfig *ca.SecurityConfig, ready chan struct{}) error {
	for {
		if err := n.waitRole(ctx, ca.ManagerRole); err != nil {
			return err
		}

		remoteAddr, _ := n.remotes.Select(n.NodeID())
		m, err := manager.New(&manager.Config{
			ForceNewCluster: n.config.ForceNewCluster,
			ProtoAddr: map[string]string{
				"tcp":  n.config.ListenRemoteAPI,
				"unix": n.config.ListenControlAPI,
			},
			AdvertiseAddr:  n.config.AdvertiseRemoteAPI,
			SecurityConfig: securityConfig,
			ExternalCAs:    n.config.ExternalCAs,
			JoinRaft:       remoteAddr.Addr,
			StateDir:       n.config.StateDir,
			HeartbeatTick:  n.config.HeartbeatTick,
			ElectionTick:   n.config.ElectionTick,
		})
		if err != nil {
			return err
		}
		done := make(chan struct{})
		var runErr error
		go func() {
			runErr = m.Run(context.Background())
			close(done)
		}()

		n.Lock()
		n.manager = m
		n.Unlock()

		connCtx, connCancel := context.WithCancel(ctx)
		go n.initManagerConnection(connCtx, ready)

		// this happens only on initial start
		if ready != nil {
			go func(ready chan struct{}) {
				select {
				case <-ready:
					addr, err := n.RemoteAPIAddr()
					if err != nil {
						log.G(ctx).WithError(err).Errorf("get remote api addr")
					} else {
						n.remotes.Observe(api.Peer{NodeID: n.NodeID(), Addr: addr}, remotes.DefaultObservationWeight)
					}
				case <-connCtx.Done():
				}
			}(ready)
			ready = nil
		}

		roleChanged := make(chan error)
		waitCtx, waitCancel := context.WithCancel(ctx)
		go func() {
			err := n.waitRole(waitCtx, ca.WorkerRole)
			roleChanged <- err
		}()

		select {
		case <-done:
			// Fail out if m.Run() returns error, otherwise wait for
			// role change.
			if runErr != nil {
				err = runErr
			} else {
				err = <-roleChanged
			}
		case err = <-roleChanged:
		}

		n.Lock()
		n.manager = nil
		n.Unlock()

		select {
		case <-done:
		case <-ctx.Done():
			err = ctx.Err()
			m.Stop(context.Background())
			<-done
		}
		connCancel()
		n.setControlSocket(nil)
		waitCancel()

		if err != nil {
			return err
		}
	}
}

type persistentRemotes struct {
	sync.RWMutex
	c *sync.Cond
	remotes.Remotes
	storePath      string
	lastSavedState []api.Peer
}

func newPersistentRemotes(f string, peers ...api.Peer) *persistentRemotes {
	pr := &persistentRemotes{
		storePath: f,
		Remotes:   remotes.NewRemotes(peers...),
	}
	pr.c = sync.NewCond(pr.RLocker())
	return pr
}

func (s *persistentRemotes) Observe(peer api.Peer, weight int) {
	s.Lock()
	defer s.Unlock()
	s.Remotes.Observe(peer, weight)
	s.c.Broadcast()
	if err := s.save(); err != nil {
		logrus.Errorf("error writing cluster state file: %v", err)
		return
	}
	return
}
func (s *persistentRemotes) Remove(peers ...api.Peer) {
	s.Lock()
	defer s.Unlock()
	s.Remotes.Remove(peers...)
	if err := s.save(); err != nil {
		logrus.Errorf("error writing cluster state file: %v", err)
		return
	}
	return
}

func (s *persistentRemotes) save() error {
	weights := s.Weights()
	remotes := make([]api.Peer, 0, len(weights))
	for r := range weights {
		remotes = append(remotes, r)
	}
	sort.Sort(sortablePeers(remotes))
	if reflect.DeepEqual(remotes, s.lastSavedState) {
		return nil
	}
	dt, err := json.Marshal(remotes)
	if err != nil {
		return err
	}
	s.lastSavedState = remotes
	return ioutils.AtomicWriteFile(s.storePath, dt, 0600)
}

// WaitSelect waits until at least one remote becomes available and then selects one.
func (s *persistentRemotes) WaitSelect(ctx context.Context) <-chan api.Peer {
	c := make(chan api.Peer, 1)
	s.RLock()
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			s.c.Broadcast()
		case <-done:
		}
	}()
	go func() {
		defer s.RUnlock()
		defer close(c)
		defer close(done)
		for {
			if ctx.Err() != nil {
				return
			}
			p, err := s.Select()
			if err == nil {
				c <- p
				return
			}
			s.c.Wait()
		}
	}()
	return c
}

// sortablePeers is a sort wrapper for []api.Peer
type sortablePeers []api.Peer

func (sp sortablePeers) Less(i, j int) bool { return sp[i].NodeID < sp[j].NodeID }

func (sp sortablePeers) Len() int { return len(sp) }

func (sp sortablePeers) Swap(i, j int) { sp[i], sp[j] = sp[j], sp[i] }
