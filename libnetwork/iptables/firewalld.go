//go:build linux

package iptables

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/containerd/log"
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

// firewalldConnection is a connection to the firewalld dbus endpoint.
type firewalldConnection struct {
	conn       *dbus.Conn
	sysObj     dbus.BusObject
	sysConfObj dbus.BusObject
	signal     chan *dbus.Signal
}

var (
	firewalld *firewalldConnection

	firewalldRunning bool      // is Firewalld service running
	onReloaded       []*func() // callbacks when Firewalld has been reloaded
)

// firewalldInit initializes firewalld management code.
func firewalldInit() error {
	var err error
	firewalld, err = newConnection()
	if err != nil {
		return err
	}

	// start handling D-Bus signals that were registered.
	firewalld.handleSignals()

	err = setupDockerZone()
	if err != nil {
		return err
	}

	return nil
}

// newConnection establishes a connection to the system D-Bus and registers
// signals to listen on.
//
// It returns an error if it's unable to establish a D-Bus connection, or
// if firewalld is not running.
func newConnection() (*firewalldConnection, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to D-Bus system bus: %v", err)
	}

	c := &firewalldConnection{
		conn:   conn,
		signal: make(chan *dbus.Signal, 10),

		// This never fails, even if the service is not running atm.
		sysObj:     conn.Object(dbusInterface, dbusPath),
		sysConfObj: conn.Object(dbusInterface, dbusConfigPath),
	}

	firewalldRunning = checkRunning(c)
	if !firewalldRunning {
		_ = c.conn.Close()
		return nil, fmt.Errorf("firewalld is not running")
	}

	return c, nil
}

// handleSignals sets up handling for D-Bus signals (NameOwnerChanged, Reloaded),
// to reload rules when firewalld is reloaded .
func (fwd *firewalldConnection) handleSignals() {
	rule := fmt.Sprintf("type='signal',path='%s',interface='%s',sender='%s',member='Reloaded'", dbusPath, dbusInterface, dbusInterface)
	fwd.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule)

	rule = fmt.Sprintf("type='signal',interface='org.freedesktop.DBus',member='NameOwnerChanged',path='/org/freedesktop/DBus',sender='org.freedesktop.DBus',arg0='%s'", dbusInterface)
	fwd.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule)
	fwd.conn.Signal(fwd.signal)

	// FIXME(thaJeztah): there's currently no way to terminate this goroutine.
	// TODO(thaJeztah): should this be rewritten to use dbus.WithSignalHandler(), instead of a self-crafted solution?
	go func() {
		for signal := range fwd.signal {
			switch {
			case strings.Contains(signal.Name, "NameOwnerChanged"):
				// re-check if firewalld is still running.
				checkRunning(fwd)
				dbusConnectionChanged(signal.Body)

			case strings.Contains(signal.Name, "Reloaded"):
				reloaded()
			}
		}
	}()
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

// checkRunning checks if firewalld is running.
//
// It calls some remote method to see whether the service is actually running.
func checkRunning(conn *firewalldConnection) bool {
	if conn == nil {
		return false
	}
	var zone string
	err := conn.sysObj.Call(dbusInterface+".getDefaultZone", 0).Store(&zone)
	return err == nil
}

// Passthrough method simply passes args through to iptables/ip6tables
func Passthrough(ipv IPV, args ...string) ([]byte, error) {
	var output string
	log.G(context.TODO()).Debugf("Firewalld passthrough: %s, %s", ipv, args)
	if err := firewalld.sysObj.Call(dbusInterface+".direct.passthrough", 0, ipv, args).Store(&output); err != nil {
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
	if err := firewalld.sysObj.Call(dbusInterface+".zone.getZones", 0).Store(&zones); err != nil {
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
	if err := firewalld.sysConfObj.Call(dbusInterface+".config.addZone", 0, dockerZone, dz.settings()).Err; err != nil {
		return err
	}
	// Reload for change to take effect
	if err := firewalld.sysObj.Call(dbusInterface+".reload", 0).Err; err != nil {
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
	if err := firewalld.sysObj.Call(dbusInterface+".zone.getInterfaces", 0, dockerZone).Store(&intfs); err != nil {
		return err
	}
	// Return if interface is already part of the zone
	if contains(intfs, intf) {
		log.G(context.TODO()).Infof("Firewalld: interface %s already part of %s zone, returning", intf, dockerZone)
		return nil
	}

	log.G(context.TODO()).Debugf("Firewalld: adding %s interface to %s zone", intf, dockerZone)
	// Runtime
	if err := firewalld.sysObj.Call(dbusInterface+".zone.addInterface", 0, dockerZone, intf).Err; err != nil {
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
	if err := firewalld.sysObj.Call(dbusInterface+".zone.getInterfaces", 0, dockerZone).Store(&intfs); err != nil {
		return err
	}
	// Remove interface if it exists
	if !contains(intfs, intf) {
		return &interfaceNotFound{fmt.Errorf("firewalld: interface %q not found in %s zone", intf, dockerZone)}
	}

	log.G(context.TODO()).Debugf("Firewalld: removing %s interface from %s zone", intf, dockerZone)
	// Runtime
	if err := firewalld.sysObj.Call(dbusInterface+".zone.removeInterface", 0, dockerZone, intf).Err; err != nil {
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
