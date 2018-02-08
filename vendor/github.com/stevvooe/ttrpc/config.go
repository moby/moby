package ttrpc

import "github.com/pkg/errors"

type serverConfig struct {
	handshaker Handshaker
}

type ServerOpt func(*serverConfig) error

// WithServerHandshaker can be passed to NewServer to ensure that the
// handshaker is called before every connection attempt.
//
// Only one handshaker is allowed per server.
func WithServerHandshaker(handshaker Handshaker) ServerOpt {
	return func(c *serverConfig) error {
		if c.handshaker != nil {
			return errors.New("only one handshaker allowed per server")
		}
		c.handshaker = handshaker
		return nil
	}
}
