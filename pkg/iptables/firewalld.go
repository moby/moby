package iptables

import (
	"github.com/Sirupsen/logrus"
	"github.com/godbus/dbus"
)

type IPV string

const (
	Iptables  IPV = "ipv4"
	Ip6tables IPV = "ipv6"
	Ebtables  IPV = "eb"
)
const (
	dbusInterface = "org.fedoraproject.FirewallD1"
	dbusPath      = "/org/fedoraproject/FirewallD1"
)

// Conn is a connection to firewalld dbus endpoint.
type Conn struct {
	sysconn *dbus.Conn
	sysobj  *dbus.Object
	signal  chan *dbus.Signal
}

var (
	connection       *Conn
	firewalldRunning bool // is Firewalld service running
)

func FirewalldInit() {
	var err error

	connection, err = newConnection()

	if err != nil {
		logrus.Errorf("Failed to connect to D-Bus system bus: %s", err)
	}

	firewalldRunning = checkRunning()
}

// New() establishes a connection to the system bus.
func newConnection() (*Conn, error) {
	c := new(Conn)
	if err := c.initConnection(); err != nil {
		return nil, err
	}

	return c, nil
}

// Innitialize D-Bus connection.
func (c *Conn) initConnection() error {
	var err error

	c.sysconn, err = dbus.SystemBus()
	if err != nil {
		return err
	}

	// This never fails, even if the service is not running atm.
	c.sysobj = c.sysconn.Object(dbusInterface, dbus.ObjectPath(dbusPath))

	return nil
}

// Call some remote method to see whether the service is actually running.
func checkRunning() bool {
	var zone string
	var err error

	if connection != nil {
		err = connection.sysobj.Call(dbusInterface+".getDefaultZone", 0).Store(&zone)
		logrus.Infof("Firewalld running: %t", err == nil)
		return err == nil
	}
	logrus.Info("Firewalld not running")
	return false
}

// Firewalld's passthrough method simply passes args through to iptables/ip6tables
func Passthrough(ipv IPV, args ...string) ([]byte, error) {
	var output string

	logrus.Debugf("Firewalld passthrough: %s, %s", ipv, args)
	err := connection.sysobj.Call(dbusInterface+".direct.passthrough", 0, ipv, args).Store(&output)
	if output != "" {
		logrus.Debugf("passthrough output: %s", output)
	}

	return []byte(output), err
}
