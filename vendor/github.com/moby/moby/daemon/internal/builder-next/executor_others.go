//go:build !linux && !windows

package buildkit

func newExecutor(_, _ string, _ *libnetwork.Controller, _ *oci.DNSConfig, _ bool, _ user.IdentityMapping, _ string, _ *cdidevices.Manager, _, _ string) (executor.Executor, error) {
	return &stubExecutor{}, nil
}
