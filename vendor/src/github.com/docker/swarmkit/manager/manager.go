package manager

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/ca"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/allocator"
	"github.com/docker/swarmkit/manager/controlapi"
	"github.com/docker/swarmkit/manager/dispatcher"
	"github.com/docker/swarmkit/manager/keymanager"
	"github.com/docker/swarmkit/manager/orchestrator"
	"github.com/docker/swarmkit/manager/raftpicker"
	"github.com/docker/swarmkit/manager/scheduler"
	"github.com/docker/swarmkit/manager/state/raft"
	"github.com/docker/swarmkit/manager/state/store"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	// defaultTaskHistoryRetentionLimit is the number of tasks to keep.
	defaultTaskHistoryRetentionLimit = 10
)

// Config is used to tune the Manager.
type Config struct {
	SecurityConfig *ca.SecurityConfig

	ProtoAddr map[string]string
	// ProtoListener will be used for grpc serving if it's not nil,
	// ProtoAddr fields will be used to create listeners otherwise.
	ProtoListener map[string]net.Listener

	// JoinRaft is an optional address of a node in an existing raft
	// cluster to join.
	JoinRaft string

	// Top-level state directory
	StateDir string

	// ForceNewCluster defines if we have to force a new cluster
	// because we are recovering from a backup data directory.
	ForceNewCluster bool

	// ElectionTick defines the amount of ticks needed without
	// leader to trigger a new election
	ElectionTick uint32

	// HeartbeatTick defines the amount of ticks between each
	// heartbeat sent to other members for health-check purposes
	HeartbeatTick uint32
}

// Manager is the cluster manager for Swarm.
// This is the high-level object holding and initializing all the manager
// subsystems.
type Manager struct {
	config    *Config
	listeners map[string]net.Listener

	caserver               *ca.Server
	Dispatcher             *dispatcher.Dispatcher
	replicatedOrchestrator *orchestrator.ReplicatedOrchestrator
	globalOrchestrator     *orchestrator.GlobalOrchestrator
	taskReaper             *orchestrator.TaskReaper
	scheduler              *scheduler.Scheduler
	allocator              *allocator.Allocator
	keyManager             *keymanager.KeyManager
	server                 *grpc.Server
	localserver            *grpc.Server
	RaftNode               *raft.Node

	mu   sync.Mutex
	once sync.Once

	stopped chan struct{}
}

// New creates a Manager which has not started to accept requests yet.
func New(config *Config) (*Manager, error) {
	dispatcherConfig := dispatcher.DefaultConfig()

	if config.ProtoAddr == nil {
		config.ProtoAddr = make(map[string]string)
	}

	if config.ProtoListener != nil && config.ProtoListener["tcp"] != nil {
		config.ProtoAddr["tcp"] = config.ProtoListener["tcp"].Addr().String()
	}

	tcpAddr := config.ProtoAddr["tcp"]

	listenHost, listenPort, err := net.SplitHostPort(tcpAddr)
	if err == nil {
		ip := net.ParseIP(listenHost)
		if ip != nil && ip.IsUnspecified() {
			// Find our local IP address associated with the default route.
			// This may not be the appropriate address to use for internal
			// cluster communications, but it seems like the best default.
			// The admin can override this address if necessary.
			conn, err := net.Dial("udp", "8.8.8.8:53")
			if err != nil {
				return nil, fmt.Errorf("could not determine local IP address: %v", err)
			}
			localAddr := conn.LocalAddr().String()
			conn.Close()

			listenHost, _, err = net.SplitHostPort(localAddr)
			if err != nil {
				return nil, fmt.Errorf("could not split local IP address: %v", err)
			}

			tcpAddr = net.JoinHostPort(listenHost, listenPort)
		}
	}

	// TODO(stevvooe): Reported address of manager is plumbed to listen addr
	// for now, may want to make this separate. This can be tricky to get right
	// so we need to make it easy to override. This needs to be the address
	// through which agent nodes access the manager.
	dispatcherConfig.Addr = tcpAddr

	err = os.MkdirAll(filepath.Dir(config.ProtoAddr["unix"]), 0700)
	if err != nil {
		return nil, fmt.Errorf("failed to create socket directory: %v", err)
	}

	err = os.MkdirAll(config.StateDir, 0700)
	if err != nil {
		return nil, fmt.Errorf("failed to create state directory: %v", err)
	}

	raftStateDir := filepath.Join(config.StateDir, "raft")
	err = os.MkdirAll(raftStateDir, 0700)
	if err != nil {
		return nil, fmt.Errorf("failed to create raft state directory: %v", err)
	}

	var listeners map[string]net.Listener
	if len(config.ProtoListener) > 0 {
		listeners = config.ProtoListener
	} else {
		listeners = make(map[string]net.Listener)

		for proto, addr := range config.ProtoAddr {
			l, err := net.Listen(proto, addr)

			// A unix socket may fail to bind if the file already
			// exists. Try replacing the file.
			unwrappedErr := err
			if op, ok := unwrappedErr.(*net.OpError); ok {
				unwrappedErr = op.Err
			}
			if sys, ok := unwrappedErr.(*os.SyscallError); ok {
				unwrappedErr = sys.Err
			}
			if proto == "unix" && unwrappedErr == syscall.EADDRINUSE {
				os.Remove(addr)
				l, err = net.Listen(proto, addr)
				if err != nil {
					return nil, err
				}
			} else if err != nil {
				return nil, err
			}
			listeners[proto] = l
		}
	}

	raftCfg := raft.DefaultNodeConfig()

	if config.ElectionTick > 0 {
		raftCfg.ElectionTick = int(config.ElectionTick)
	}
	if config.HeartbeatTick > 0 {
		raftCfg.HeartbeatTick = int(config.HeartbeatTick)
	}

	newNodeOpts := raft.NewNodeOptions{
		ID:              config.SecurityConfig.ClientTLSCreds.NodeID(),
		Addr:            tcpAddr,
		JoinAddr:        config.JoinRaft,
		Config:          raftCfg,
		StateDir:        raftStateDir,
		ForceNewCluster: config.ForceNewCluster,
		TLSCredentials:  config.SecurityConfig.ClientTLSCreds,
	}
	RaftNode, err := raft.NewNode(context.TODO(), newNodeOpts)
	if err != nil {
		for _, lis := range listeners {
			lis.Close()
		}
		return nil, fmt.Errorf("can't create raft node: %v", err)
	}

	opts := []grpc.ServerOption{
		grpc.Creds(config.SecurityConfig.ServerTLSCreds)}

	m := &Manager{
		config:      config,
		listeners:   listeners,
		caserver:    ca.NewServer(RaftNode.MemoryStore(), config.SecurityConfig),
		Dispatcher:  dispatcher.New(RaftNode, dispatcherConfig),
		keyManager:  keymanager.New(RaftNode.MemoryStore(), keymanager.DefaultConfig()),
		server:      grpc.NewServer(opts...),
		localserver: grpc.NewServer(opts...),
		RaftNode:    RaftNode,
		stopped:     make(chan struct{}),
	}

	return m, nil
}

// Run starts all manager sub-systems and the gRPC server at the configured
// address.
// The call never returns unless an error occurs or `Stop()` is called.
//
// TODO(aluzzardi): /!\ This function is *way* too complex. /!\
// It needs to be split into smaller manageable functions.
func (m *Manager) Run(parent context.Context) error {
	ctx, ctxCancel := context.WithCancel(parent)
	defer ctxCancel()

	// Harakiri.
	go func() {
		select {
		case <-ctx.Done():
		case <-m.stopped:
			ctxCancel()
		}
	}()

	leadershipCh, cancel := m.RaftNode.SubscribeLeadership()
	defer cancel()

	go func() {
		for leadershipEvent := range leadershipCh {
			// read out and discard all of the messages when we've stopped
			// don't acquire the mutex yet. if stopped is closed, we don't need
			// this stops this loop from starving Run()'s attempt to Lock
			select {
			case <-m.stopped:
				continue
			default:
				// do nothing, we're not stopped
			}
			// we're not stopping so NOW acquire the mutex
			m.mu.Lock()
			newState := leadershipEvent.(raft.LeadershipState)

			if newState == raft.IsLeader {
				s := m.RaftNode.MemoryStore()

				rootCA := m.config.SecurityConfig.RootCA()
				nodeID := m.config.SecurityConfig.ClientTLSCreds.NodeID()

				raftCfg := raft.DefaultRaftConfig()
				raftCfg.ElectionTick = uint32(m.RaftNode.Config.ElectionTick)
				raftCfg.HeartbeatTick = uint32(m.RaftNode.Config.HeartbeatTick)

				clusterID := m.config.SecurityConfig.ClientTLSCreds.Organization()
				s.Update(func(tx store.Tx) error {
					// Add a default cluster object to the
					// store. Don't check the error because
					// we expect this to fail unless this
					// is a brand new cluster.
					store.CreateCluster(tx, &api.Cluster{
						ID: clusterID,
						Spec: api.ClusterSpec{
							Annotations: api.Annotations{
								Name: store.DefaultClusterName,
							},
							AcceptancePolicy: ca.DefaultAcceptancePolicy(),
							Orchestration: api.OrchestrationConfig{
								TaskHistoryRetentionLimit: defaultTaskHistoryRetentionLimit,
							},
							Dispatcher: api.DispatcherConfig{
								HeartbeatPeriod: uint64(dispatcher.DefaultHeartBeatPeriod),
							},
							Raft:     raftCfg,
							CAConfig: ca.DefaultCAConfig(),
						},
						RootCA: &api.RootCA{
							CAKey:  rootCA.Key,
							CACert: rootCA.Cert,
						},
					})
					// Add Node entry for ourself, if one
					// doesn't exist already.
					store.CreateNode(tx, &api.Node{
						ID: nodeID,
						Certificate: api.Certificate{
							CN:   nodeID,
							Role: api.NodeRoleManager,
							Status: api.IssuanceStatus{
								State: api.IssuanceStateIssued,
							},
						},
						Spec: api.NodeSpec{
							Role:       api.NodeRoleManager,
							Membership: api.NodeMembershipAccepted,
						},
					})
					return nil
				})

				// Attempt to rotate the key-encrypting-key of the root CA key-material
				err := m.rotateRootCAKEK(ctx, clusterID)
				if err != nil {
					log.G(ctx).WithError(err).Error("root key-encrypting-key rotation failed")
				}

				m.replicatedOrchestrator = orchestrator.New(s)
				m.globalOrchestrator = orchestrator.NewGlobalOrchestrator(s)
				m.taskReaper = orchestrator.NewTaskReaper(s)
				m.scheduler = scheduler.New(s)

				// TODO(stevvooe): Allocate a context that can be used to
				// shutdown underlying manager processes when leadership is
				// lost.

				m.allocator, err = allocator.New(s)
				if err != nil {
					log.G(ctx).WithError(err).Error("failed to create allocator")
					// TODO(stevvooe): It doesn't seem correct here to fail
					// creating the allocator but then use it anyways.
				}

				go func(keyManager *keymanager.KeyManager) {
					if err := keyManager.Run(ctx); err != nil {
						log.G(ctx).WithError(err).Error("keymanager failed with an error")
					}
				}(m.keyManager)

				go func(d *dispatcher.Dispatcher) {
					if err := d.Run(ctx); err != nil {
						log.G(ctx).WithError(err).Error("Dispatcher exited with an error")
					}
				}(m.Dispatcher)

				go func(server *ca.Server) {
					if err := server.Run(ctx); err != nil {
						log.G(ctx).WithError(err).Error("CA signer exited with an error")
					}
				}(m.caserver)

				// Start all sub-components in separate goroutines.
				// TODO(aluzzardi): This should have some kind of error handling so that
				// any component that goes down would bring the entire manager down.

				if m.allocator != nil {
					go func(allocator *allocator.Allocator) {
						if err := allocator.Run(ctx); err != nil {
							log.G(ctx).WithError(err).Error("allocator exited with an error")
						}
					}(m.allocator)
				}

				go func(scheduler *scheduler.Scheduler) {
					if err := scheduler.Run(ctx); err != nil {
						log.G(ctx).WithError(err).Error("scheduler exited with an error")
					}
				}(m.scheduler)
				go func(taskReaper *orchestrator.TaskReaper) {
					taskReaper.Run()
				}(m.taskReaper)
				go func(orchestrator *orchestrator.ReplicatedOrchestrator) {
					if err := orchestrator.Run(ctx); err != nil {
						log.G(ctx).WithError(err).Error("replicated orchestrator exited with an error")
					}
				}(m.replicatedOrchestrator)
				go func(globalOrchestrator *orchestrator.GlobalOrchestrator) {
					if err := globalOrchestrator.Run(ctx); err != nil {
						log.G(ctx).WithError(err).Error("global orchestrator exited with an error")
					}
				}(m.globalOrchestrator)

			} else if newState == raft.IsFollower {
				m.Dispatcher.Stop()
				m.caserver.Stop()

				if m.allocator != nil {
					m.allocator.Stop()
					m.allocator = nil
				}

				m.replicatedOrchestrator.Stop()
				m.replicatedOrchestrator = nil

				m.globalOrchestrator.Stop()
				m.globalOrchestrator = nil

				m.taskReaper.Stop()
				m.taskReaper = nil

				m.scheduler.Stop()
				m.scheduler = nil

				m.keyManager.Stop()
				m.keyManager = nil
			}
			m.mu.Unlock()
		}
	}()

	go func() {
		err := m.RaftNode.Run(ctx)
		if err != nil {
			log.G(ctx).Error(err)
			m.Stop(ctx)
		}
	}()

	proxyOpts := []grpc.DialOption{
		grpc.WithBackoffMaxDelay(2 * time.Second),
		grpc.WithTransportCredentials(m.config.SecurityConfig.ClientTLSCreds),
	}

	cs := raftpicker.NewConnSelector(m.RaftNode, proxyOpts...)

	authorize := func(ctx context.Context, roles []string) error {
		// Authorize the remote roles, ensure they can only be forwarded by managers
		_, err := ca.AuthorizeForwardedRoleAndOrg(ctx, roles, []string{ca.ManagerRole}, m.config.SecurityConfig.ClientTLSCreds.Organization())
		return err
	}

	baseControlAPI := controlapi.NewServer(m.RaftNode.MemoryStore(), m.RaftNode)

	authenticatedControlAPI := api.NewAuthenticatedWrapperControlServer(baseControlAPI, authorize)
	authenticatedDispatcherAPI := api.NewAuthenticatedWrapperDispatcherServer(m.Dispatcher, authorize)
	authenticatedCAAPI := api.NewAuthenticatedWrapperCAServer(m.caserver, authorize)
	authenticatedNodeCAAPI := api.NewAuthenticatedWrapperNodeCAServer(m.caserver, authorize)
	authenticatedRaftAPI := api.NewAuthenticatedWrapperRaftServer(m.RaftNode, authorize)

	proxyDispatcherAPI := api.NewRaftProxyDispatcherServer(authenticatedDispatcherAPI, cs, m.RaftNode, ca.WithMetadataForwardTLSInfo)
	proxyCAAPI := api.NewRaftProxyCAServer(authenticatedCAAPI, cs, m.RaftNode, ca.WithMetadataForwardTLSInfo)
	proxyNodeCAAPI := api.NewRaftProxyNodeCAServer(authenticatedNodeCAAPI, cs, m.RaftNode, ca.WithMetadataForwardTLSInfo)

	// localProxyControlAPI is a special kind of proxy. It is only wired up
	// to receive requests from a trusted local socket, and these requests
	// don't use TLS, therefore the requests it handles locally should
	// bypass authorization. When it proxies, it sends them as requests from
	// this manager rather than forwarded requests (it has no TLS
	// information to put in the metadata map).
	forwardAsOwnRequest := func(ctx context.Context) (context.Context, error) { return ctx, nil }
	localProxyControlAPI := api.NewRaftProxyControlServer(baseControlAPI, cs, m.RaftNode, forwardAsOwnRequest)

	// Everything registered on m.server should be an authenticated
	// wrapper, or a proxy wrapping an authenticated wrapper!
	api.RegisterCAServer(m.server, proxyCAAPI)
	api.RegisterNodeCAServer(m.server, proxyNodeCAAPI)
	api.RegisterRaftServer(m.server, authenticatedRaftAPI)
	api.RegisterControlServer(m.localserver, localProxyControlAPI)
	api.RegisterControlServer(m.server, authenticatedControlAPI)
	api.RegisterDispatcherServer(m.server, proxyDispatcherAPI)

	errServe := make(chan error, 2)
	for proto, l := range m.listeners {
		go func(proto string, lis net.Listener) {
			ctx := log.WithLogger(ctx, log.G(ctx).WithFields(
				logrus.Fields{
					"proto": lis.Addr().Network(),
					"addr":  lis.Addr().String()}))
			if proto == "unix" {
				log.G(ctx).Info("Listening for local connections")
				errServe <- m.localserver.Serve(lis)
			} else {
				log.G(ctx).Info("Listening for connections")
				errServe <- m.server.Serve(lis)
			}
		}(proto, l)
	}

	if err := raft.WaitForLeader(ctx, m.RaftNode); err != nil {
		m.server.Stop()
		return err
	}

	c, err := raft.WaitForCluster(ctx, m.RaftNode)
	if err != nil {
		m.server.Stop()
		return err
	}
	raftConfig := c.Spec.Raft

	if int(raftConfig.ElectionTick) != m.RaftNode.Config.ElectionTick {
		log.G(ctx).Warningf("election tick value (%ds) is different from the one defined in the cluster config (%vs), the cluster may be unstable", m.RaftNode.Config.ElectionTick, raftConfig.ElectionTick)
	}
	if int(raftConfig.HeartbeatTick) != m.RaftNode.Config.HeartbeatTick {
		log.G(ctx).Warningf("heartbeat tick value (%ds) is different from the one defined in the cluster config (%vs), the cluster may be unstable", m.RaftNode.Config.HeartbeatTick, raftConfig.HeartbeatTick)
	}

	// wait for an error in serving.
	err = <-errServe
	select {
	// check to see if stopped was posted to. if so, we're in the process of
	// stopping, or done and that's why we got the error. if stopping is
	// deliberate, stopped will ALWAYS be closed before the error is trigger,
	// so this path will ALWAYS be taken if the stop was deliberate
	case <-m.stopped:
		// shutdown was requested, do not return an error
		// but first, we wait to acquire a mutex to guarantee that stopping is
		// finished. as long as we acquire the mutex BEFORE we return, we know
		// that stopping is stopped.
		m.mu.Lock()
		m.mu.Unlock()
		return nil
	// otherwise, we'll get something from errServe, which indicates that an
	// error in serving has actually occurred and this isn't a planned shutdown
	default:
		return err
	}
}

// Stop stops the manager. It immediately closes all open connections and
// active RPCs as well as stopping the scheduler.
func (m *Manager) Stop(ctx context.Context) {
	log.G(ctx).Info("Stopping manager")

	// the mutex stops us from trying to stop while we're alrady stopping, or
	// from returning before we've finished stopping.
	m.mu.Lock()
	defer m.mu.Unlock()
	select {

	// check to see that we've already stopped
	case <-m.stopped:
		return
	default:
		// do nothing, we're stopping for the first time
	}

	// once we start stopping, send a signal that we're doing so. this tells
	// Run that we've started stopping, when it gets the error from errServe
	// it also prevents the loop from processing any more stuff.
	close(m.stopped)

	m.Dispatcher.Stop()
	m.caserver.Stop()

	if m.allocator != nil {
		m.allocator.Stop()
	}
	if m.replicatedOrchestrator != nil {
		m.replicatedOrchestrator.Stop()
	}
	if m.globalOrchestrator != nil {
		m.globalOrchestrator.Stop()
	}
	if m.taskReaper != nil {
		m.taskReaper.Stop()
	}
	if m.scheduler != nil {
		m.scheduler.Stop()
	}

	m.RaftNode.Shutdown()
	// some time after this point, Run will recieve an error from one of these
	m.server.Stop()
	m.localserver.Stop()

	log.G(ctx).Info("Manager shut down")
	// mutex is released and Run can return now
}

// rotateRootCAKEK will attempt to rotate the key-encryption-key for root CA key-material in raft.
// If there is no passphrase set in ENV, it returns.
// If there is plain-text root key-material, and a passphrase set, it encrypts it.
// If there is encrypted root key-material and it is using the current passphrase, it returns.
// If there is encrypted root key-material, and it is using the previous passphrase, it
// re-encrypts it with the current passphrase.
func (m *Manager) rotateRootCAKEK(ctx context.Context, clusterID string) error {
	// If we don't have a KEK, we won't ever be rotating anything
	strPassphrase := os.Getenv(ca.PassphraseENVVar)
	if strPassphrase == "" {
		return nil
	}
	strPassphrasePrev := os.Getenv(ca.PassphraseENVVarPrev)
	passphrase := []byte(strPassphrase)
	passphrasePrev := []byte(strPassphrasePrev)

	s := m.RaftNode.MemoryStore()
	var (
		cluster  *api.Cluster
		err      error
		finalKey []byte
	)
	// Retrieve the cluster identified by ClusterID
	s.View(func(readTx store.ReadTx) {
		cluster = store.GetCluster(readTx, clusterID)
	})
	if cluster == nil {
		return fmt.Errorf("cluster not found: %s", clusterID)
	}

	// Try to get the private key from the cluster
	privKeyPEM := cluster.RootCA.CAKey
	if privKeyPEM == nil || len(privKeyPEM) == 0 {
		// We have no PEM root private key in this cluster.
		log.G(ctx).Warnf("cluster %s does not have private key material", clusterID)
		return nil
	}

	// Decode the PEM private key
	keyBlock, _ := pem.Decode(privKeyPEM)
	if keyBlock == nil {
		return fmt.Errorf("invalid PEM-encoded private key inside of cluster %s", clusterID)
	}
	// If this key is not encrypted, then we have to encrypt it
	if !x509.IsEncryptedPEMBlock(keyBlock) {
		finalKey, err = ca.EncryptECPrivateKey(privKeyPEM, strPassphrase)
		if err != nil {
			return err
		}
	} else {
		// This key is already encrypted, let's try to decrypt with the current main passphrase
		_, err = x509.DecryptPEMBlock(keyBlock, []byte(passphrase))
		if err == nil {
			// The main key is the correct KEK, nothing to do here
			return nil
		}
		// This key is already encrypted, but failed with current main passphrase.
		// Let's try to decrypt with the previous passphrase
		unencryptedKey, err := x509.DecryptPEMBlock(keyBlock, []byte(passphrasePrev))
		if err != nil {
			// We were not able to decrypt either with the main or backup passphrase, error
			return err
		}
		unencryptedKeyBlock := &pem.Block{
			Type:    keyBlock.Type,
			Bytes:   unencryptedKey,
			Headers: keyBlock.Headers,
		}

		// We were able to decrypt the key, but with the previous passphrase. Let's encrypt
		// with the new one and store it in raft
		finalKey, err = ca.EncryptECPrivateKey(pem.EncodeToMemory(unencryptedKeyBlock), strPassphrase)
		if err != nil {
			log.G(ctx).Debugf("failed to rotate the key-encrypting-key for the root key material of cluster %s", clusterID)
			return err
		}
	}

	log.G(ctx).Infof("Re-encrypting the root key material of cluster %s", clusterID)
	// Let's update the key in the cluster object
	return s.Update(func(tx store.Tx) error {
		cluster = store.GetCluster(tx, clusterID)
		if cluster == nil {
			return fmt.Errorf("cluster not found")
		}
		cluster.RootCA.CAKey = finalKey
		return store.UpdateCluster(tx, cluster)
	})

}
