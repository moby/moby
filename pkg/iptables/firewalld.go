package iptables

import (
	"fmt"
	"strings"

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
	firewalldRunning bool      // is Firewalld service running
	onReloaded       []*func() // callbacks when Firewalld has been reloaded
)

func FirewalldInit() error {
	var err error

	if connection, err = newConnection(); err != nil {
		return fmt.Errorf("Failed to connect to D-Bus system bus: %v", err)
	}
	if connection != nil {
		go signalHandler()
	}

	firewalldRunning = checkRunning()
	return nil
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

	rule := fmt.Sprintf("type='signal',path='%s',interface='%s',sender='%s',member='Reloaded'",
		dbusPath, dbusInterface, dbusInterface)
	c.sysconn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule)

	rule = fmt.Sprintf("type='signal',interface='org.freedesktop.DBus',member='NameOwnerChanged',path='/org/freedesktop/DBus',sender='org.freedesktop.DBus',arg0='%s'",
		dbusInterface)
	c.sysconn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule)

	c.signal = make(chan *dbus.Signal, 10)
	c.sysconn.Signal(c.signal)

	return nil
}

func signalHandler() {
	for signal := range connection.signal {
		if strings.Contains(signal.Name, "NameOwnerChanged") {
			firewalldRunning = checkRunning()
			dbusConnectionChanged(signal.Body)
		} else if strings.Contains(signal.Name, "Reloaded") {
			reloaded()
		}
	}
}

func dbusConnectionChanged(args []interface{}) {
	name := args[0].(string)
	old_owner := args[1].(string)
	new_owner := args[2].(string)

	if name != dbusInterface {
		return
	}

	if len(new_owner) > 0 {
		connectionEstablished()
	} else if len(old_owner) > 0 {
		connectionLost()
	}
}

func connectionEstablished() {
	reloaded()
}

func connectionLost() {
	// Doesn't do anything for now. Libvirt also doesn't react to this.
}

// call all callbacks
func reloaded() {
	for _, pf := range onReloaded {
		(*pf)()
	}
}

// add callback
func OnReloaded(callback func()) {
	for _, pf := range onReloaded {
		if pf == &callback {
			return
		}
	}
	onReloaded = append(onReloaded, &callback)
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
	return false
}

// Firewalld's passthrough method simply passes args through to iptables/ip6tables
func Passthrough(ipv IPV, args ...string) ([]byte, error) {
	var output string
	logrus.Debugf("Firewalld passthrough: %s, %s", ipv, args)
	if err := connection.sysobj.Call(dbusInterface+".direct.passthrough", 0, ipv, args).Store(&output); err != nil {
		return nil, err
	}
	return []byte(output), nil
}
