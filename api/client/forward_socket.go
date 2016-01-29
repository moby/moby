package client

import (
	"io"
	"net"
	"net/url"
	"strings"
)

func (cli *DockerCli) forwardSocket(id, socketURL string, stop chan struct{}) error {
	u, err := url.Parse(socketURL)
	if err != nil {
		return err
	}
	localSock, err := net.Dial(u.Scheme, strings.TrimPrefix(socketURL, u.Scheme+"://"))
	if err != nil {
		return err
	}
	defer localSock.Close()

	hijack, err := cli.client.ContainerForwardSocket(id)
	if err != nil {
		return err
	}
	defer hijack.Conn.Close()
	chErr := make(chan error, 2)
	go func() {
		_, err := io.Copy(hijack.Conn, localSock)
		chErr <- err
	}()

	go func() {
		_, err := io.Copy(localSock, hijack.Conn)
		chErr <- err
	}()

	select {
	case err = <-chErr:
	case <-stop:
	}
	hijack.Conn.Close()
	<-chErr

	return err
}
