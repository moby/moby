//go:build windows
// +build windows

package cniprovider

import (
	"github.com/Microsoft/hcsshim/hcn"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

func createNetNS(_ *cniProvider, id string) (string, error) {
	nsTemplate := hcn.NewNamespace(hcn.NamespaceTypeGuest)
	ns, err := nsTemplate.Create()
	if err != nil {
		return "", errors.Wrapf(err, "HostComputeNamespace.Create failed for %s", nsTemplate.Id)
	}

	return ns.Id, nil
}

func setNetNS(s *specs.Spec, nativeID string) error {
	// Containerd doesn't have a wrapper for this. Code based on oci.WithLinuxNamespace and
	// https://github.com/opencontainers/runtime-tools/blob/07406c5828aaf93f60d2aad770312d736811a276/generate/generate.go#L1810-L1814
	if s.Windows == nil {
		s.Windows = &specs.Windows{}
	}
	if s.Windows.Network == nil {
		s.Windows.Network = &specs.WindowsNetwork{}
	}

	s.Windows.Network.NetworkNamespace = nativeID

	return nil
}

func unmountNetNS(nativeID string) error {
	// We don't need to unmount the NS.
	return nil
}

func deleteNetNS(nativeID string) error {
	ns, err := hcn.GetNamespaceByID(nativeID)
	if err != nil {
		return errors.Wrapf(err, "failed to get namespace %s", nativeID)
	}

	return ns.Delete()
}
