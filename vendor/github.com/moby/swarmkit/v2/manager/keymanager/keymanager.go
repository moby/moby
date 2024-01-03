package keymanager

// keymanager does the allocation, rotation and distribution of symmetric
// keys to the agents. This is to securely bootstrap network communication
// between agents. It can be used for encrypting gossip between the agents
// which is used to exchange service discovery and overlay network control
// plane information. It can also be used to encrypt overlay data traffic.
import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"sync"
	"time"

	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/log"
	"github.com/moby/swarmkit/v2/manager/state/store"
	"github.com/pkg/errors"
)

const (
	// DefaultKeyLen is the default length (in bytes) of the key allocated
	DefaultKeyLen = 16

	// DefaultKeyRotationInterval used by key manager
	DefaultKeyRotationInterval = 12 * time.Hour

	// SubsystemGossip handles gossip protocol between the agents
	SubsystemGossip = "networking:gossip"

	// SubsystemIPSec is overlay network data encryption subsystem
	SubsystemIPSec = "networking:ipsec"

	// DefaultSubsystem is gossip
	DefaultSubsystem = SubsystemGossip
	// number of keys to mainrain in the key ring.
	keyringSize = 3
)

// map of subsystems and corresponding encryption algorithm. Initially only
// AES_128 in GCM mode is supported.
var subsysToAlgo = map[string]api.EncryptionKey_Algorithm{
	SubsystemGossip: api.AES_128_GCM,
	SubsystemIPSec:  api.AES_128_GCM,
}

type keyRing struct {
	lClock uint64
	keys   []*api.EncryptionKey
}

// Config for the keymanager that can be modified
type Config struct {
	ClusterName      string
	Keylen           int
	RotationInterval time.Duration
	Subsystems       []string
}

// KeyManager handles key allocation, rotation & distribution
type KeyManager struct {
	config  *Config
	store   *store.MemoryStore
	keyRing *keyRing
	ctx     context.Context
	cancel  context.CancelFunc

	mu sync.Mutex
}

// DefaultConfig provides the default config for keymanager
func DefaultConfig() *Config {
	return &Config{
		ClusterName:      store.DefaultClusterName,
		Keylen:           DefaultKeyLen,
		RotationInterval: DefaultKeyRotationInterval,
		Subsystems:       []string{SubsystemGossip, SubsystemIPSec},
	}
}

// New creates an instance of keymanager with the given config
func New(store *store.MemoryStore, config *Config) *KeyManager {
	for _, subsys := range config.Subsystems {
		if subsys != SubsystemGossip && subsys != SubsystemIPSec {
			return nil
		}
	}
	return &KeyManager{
		config:  config,
		store:   store,
		keyRing: &keyRing{lClock: genSkew()},
	}
}

func (k *KeyManager) allocateKey(ctx context.Context, subsys string) *api.EncryptionKey {
	key := make([]byte, k.config.Keylen)

	_, err := cryptorand.Read(key)
	if err != nil {
		panic(errors.Wrap(err, "key generated failed"))
	}
	k.keyRing.lClock++

	return &api.EncryptionKey{
		Subsystem:   subsys,
		Algorithm:   subsysToAlgo[subsys],
		Key:         key,
		LamportTime: k.keyRing.lClock,
	}
}

func (k *KeyManager) updateKey(cluster *api.Cluster) error {
	return k.store.Update(func(tx store.Tx) error {
		cluster = store.GetCluster(tx, cluster.ID)
		if cluster == nil {
			return nil
		}
		cluster.EncryptionKeyLamportClock = k.keyRing.lClock
		cluster.NetworkBootstrapKeys = k.keyRing.keys
		return store.UpdateCluster(tx, cluster)
	})
}

func (k *KeyManager) rotateKey(ctx context.Context) error {
	var (
		clusters []*api.Cluster
		err      error
	)
	k.store.View(func(readTx store.ReadTx) {
		clusters, err = store.FindClusters(readTx, store.ByName(k.config.ClusterName))
	})

	if err != nil {
		log.G(ctx).Errorf("reading cluster config failed, %v", err)
		return err
	}

	cluster := clusters[0]
	if len(cluster.NetworkBootstrapKeys) == 0 {
		panic(errors.New("no key in the cluster config"))
	}

	subsysKeys := map[string][]*api.EncryptionKey{}
	for _, key := range k.keyRing.keys {
		subsysKeys[key.Subsystem] = append(subsysKeys[key.Subsystem], key)
	}
	k.keyRing.keys = []*api.EncryptionKey{}

	// We maintain the latest key and the one before in the key ring to allow
	// agents to communicate without disruption on key change.
	for subsys, keys := range subsysKeys {
		if len(keys) == keyringSize {
			min := 0
			for i, key := range keys[1:] {
				if key.LamportTime < keys[min].LamportTime {
					min = i
				}
			}
			keys = append(keys[0:min], keys[min+1:]...)
		}
		keys = append(keys, k.allocateKey(ctx, subsys))
		subsysKeys[subsys] = keys
	}

	for _, keys := range subsysKeys {
		k.keyRing.keys = append(k.keyRing.keys, keys...)
	}

	return k.updateKey(cluster)
}

// Run starts the keymanager, it doesn't return
func (k *KeyManager) Run(ctx context.Context) error {
	k.mu.Lock()
	ctx = log.WithModule(ctx, "keymanager")
	var (
		clusters []*api.Cluster
		err      error
	)
	k.store.View(func(readTx store.ReadTx) {
		clusters, err = store.FindClusters(readTx, store.ByName(k.config.ClusterName))
	})

	if err != nil {
		log.G(ctx).Errorf("reading cluster config failed, %v", err)
		k.mu.Unlock()
		return err
	}

	cluster := clusters[0]
	if len(cluster.NetworkBootstrapKeys) == 0 {
		for _, subsys := range k.config.Subsystems {
			for i := 0; i < keyringSize; i++ {
				k.keyRing.keys = append(k.keyRing.keys, k.allocateKey(ctx, subsys))
			}
		}
		if err := k.updateKey(cluster); err != nil {
			log.G(ctx).Errorf("store update failed %v", err)
		}
	} else {
		k.keyRing.lClock = cluster.EncryptionKeyLamportClock
		k.keyRing.keys = cluster.NetworkBootstrapKeys
	}

	ticker := time.NewTicker(k.config.RotationInterval)
	defer ticker.Stop()

	k.ctx, k.cancel = context.WithCancel(ctx)
	k.mu.Unlock()

	for {
		select {
		case <-ticker.C:
			k.rotateKey(ctx)
		case <-k.ctx.Done():
			return nil
		}
	}
}

// Stop stops the running instance of key manager
func (k *KeyManager) Stop() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.cancel == nil {
		return errors.New("keymanager is not started")
	}
	k.cancel()
	return nil
}

// genSkew generates a random uint64 number between 0 and 65535
func genSkew() uint64 {
	b := make([]byte, 2)
	if _, err := cryptorand.Read(b); err != nil {
		panic(err)
	}
	return uint64(binary.BigEndian.Uint16(b))
}
