package swarm

type SecurityConfig struct {
	Userns         string `json:",omitempty"`
	CredentialSpec string `json:",omitempty"`
}
