//go:build linux

package iptables

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/containerd/log"
	dbus "github.com/godbus/dbus/v5"
)

const (
	// ipTables point ipv4 table
	ipTables = "ipv4"
	// ip6Tables point to ipv6 table
	ip6Tables = "ipv6"
)

const (
	// See https://github.com/firewalld/firewalld/blob/v1.3.3/doc/xml/firewalld.dbus.xml#L49-L64
	dbusInterface  = "org.fedoraproject.FirewallD1"
	dbusPath       = "/org/fedoraproject/FirewallD1"
	dbusConfigPath = "/org/fedoraproject/FirewallD1/config"
	dockerZone     = "docker"
)

// firewalldConnection is a connection to the firewalld dbus endpoint.
type firewalldConnection struct {
	conn       *dbus.Conn
	running    atomic.Bool
	sysObj     dbus.BusObject
	sysConfObj dbus.BusObject
	signal     chan *dbus.Signal

	onReloaded []*func() // callbacks to run when Firewalld is reloaded.
}

var firewalld *firewalldConnection

// firewalldInit initializes firewalld management code.
func firewalldInit() (*firewalldConnection, error) {
	fwd, err := newConnection()
	if err != nil {
		return nil, err
	}

	// start handling D-Bus signals that were registered.
	fwd.handleSignals()

	err = fwd.setupDockerZone()
	if err != nil {
		_ = fwd.conn.Close()
		return nil, err
	}

	return fwd, nil
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

	if !c.checkRunning() {
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
				fwd.checkRunning()
				fwd.onConnectionChange(signal.Body)

			case strings.Contains(signal.Name, "Reloaded"):
				fwd.onReload()
			}
		}
	}()
}

func (fwd *firewalldConnection) onConnectionChange(args []any) {
	if name := args[0].(string); name != dbusInterface {
		return
	}
	oldOwner := args[1].(string)
	newOwner := args[2].(string)
	if len(newOwner) > 0 {
		fwd.onConnectionEstablished()
	} else if len(oldOwner) > 0 {
		fwd.onConnectionLost()
	}
}

func (fwd *firewalldConnection) onConnectionEstablished() {
	fwd.onReload()
}

func (fwd *firewalldConnection) onConnectionLost() {
	// Doesn't do anything for now. Libvirt also doesn't react to this.
}

// onReload executes all registered callbacks.
func (fwd *firewalldConnection) onReload() {
	// Note that we're not checking if firewalld is running here,
	// as the previous code did not check for this, and (technically),
	// the callbacks may not require firewalld. So we're assuming here
	// that all callback can be executed if this onReload function
	// is triggered (either from a D-Bus event, or otherwise).
	//
	// TODO(thaJeztah): verify if these should always be run, or only if firewalld is running.
	if fwd == nil {
		return
	}
	for _, pf := range fwd.onReloaded {
		(*pf)()
	}
}

// registerReloadCallback adds a callback to be executed when firewalld
// is reloaded. Adding a callback is idempotent; it ignores the given
// callback if it's already registered.
//
// It is a no-op if firewalld is not running or firewalldConnection is not
// initialized.
func (fwd *firewalldConnection) registerReloadCallback(callback func()) {
	// Note that we're not checking if firewalld is running here,
	// as the previous code did not check for this, and (technically),
	// the callbacks may not require firewalld, or may be registered
	// when firewalld is not running.
	//
	// We're assuming here that all callback can be executed if this
	// onReload function is triggered (either from a D-Bus event, or
	// otherwise).
	//
	// TODO(thaJeztah): verify if these should always be run, or only if firewalld is running at the moment the callback is registered.
	if fwd == nil {
		return
	}
	for _, pf := range fwd.onReloaded {
		if pf == &callback {
			return
		}
	}
	fwd.onReloaded = append(fwd.onReloaded, &callback)
}

// checkRunning checks if firewalld is running.
//
// It calls some remote method to see whether the service is actually running.
func (fwd *firewalldConnection) checkRunning() bool {
	var zone string
	if err := fwd.sysObj.Call(dbusInterface+".getDefaultZone", 0).Store(&zone); err != nil {
		fwd.running.Store(false)
	} else {
		fwd.running.Store(true)
	}
	return fwd.running.Load()
}

// isRunning returns whether firewalld is running.
func (fwd *firewalldConnection) isRunning() bool {
	if fwd == nil {
		return false
	}
	return fwd.running.Load()
}

// passthrough passes args through to iptables or ip6tables.
//
// It is a no-op if firewalld is not running or not initialized.
func (fwd *firewalldConnection) passthrough(ipVersion IPVersion, args ...string) ([]byte, error) {
	if !fwd.isRunning() {
		return []byte(""), nil
	}

	// select correct IP version for firewalld
	ipv := ipTables
	if ipVersion == IPv6 {
		ipv = ip6Tables
	}

	var output string
	log.G(context.TODO()).Debugf("Firewalld passthrough: %s, %s", ipv, args)
	if err := fwd.sysObj.Call(dbusInterface+".direct.passthrough", 0, ipv, args).Store(&output); err != nil {
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

// setupDockerZone creates a zone called docker in firewalld which includes
// docker interfaces to allow container networking.
func (fwd *firewalldConnection) setupDockerZone() error {
	var zones []string
	// Check if zone exists
	if err := fwd.sysObj.Call(dbusInterface+".zone.getZones", 0).Store(&zones); err != nil {
		return fmt.Errorf("firewalld: failed to check if %s zone already exists: %v", dockerZone, err)
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
	if err := fwd.sysConfObj.Call(dbusInterface+".config.addZone", 0, dockerZone, dz.settings()).Err; err != nil {
		return fmt.Errorf("firewalld: failed to set up %s zone: %v", dockerZone, err)
	}
	// Reload for change to take effect
	if err := fwd.sysObj.Call(dbusInterface+".reload", 0).Err; err != nil {
		return fmt.Errorf("firewalld: failed to set up %s zone: %v", dockerZone, err)
	}

	return nil
}

// addInterface adds the interface to the trusted zone. It is a no-op if
// firewalld is not running or firewalldConnection not initialized.
func (fwd *firewalldConnection) addInterface(intf string) error {
	if !fwd.isRunning() {
		return nil
	}

	var intfs []string
	// Check if interface is already added to the zone
	if err := fwd.sysObj.Call(dbusInterface+".zone.getInterfaces", 0, dockerZone).Store(&intfs); err != nil {
		return err
	}
	// Return if interface is already part of the zone
	if contains(intfs, intf) {
		log.G(context.TODO()).Infof("Firewalld: interface %s already part of %s zone, returning", intf, dockerZone)
		return nil
	}

	log.G(context.TODO()).Debugf("Firewalld: adding %s interface to %s zone", intf, dockerZone)
	// Runtime
	if err := fwd.sysObj.Call(dbusInterface+".zone.addInterface", 0, dockerZone, intf).Err; err != nil {
		return err
	}
	return nil
}

// delInterface removes the interface from the trusted zone It is a no-op if
// firewalld is not running or firewalldConnection not initialized.
func (fwd *firewalldConnection) delInterface(intf string) error {
	if !fwd.isRunning() {
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
