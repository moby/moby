package registry

import (
	"github.com/docker/docker/internal/test/certutil"
	"gotest.tools/icmd"
)

// Schema1 sets the registry to serve v1 api
func Schema1(c *Config) {
	c.schema1 = true
}

// Htpasswd sets the auth method with htpasswd
func Htpasswd(c *Config) {
	c.auth = "htpasswd"
}

// Token sets the auth method to token, with the specified token url
func Token(tokenURL string) func(*Config) {
	return func(c *Config) {
		c.auth = "token"
		c.tokenURL = tokenURL
	}
}

// URL sets the registry url
func URL(registryURL string) func(*Config) {
	return func(c *Config) {
		c.registryURL = registryURL
	}
}

// Exec uses the provided function to create the registry command
func Exec(f func(command string, arg ...string) icmd.Cmd) func(*Config) {
	return func(c *Config) {
		c.exec = f
	}
}

// TLS specifies the TLS certificates to use for this registry
func TLS(cfg certutil.TLSConfig) func(*Config) {
	return func(c *Config) {
		c.tls = cfg
	}
}
