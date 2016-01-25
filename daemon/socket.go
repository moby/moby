package daemon

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/stringid"
)

func (daemon *Daemon) ContainerForwardSocket(name, containerPath string, remoteSock io.ReadWriter) error {
	c, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}
	if !c.IsRunning() {
		return fmt.Errorf("container must be running")
	}
	return daemon.containerForwardSocket(c, remoteSock)
}

func (daemon *Daemon) containerForwardSocket(c *container.Container, remoteSock io.ReadWriter) error {
	os.MkdirAll("/run/docker/sockets", 0755)
	sockID := stringid.GenerateNonCryptoID()
	sockPath := filepath.Join("/run/docker/sockets", sockID+".sock")
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		return err
	}
	defer l.Close()
	defer os.RemoveAll(sockPath)

	proxyStreams := func(local io.ReadWriteCloser, remote io.ReadWriter) {
		logrus.Debugf("proxying socket %s", sockPath)
		chErr := make(chan error, 2)
		go func() {
			_, err := io.Copy(local, remote)
			chErr <- err
		}()
		go func() {
			_, err := io.Copy(remote, local)
			chErr <- err
		}()
		if err := <-chErr; err != nil {
			logrus.Warnf("Error while forwarding socket conn: %v", err)
		}
		local.Close()
		<-chErr
	}

	chContainerStop := make(chan struct{})
	go func() {
		c.WaitStop(-1 * time.Second)
		close(chContainerStop)
	}()

	chDone := make(chan struct{})
	go func() {
		defer close(chDone)

		chConn := make(chan net.Conn)
		go func() {
			for {
				conn, err := l.Accept()
				if err != nil {
					logrus.Debugf("Error while receiving connection on forwarded socket for %s: %v", c.ID, err)
					return
				}
				chConn <- conn
			}
		}()

		for {
			select {
			case <-chContainerStop:
				return
			case conn := <-chConn:
				go proxyStreams(conn, remoteSock)
			}
		}
	}()

	<-chDone
	return nil
}
