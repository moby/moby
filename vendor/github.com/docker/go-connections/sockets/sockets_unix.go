//go:build !windows

package sockets

func configureNpipeTransport(any, string) error {
	return ErrProtocolNotAvailable
}
