// +build go1.8

package transport

import "crypto/tls"

// TLSConfigClone returns a clone of tls.Config. This function is provided for
// compatibility for go1.7 that doesn't include this method in stdlib.
func TLSConfigClone(c *tls.Config) *tls.Config {
	return c.Clone()
}
