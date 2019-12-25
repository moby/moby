package oci

// ProcMode configures PID namespaces
type ProcessMode int

const (
	// ProcessSandbox unshares pidns and mount procfs.
	ProcessSandbox ProcessMode = iota
	// NoProcessSandbox uses host pidns and bind-mount procfs.
	// Note that NoProcessSandbox allows build containers to kill (and potentially ptrace) an arbitrary process in the BuildKit host namespace.
	// NoProcessSandbox should be enabled only when the BuildKit is running in a container as an unprivileged user.
	NoProcessSandbox
)
