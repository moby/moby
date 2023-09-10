//go:build linux && !no_systemd

package iptables

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/pkg/rootless"
	dbus "github.com/godbus/dbus/v5"
)

// IPV defines the table string
type IPV string

const (
	// Iptables point ipv4 table
	Iptables IPV = "ipv4"
	// IP6Tables point to ipv6 table
	IP6Tables IPV = "ipv6"
)

const (
	dbusInterface  = "org.fedoraproject.FirewallD1"
	dbusPath       = "/org/fedoraproject/FirewallD1"
	dbusConfigPath = "/org/fedoraproject/FirewallD1/config"
	dockerZone     = "docker"
)

// Conn is a connection to firewalld dbus endpoint.
type Conn struct {
	sysconn    *dbus.Conn
	sysObj     dbus.BusObject
	sysConfObj dbus.BusObject
	signal     chan *dbus.Signal
}

var (
	connection *Conn

	firewalldRunning bool      // is Firewalld service running
	onReloaded       []*func() // callbacks when Firewalld has been reloaded
)

// firewalldInit initializes firewalld management code.
func firewalldInit() error {
	var err error

	if connection, err = newConnection(); err != nil {
		return fmt.Errorf("Failed to connect to D-Bus system bus: %v", err)
	}
	firewalldRunning = checkRunning()
	if !firewalldRunning {
		connection.sysconn.Close()
		connection = nil
	}
	if connection != nil {
		go signalHandler()
		if err := setupDockerZone(); err != nil {
			return err
		}
	}

	return nil
}

// newConnection establishes a connection to the system bus.
func newConnection() (*Conn, error) {
	c := &Conn{}

	var err error
	c.sysconn, err = dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	// This never fails, even if the service is not running atm.
	c.sysObj = c.sysconn.Object(dbusInterface, dbusPath)
	c.sysConfObj = c.sysconn.Object(dbusInterface, dbusConfigPath)

	rule := fmt.Sprintf("type='signal',path='%s',interface='%s',sender='%s',member='Reloaded'", dbusPath, dbusInterface, dbusInterface)
	c.sysconn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule)

	rule = fmt.Sprintf("type='signal',interface='org.freedesktop.DBus',member='NameOwnerChanged',path='/org/freedesktop/DBus',sender='org.freedesktop.DBus',arg0='%s'", dbusInterface)
	c.sysconn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule)

	c.signal = make(chan *dbus.Signal, 10)
	c.sysconn.Signal(c.signal)
	return c, nil
}

func signalHandler() {
	for signal := range connection.signal {
		switch {
		case strings.Contains(signal.Name, "NameOwnerChanged"):
			firewalldRunning = checkRunning()
			dbusConnectionChanged(signal.Body)

		case strings.Contains(signal.Name, "Reloaded"):
			reloaded()
		}
	}
}

func dbusConnectionChanged(args []interface{}) {
	name := args[0].(string)
	oldOwner := args[1].(string)
	newOwner := args[2].(string)

	if name != dbusInterface {
		return
	}

	if len(newOwner) > 0 {
		connectionEstablished()
	} else if len(oldOwner) > 0 {
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

// OnReloaded add callback
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
	if connection == nil {
		return false
	}
	var zone string
	err := connection.sysObj.Call(dbusInterface+".getDefaultZone", 0).Store(&zone)
	return err == nil
}

// Passthrough method simply passes args through to iptables/ip6tables
func Passthrough(ipv IPV, args ...string) ([]byte, error) {
	var output string
	log.G(context.TODO()).Debugf("Firewalld passthrough: %s, %s", ipv, args)
	if err := connection.sysObj.Call(dbusInterface+".direct.passthrough", 0, ipv, args).Store(&output); err != nil {
		return nil, err
	}
	return []byte(output), nil
}

// firewalldZone holds the firewalld zone settings.
//
// Documented in https://firewalld.org/documentation/man-pages/firewalld.dbus.html#FirewallD1.zone
type firewalldZone struct {
	version            string
	name               string
	description        string
	unused             bool
	target             string
	services           []string
	ports              [][]interface{}
	icmpBlocks         []string
	masquerade         bool
	forwardPorts       [][]interface{}
	interfaces         []string
	sourceAddresses    []string
	richRules          []string
	protocols          []string
	sourcePorts        [][]interface{}
	icmpBlockInversion bool
}

// settings returns the firewalldZone struct as an interface slice,
// which can be passed to "org.fedoraproject.FirewallD1.config.addZone".
func (z firewalldZone) settings() []interface{} {
	// TODO(thaJeztah): does D-Bus require optional fields to be passed as well?
	return []interface{}{
		z.version,
		z.name,
		z.description,
		z.unused,
		z.target,
		z.services,
		z.ports,
		z.icmpBlocks,
		z.masquerade,
		z.forwardPorts,
		z.interfaces,
		z.sourceAddresses,
		z.richRules,
		z.protocols,
		z.sourcePorts,
		z.icmpBlockInversion,
	}
}

// setupDockerZone creates a zone called docker in firewalld which includes docker interfaces to allow
// container networking
func setupDockerZone() error {
	var zones []string
	// Check if zone exists
	if err := connection.sysObj.Call(dbusInterface+".zone.getZones", 0).Store(&zones); err != nil {
		return err
	}
	if contains(zones, dockerZone) {
		log.G(context.TODO()).Infof("Firewalld: %s zone already exists, returning", dockerZone)
		return nil
	}
	log.G(context.TODO()).Debugf("Firewalld: creating %s zone", dockerZone)

	// Permanent
	dz := firewalldZone{
		version:     "1.0",
		name:        dockerZone,
		description: "zone for docker bridge network interfaces",
		target:      "ACCEPT",
	}
	if err := connection.sysConfObj.Call(dbusInterface+".config.addZone", 0, dockerZone, dz.settings()).Err; err != nil {
		return err
	}
	// Reload for change to take effect
	if err := connection.sysObj.Call(dbusInterface+".reload", 0).Err; err != nil {
		return err
	}

	return nil
}

// AddInterfaceFirewalld adds the interface to the trusted zone. It is a
// no-op if firewalld is not running.
func AddInterfaceFirewalld(intf string) error {
	if !firewalldRunning {
		return nil
	}

	var intfs []string
	// Check if interface is already added to the zone
	if err := connection.sysObj.Call(dbusInterface+".zone.getInterfaces", 0, dockerZone).Store(&intfs); err != nil {
		return err
	}
	// Return if interface is already part of the zone
	if contains(intfs, intf) {
		log.G(context.TODO()).Infof("Firewalld: interface %s already part of %s zone, returning", intf, dockerZone)
		return nil
	}

	log.G(context.TODO()).Debugf("Firewalld: adding %s interface to %s zone", intf, dockerZone)
	// Runtime
	if err := connection.sysObj.Call(dbusInterface+".zone.addInterface", 0, dockerZone, intf).Err; err != nil {
		return err
	}
	return nil
}

// DelInterfaceFirewalld removes the interface from the trusted zone It is a
// no-op if firewalld is not running.
func DelInterfaceFirewalld(intf string) error {
	if !firewalldRunning {
		return nil
	}

	var intfs []string
	// Check if interface is part of the zone
	if err := connection.sysObj.Call(dbusInterface+".zone.getInterfaces", 0, dockerZone).Store(&intfs); err != nil {
		return err
	}
	// Remove interface if it exists
	if !contains(intfs, intf) {
		return &interfaceNotFound{fmt.Errorf("firewalld: interface %q not found in %s zone", intf, dockerZone)}
	}

	log.G(context.TODO()).Debugf("Firewalld: removing %s interface from %s zone", intf, dockerZone)
	// Runtime
	if err := connection.sysObj.Call(dbusInterface+".zone.removeInterface", 0, dockerZone, intf).Err; err != nil {
		return err
	}
	return nil
}

type interfaceNotFound struct{ error }

func (interfaceNotFound) NotFound() {}

func contains(list []string, val string) bool {
	for _, v := range list {
		if v == val {
			return true
		}
	}
	return false
}

func initFirewalld() {
	// When running with RootlessKit, firewalld is running as the root outside our network namespace
	// https://github.com/moby/moby/issues/43781
	if rootless.RunningWithRootlessKit() {
		log.G(context.TODO()).Info("skipping firewalld management for rootless mode")
		return
	}
	if err := firewalldInit(); err != nil {
		log.G(context.TODO()).WithError(err).Debugf("unable to initialize firewalld; using raw iptables instead")
	}
}

// Raw calls 'iptables' system command, passing supplied arguments.
func (iptable IPTable) Raw(args ...string) ([]byte, error) {
	if firewalldRunning {
		// select correct IP version for firewalld
		ipv := Iptables
		if iptable.ipVersion == IPv6 {
			ipv = IP6Tables
		}

		startTime := time.Now()
		output, err := Passthrough(ipv, args...)
		if err == nil || !strings.Contains(err.Error(), "was not provided by any .service files") {
			return filterOutput(startTime, output, args...), err
		}
	}
	return iptable.raw(args...)
}
