package swarm

import "os"

// Secret represents a secret.
type Secret struct {
	ID string
	Meta
	Spec       SecretSpec
	Digest     string
	SecretSize int64
}

// SecretSpec represents a secret specification from a secret in swarm
type SecretSpec struct {
	Annotations
	Data []byte `json:",omitempty"`
}

// SecretReferenceFileTarget is a file target in a secret reference
type SecretReferenceFileTarget struct {
	Name string
	UID  string
	GID  string
	Mode os.FileMode
}

// SecretReference is a reference to a secret in swarm
type SecretReference struct {
	SecretID   string
	SecretName string
	Target     *SecretReferenceFileTarget
}
