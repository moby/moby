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

type SecretSpec struct {
	Annotations
	Data []byte
}

type SecretReferenceFileTarget struct {
	Name string
	UID  string
	GID  string
	Mode os.FileMode
}

type SecretReference struct {
	SecretID   string
	SecretName string
	Target     SecretReferenceFileTarget
}

// SecretRequestSpec is a type for requesting secrets
type SecretRequestSpec struct {
	Source string
	Target string
	UID    string
	GID    string
	Mode   os.FileMode
}
