package cniprovider

import (
	"context"
	"os"
	"runtime"

	cni "github.com/containerd/go-cni"
	"github.com/gofrs/flock"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/util/network"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

type Opt struct {
	Root       string
	ConfigPath string
	BinaryDir  string
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

	cniOptions = append(cniOptions, cni.WithConfFile(opt.ConfigPath))

	cniHandle, err := cni.New(cniOptions...)
	if err != nil {
		return nil, err
	}

	cp := &cniProvider{CNI: cniHandle, root: opt.Root}
	if err := cp.initNetwork(); err != nil {
		return nil, err
	}
	return cp, nil
}

type cniProvider struct {
	cni.CNI
	root string
}

func (c *cniProvider) initNetwork() error {
	if v := os.Getenv("BUILDKIT_CNI_INIT_LOCK_PATH"); v != "" {
		l := flock.New(v)
		if err := l.Lock(); err != nil {
			return err
		}
		defer l.Unlock()
	}
	ns, err := c.New()
	if err != nil {
		return err
	}
	return ns.Close()
}

func (c *cniProvider) New() (network.Namespace, error) {
	id := identity.NewID()
	nativeID, err := createNetNS(c, id)
	if err != nil {
		return nil, err
	}

	if _, err := c.CNI.Setup(context.TODO(), id, nativeID); err != nil {
		deleteNetNS(nativeID)
		return nil, errors.Wrap(err, "CNI setup error")
	}

	return &cniNS{nativeID: nativeID, id: id, handle: c.CNI}, nil
}

type cniNS struct {
	handle   cni.CNI
	id       string
	nativeID string
}

func (ns *cniNS) Set(s *specs.Spec) error {
	return setNetNS(s, ns.nativeID)
}

func (ns *cniNS) Close() error {
	err := ns.handle.Remove(context.TODO(), ns.id, ns.nativeID)
	if err1 := unmountNetNS(ns.nativeID); err1 != nil && err == nil {
		err = err1
	}
	if err1 := deleteNetNS(ns.nativeID); err1 != nil && err == nil {
		err = err1
	}
	return err
}
