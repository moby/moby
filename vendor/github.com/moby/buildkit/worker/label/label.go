package label

// Pre-defined label keys
const (
	prefix = "org.mobyproject.buildkit.worker."

	Executor            = prefix + "executor"    // "oci" or "containerd"
	Snapshotter         = prefix + "snapshotter" // containerd snapshotter name ("overlay", "native", ...)
	Hostname            = prefix + "hostname"
	Network             = prefix + "network" // "cni" or "host"
	ApparmorProfile     = prefix + "apparmor.profile"
	SELinuxEnabled      = prefix + "selinux.enabled"      // "true" or "false"
	OCIProcessMode      = prefix + "oci.process-mode"     // OCI worker: process mode ("sandbox", "no-sandbox")
	ContainerdUUID      = prefix + "containerd.uuid"      // containerd worker: containerd UUID
	ContainerdNamespace = prefix + "containerd.namespace" // containerd worker: containerd namespace
)
