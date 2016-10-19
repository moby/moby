package swarm

// Secret represents a secret.
type Secret struct {
	ID string
	Meta
	Spec       *SecretSpec `json:",omitempty"`
	Digest     string      `json:",omitempty"`
	SecretSize int64       `json:",omitempty"`
}

type SecretSpec struct {
	Annotations
	Data []byte `json",omitempty"`
}

type SecretReferenceMode int

const (
	SecretReferenceSystem SecretReferenceMode = 0
	SecretReferenceFile   SecretReferenceMode = 1
	SecretReferenceEnv    SecretReferenceMode = 2
)

type SecretReference struct {
	SecretID   string              `json:",omitempty"`
	Mode       SecretReferenceMode `json:",omitempty"`
	Target     string              `json:",omitempty"`
	SecretName string              `json:",omitempty"`
}
