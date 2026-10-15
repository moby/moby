package eventstream

import "fmt"

// RequireBidiHTTP2 returns an error if the response protocol is not at least
// HTTP/2, which is required for bidirectional event streams.
func (c *Codec) RequireBidiHTTP2(proto string, protoMajor int) error {
	if protoMajor < 2 {
		return fmt.Errorf("operation requires minimum HTTP protocol of HTTP/2, but was %s", proto)
	}
	return nil
}
