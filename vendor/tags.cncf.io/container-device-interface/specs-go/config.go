package specs

import "os"

// CurrentVersion is the current version of the Spec.
const CurrentVersion = "0.7.0"

// Spec is the base configuration for CDI
type Spec struct {
	Version string `json:"cdiVersion"`
	Kind    string `json:"kind"`
	// Annotations add meta information per CDI spec. Note these are CDI-specific and do not affect container metadata.
	Annotations    map[string]string `json:"annotations,omitempty"`
	Devices        []Device          `json:"devices"`
	ContainerEdits ContainerEdits    `json:"containerEdits,omitempty"`
}

// Device is a "Device" a container runtime can add to a container
type Device struct {
	Name string `json:"name"`
	// Annotations add meta information per device. Note these are CDI-specific and do not affect container metadata.
	Annotations    map[string]string `json:"annotations,omitempty"`
	ContainerEdits ContainerEdits    `json:"containerEdits"`
}

// ContainerEdits are edits a container runtime must make to the OCI spec to expose the device.
type ContainerEdits struct {
	Env            []string      `json:"env,omitempty"`
	DeviceNodes    []*DeviceNode `json:"deviceNodes,omitempty"`
	Hooks          []*Hook       `json:"hooks,omitempty"`
	Mounts         []*Mount      `json:"mounts,omitempty"`
	IntelRdt       *IntelRdt     `json:"intelRdt,omitempty"`
	AdditionalGIDs []uint32      `json:"additionalGids,omitempty"`
}

// DeviceNode represents a device node that needs to be added to the OCI spec.
type DeviceNode struct {
	Path        string       `json:"path"`
	HostPath    string       `json:"hostPath,omitempty"`
	Type        string       `json:"type,omitempty"`
	Major       int64        `json:"major,omitempty"`
	Minor       int64        `json:"minor,omitempty"`
	FileMode    *os.FileMode `json:"fileMode,omitempty"`
	Permissions string       `json:"permissions,omitempty"`
	UID         *uint32      `json:"uid,omitempty"`
	GID         *uint32      `json:"gid,omitempty"`
}

// Mount represents a mount that needs to be added to the OCI spec.
type Mount struct {
	HostPath      string   `json:"hostPath"`
	ContainerPath string   `json:"containerPath"`
	Options       []string `json:"options,omitempty"`
	Type          string   `json:"type,omitempty"`
}

// Hook represents a hook that needs to be added to the OCI spec.
type Hook struct {
	HookName string   `json:"hookName"`
	Path     string   `json:"path"`
	Args     []string `json:"args,omitempty"`
	Env      []string `json:"env,omitempty"`
	Timeout  *int     `json:"timeout,omitempty"`
}

// IntelRdt describes the Linux IntelRdt parameters to set in the OCI spec.
type IntelRdt struct {
	ClosID        string `json:"closID,omitempty"`
	L3CacheSchema string `json:"l3CacheSchema,omitempty"`
	MemBwSchema   string `json:"memBwSchema,omitempty"`
	EnableCMT     bool   `json:"enableCMT,omitempty"`
	EnableMBM     bool   `json:"enableMBM,omitempty"`
}
