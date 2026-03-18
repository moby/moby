package config

type RegistryConfig struct {
	Mirrors      []string     `toml:"mirrors"`
	PlainHTTP    *bool        `toml:"http"`
	Insecure     *bool        `toml:"insecure"`
	RootCAs      []string     `toml:"ca"`
	KeyPairs     []TLSKeyPair `toml:"keypair"`
	TLSConfigDir []string     `toml:"tlsconfigdir"`
}

type TLSKeyPair struct {
	Key         string `toml:"key"`
	Certificate string `toml:"cert"`
}
