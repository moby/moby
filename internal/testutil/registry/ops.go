package registry

import "io"

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

// WithStdout sets the stdout of the registry command to the passed in writer.
func WithStdout(w io.Writer) func(c *Config) {
	return func(c *Config) {
		c.stdout = w
	}
}

// WithStderr sets the stdout of the registry command to the passed in writer.
func WithStderr(w io.Writer) func(c *Config) {
	return func(c *Config) {
		c.stderr = w
	}
}
