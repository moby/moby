package cniprovider

import (
	"context"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	cni "github.com/containerd/go-cni"
	"github.com/gofrs/flock"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/network"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/trace"
)

const aboveTargetGracePeriod = 5 * time.Minute

type Opt struct {
	Root       string
	ConfigPath string
	BinaryDir  string
	PoolSize   int
}

func New(opt Opt) (network.Provider, error) {
	if _, err := os.Stat(opt.ConfigPath); err != nil {
		return nil, errors.Wrapf(err, "failed to read cni config %q", opt.ConfigPath)
	}
	if _, err := os.Stat(opt.BinaryDir); err != nil {
		return nil, errors.Wrapf(err, "failed to read cni binary dir %q", opt.BinaryDir)
	}

	cniOptions := []cni.Opt{cni.WithPluginDir([]string{opt.BinaryDir}), cni.WithInterfacePrefix("eth")}

	// Windows doesn't use CNI for loopback.
	if runtime.GOOS != "windows" {
		cniOptions = append([]cni.Opt{cni.WithMinNetworkCount(2)}, cniOptions...)
		cniOptions = append(cniOptions, cni.WithLoNetwork)
	}

	if strings.HasSuffix(opt.ConfigPath, ".conflist") {
		cniOptions = append(cniOptions, cni.WithConfListFile(opt.ConfigPath))
	} else {
		cniOptions = append(cniOptions, cni.WithConfFile(opt.ConfigPath))
	}

	cniHandle, err := cni.New(cniOptions...)
	if err != nil {
		return nil, err
	}

	cp := &cniProvider{
		CNI:  cniHandle,
		root: opt.Root,
	}
	cleanOldNamespaces(cp)

	cp.nsPool = &cniPool{targetSize: opt.PoolSize, provider: cp}
	if err := cp.initNetwork(); err != nil {
		return nil, err
	}
	go cp.nsPool.fillPool(context.TODO())
	return cp, nil
}

type cniProvider struct {
	cni.CNI
	root   string
	nsPool *cniPool
}

func (c *cniProvider) initNetwork() error {
	if v := os.Getenv("BUILDKIT_CNI_INIT_LOCK_PATH"); v != "" {
		l := flock.New(v)
		if err := l.Lock(); err != nil {
			return err
		}
		defer l.Unlock()
	}
	ns, err := c.New(context.TODO(), "")
	if err != nil {
		return err
	}
	return ns.Close()
}

func (c *cniProvider) Close() error {
	c.nsPool.close()
	return nil
}

type cniPool struct {
	provider   *cniProvider
	mu         sync.Mutex
	targetSize int
	actualSize int
	// LIFO: Ordered least recently used to most recently used
	available []*cniNS
	closed    bool
}

func (pool *cniPool) close() {
	bklog.L.Debugf("cleaning up cni pool")

	pool.mu.Lock()
	pool.closed = true
	defer pool.mu.Unlock()
	for len(pool.available) > 0 {
		_ = pool.available[0].release()
		pool.available = pool.available[1:]
		pool.actualSize--
	}
}

func (pool *cniPool) fillPool(ctx context.Context) {
	for {
		pool.mu.Lock()
		if pool.closed {
			pool.mu.Unlock()
			return
		}
		actualSize := pool.actualSize
		pool.mu.Unlock()
		if actualSize >= pool.targetSize {
			return
		}
		ns, err := pool.getNew(ctx)
		if err != nil {
			bklog.G(ctx).Errorf("failed to create new network namespace while prefilling pool: %s", err)
			return
		}
		pool.put(ns)
	}
}

func (pool *cniPool) get(ctx context.Context) (*cniNS, error) {
	pool.mu.Lock()
	if len(pool.available) > 0 {
		ns := pool.available[len(pool.available)-1]
		pool.available = pool.available[:len(pool.available)-1]
		pool.mu.Unlock()
		trace.SpanFromContext(ctx).AddEvent("returning network namespace from pool")
		bklog.G(ctx).Debugf("returning network namespace %s from pool", ns.id)
		return ns, nil
	}
	pool.mu.Unlock()

	return pool.getNew(ctx)
}

func (pool *cniPool) getNew(ctx context.Context) (*cniNS, error) {
	ns, err := pool.provider.newNS(ctx, "")
	if err != nil {
		return nil, err
	}
	ns.pool = pool

	pool.mu.Lock()
	defer pool.mu.Unlock()
	if pool.closed {
		return nil, errors.New("cni pool is closed")
	}
	pool.actualSize++
	return ns, nil
}

func (pool *cniPool) put(ns *cniNS) {
	putTime := time.Now()
	ns.lastUsed = putTime

	pool.mu.Lock()
	defer pool.mu.Unlock()
	if pool.closed {
		_ = ns.release()
		return
	}
	pool.available = append(pool.available, ns)
	actualSize := pool.actualSize

	if actualSize > pool.targetSize {
		// We have more network namespaces than our target number, so
		// schedule a shrinking pass.
		time.AfterFunc(aboveTargetGracePeriod, pool.cleanupToTargetSize)
	}
}

func (pool *cniPool) cleanupToTargetSize() {
	var toRelease []*cniNS
	defer func() {
		for _, poolNS := range toRelease {
			_ = poolNS.release()
		}
	}()

	pool.mu.Lock()
	defer pool.mu.Unlock()
	for pool.actualSize > pool.targetSize &&
		len(pool.available) > 0 &&
		time.Since(pool.available[0].lastUsed) >= aboveTargetGracePeriod {
		bklog.L.Debugf("releasing network namespace %s since it was last used at %s", pool.available[0].id, pool.available[0].lastUsed)
		toRelease = append(toRelease, pool.available[0])
		pool.available = pool.available[1:]
		pool.actualSize--
	}
}

func (c *cniProvider) New(ctx context.Context, hostname string) (network.Namespace, error) {
	// We can't use the pool for namespaces that need a custom hostname.
	// We also avoid using it on windows because we don't have a cleanup
	// mechanism for Windows yet.
	if hostname == "" || runtime.GOOS == "windows" {
		return c.nsPool.get(ctx)
	}
	return c.newNS(ctx, hostname)
}

func (c *cniProvider) newNS(ctx context.Context, hostname string) (*cniNS, error) {
	id := identity.NewID()
	trace.SpanFromContext(ctx).AddEvent("creating new network namespace")
	bklog.G(ctx).Debugf("creating new network namespace %s", id)
	nativeID, err := createNetNS(c, id)
	if err != nil {
		return nil, err
	}
	trace.SpanFromContext(ctx).AddEvent("finished creating network namespace")
	bklog.G(ctx).Debugf("finished creating network namespace %s", id)

	nsOpts := []cni.NamespaceOpts{}

	if hostname != "" {
		nsOpts = append(nsOpts,
			// NB: K8S_POD_NAME is a semi-well-known arg set by k8s and podman and
			// leveraged by the dnsname CNI plugin. a more generic name would be nice.
			cni.WithArgs("K8S_POD_NAME", hostname),

			// must be set for plugins that don't understand K8S_POD_NAME
			cni.WithArgs("IgnoreUnknown", "1"))
	}

	if _, err := c.CNI.Setup(context.TODO(), id, nativeID, nsOpts...); err != nil {
		deleteNetNS(nativeID)
		return nil, errors.Wrap(err, "CNI setup error")
	}
	trace.SpanFromContext(ctx).AddEvent("finished setting up network namespace")
	bklog.G(ctx).Debugf("finished setting up network namespace %s", id)

	return &cniNS{
		nativeID: nativeID,
		id:       id,
		handle:   c.CNI,
		opts:     nsOpts,
	}, nil
}

type cniNS struct {
	pool     *cniPool
	handle   cni.CNI
	id       string
	nativeID string
	opts     []cni.NamespaceOpts
	lastUsed time.Time
}

func (ns *cniNS) Set(s *specs.Spec) error {
	return setNetNS(s, ns.nativeID)
}

func (ns *cniNS) Close() error {
	if ns.pool == nil {
		return ns.release()
	}
	ns.pool.put(ns)
	return nil
}

func (ns *cniNS) release() error {
	bklog.L.Debugf("releasing cni network namespace %s", ns.id)
	err := ns.handle.Remove(context.TODO(), ns.id, ns.nativeID, ns.opts...)
	if err1 := unmountNetNS(ns.nativeID); err1 != nil && err == nil {
		err = err1
	}
	if err1 := deleteNetNS(ns.nativeID); err1 != nil && err == nil {
		err = err1
	}
	return err
}
