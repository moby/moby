//go:build !linux && !windows
// +build !linux,!windows

package cniprovider

import (
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

func createNetNS(c *cniProvider, id string) (string, error) {
	return "", errors.New("creating netns for cni not supported")
}

func setNetNS(s *specs.Spec, nativeID string) error {
	return errors.New("enabling netns for cni not supported")
}

func unmountNetNS(nativeID string) error {
	return errors.New("unmounting netns for cni not supported")
}

func deleteNetNS(nativeID string) error {
	return errors.New("deleting netns for cni not supported")
}

func cleanOldNamespaces(_ *cniProvider) {
}
