package cniprovider

import (
	"context"
	"os"
	"runtime"
	"strings"

	cni "github.com/containerd/go-cni"
	"github.com/gofrs/flock"
	resourcestypes "github.com/moby/buildkit/executor/resources/types"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/network"
	"github.com/moby/buildkit/util/network/netpool"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/trace"
)

type Opt struct {
	Root         string
	ConfigPath   string
	BinaryDir    string
	PoolSize     int
	BridgeName   string
	BridgeSubnet string
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

	var cniHandle cni.CNI
	fn := func(_ context.Context) error {
		var err error
		cniHandle, err = cni.New(cniOptions...)
		return err
	}
	if err := withDetachedNetNSIfAny(context.TODO(), fn); err != nil {
		return nil, err
	}

	cp := &cniProvider{
		CNI:  cniHandle,
		root: opt.Root,
	}
	cleanOldNamespaces(cp)

	cp.nsPool = newCNIPool(cp, opt.PoolSize)
	if err := cp.initNetwork(true); err != nil {
		return nil, err
	}
	go cp.nsPool.Fill(context.TODO())
	return cp, nil
}

type cniProvider struct {
	cni.CNI
	root    string
	nsPool  *netpool.Pool[*cniNS]
	release func() error
}

func (c *cniProvider) initNetwork(lock bool) error {
	if lock {
		unlock, err := initLock()
		if err != nil {
			return err
		}
		defer unlock()
	}
	ns, err := c.New(context.TODO(), "", network.NamespaceOptions{})
	if err != nil {
		return err
	}
	return ns.Close()
}

func (c *cniProvider) Close() error {
	var err error
	if e := c.nsPool.Close(); e != nil {
		err = e
	}
	if c.release != nil {
		if e := c.release(); e != nil && err == nil {
			err = e
		}
	}
	return err
}

func initLock() (func() error, error) {
	if v := os.Getenv("BUILDKIT_CNI_INIT_LOCK_PATH"); v != "" {
		l := flock.New(v)
		if err := l.Lock(); err != nil {
			return nil, err
		}
		return l.Unlock, nil
	}
	return func() error { return nil }, nil
}

func newCNIPool(c *cniProvider, targetSize int) *netpool.Pool[*cniNS] {
	var pool *netpool.Pool[*cniNS]
	pool = netpool.New(netpool.Opt[*cniNS]{
		Name:       "cni network namespace",
		TargetSize: targetSize,
		New: func(ctx context.Context) (*cniNS, error) {
			var ns *cniNS
			fn := func(ctx context.Context) error {
				var err error
				ns, err = c.newNS(ctx, "")
				return err
			}
			if err := withDetachedNetNSIfAny(ctx, fn); err != nil {
				return nil, err
			}
			ns.pool = pool
			return ns, nil
		},
		Release: func(ns *cniNS) error {
			return ns.release()
		},
	})
	return pool
}

func (c *cniProvider) New(ctx context.Context, hostname string, _ network.NamespaceOptions) (network.Namespace, error) {
	// We can't use the pool for namespaces that need a custom hostname.
	// We also avoid using it on windows because we don't have a cleanup
	// mechanism for Windows yet.
	if hostname == "" || runtime.GOOS == "windows" {
		return c.nsPool.Get(ctx)
	}
	var res network.Namespace
	fn := func(ctx context.Context) error {
		var err error
		res, err = c.newNS(ctx, hostname)
		return err
	}
	if err := withDetachedNetNSIfAny(ctx, fn); err != nil {
		return nil, err
	}
	return res, nil
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

	var cniRes *cni.Result
	if ctx.Value(contextKeyDetachedNetNS) == nil {
		cniRes, err = c.Setup(context.TODO(), id, nativeID, nsOpts...)
	} else {
		// Parallel Setup cannot be used, apparently due to the goroutine issue with setns
		cniRes, err = c.SetupSerially(context.TODO(), id, nativeID, nsOpts...)
	}
	if err != nil {
		deleteNetNS(nativeID)
		return nil, errors.Wrap(err, "CNI setup error")
	}
	trace.SpanFromContext(ctx).AddEvent("finished setting up network namespace")
	bklog.G(ctx).Debugf("finished setting up network namespace %s", id)

	vethName := ""
	for k := range cniRes.Interfaces {
		if strings.HasPrefix(k, "veth") {
			if vethName != "" {
				// invalid config
				vethName = ""
				break
			}
			vethName = k
		}
	}

	ns := &cniNS{
		nativeID: nativeID,
		id:       id,
		handle:   c.CNI,
		opts:     nsOpts,
		vethName: vethName,
	}

	if ns.vethName != "" {
		sample, err := ns.sample()
		if err == nil && sample != nil {
			ns.canSample = true
			ns.offsetSample = sample
		}
	}

	return ns, nil
}

type cniNS struct {
	pool         *netpool.Pool[*cniNS]
	handle       cni.CNI
	id           string
	nativeID     string
	opts         []cni.NamespaceOpts
	vethName     string
	canSample    bool
	offsetSample *resourcestypes.NetworkSample
	prevSample   *resourcestypes.NetworkSample
}

func (ns *cniNS) Set(s *specs.Spec) error {
	return setNetNS(s, ns.nativeID)
}

func (ns *cniNS) Close() error {
	if ns.prevSample != nil {
		ns.offsetSample = ns.prevSample
	}
	if ns.pool == nil {
		return ns.release()
	}
	ns.pool.Put(ns)
	return nil
}

func (ns *cniNS) Sample() (*resourcestypes.NetworkSample, error) {
	if !ns.canSample {
		return nil, nil
	}
	var s *resourcestypes.NetworkSample
	fn := func(_ context.Context) error {
		var err error
		s, err = ns.sample()
		return err
	}
	err := withDetachedNetNSIfAny(context.TODO(), fn)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, nil
	}
	if ns.offsetSample != nil {
		s.TxBytes -= ns.offsetSample.TxBytes
		s.RxBytes -= ns.offsetSample.RxBytes
		s.TxPackets -= ns.offsetSample.TxPackets
		s.RxPackets -= ns.offsetSample.RxPackets
		s.TxErrors -= ns.offsetSample.TxErrors
		s.RxErrors -= ns.offsetSample.RxErrors
		s.TxDropped -= ns.offsetSample.TxDropped
		s.RxDropped -= ns.offsetSample.RxDropped
	}
	return s, nil
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

type contextKeyT string

// contextKeyDetachedNetNS is associated with a string value that denotes RootlessKit's detached-netns.
var contextKeyDetachedNetNS = contextKeyT("buildkit/util/network/cniprovider/detached-netns")
