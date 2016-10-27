package swarm

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

type SecretReferenceMode int

const (
	SecretReferenceSystem SecretReferenceMode = 0
	SecretReferenceFile   SecretReferenceMode = 1
	SecretReferenceEnv    SecretReferenceMode = 2
)

type SecretReference struct {
	SecretID   string
	Mode       SecretReferenceMode
	Target     string
	SecretName string
}
