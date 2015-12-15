package types

// AuthConfig contains authorization information for connecting to a Registry
type AuthConfig struct {
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	Auth          string `json:"auth"`
	ServerAddress string `json:"serveraddress,omitempty"`
	RegistryToken string `json:"registrytoken,omitempty"`
}
