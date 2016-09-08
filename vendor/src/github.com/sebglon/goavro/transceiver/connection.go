package transceiver

import (
	"io"
	"net"
	"strconv"
)

type HandShakeConn interface {
	io.ReadWriteCloser
	IsChecked() bool
	Checked(bool)
	GetConn() (net.Conn, error)
}

type Connection  struct {
	net.Conn
	checked bool
	bad bool
}


func NewConnection(config Config)  (*Connection, error) {

	conn := &Connection{}
	var err error
	switch config.Network {
	case "tcp":
		conn.Conn, err = net.DialTimeout(config.Network, config.Host+":"+strconv.Itoa(config.Port), config.Timeout)
	case "unix":
		conn.Conn, err = net.DialTimeout(config.Network, config.SocketPath, config.Timeout)
	default:
		err = net.UnknownNetworkError(config.Network)
	}

	return conn, err
}

func (c *Connection) Checked(check bool) {
	c.checked = check
}

func (c *Connection) IsChecked() bool{
	return c.checked
}

