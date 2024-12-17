//go:build linux

package iptables

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/log"
	dbus "github.com/godbus/dbus/v5"
	"github.com/pkg/errors"
)

const (
	dbusInterface   = "org.fedoraproject.FirewallD1"
	dbusPath        = "/org/fedoraproject/FirewallD1"
	dbusConfigPath  = "/org/fedoraproject/FirewallD1/config"
	dockerZone      = "docker"
	dockerFwdPolicy = "docker-forwarding"
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
		zoneAdded, err := setupDockerZone()
		if err != nil {
			return err
		}
		policyAdded, policyAddErr := setupDockerForwardingPolicy()
		if policyAddErr != nil {
			// Log the error, but still reload firewalld if necessary.
			log.G(context.TODO()).WithError(policyAddErr).Warnf("Firewalld: failed to add policy %s", dockerFwdPolicy)
		}
		if zoneAdded || policyAdded {
			// Reload for changes to take effect.
			if err := connection.sysObj.Call(dbusInterface+".reload", 0).Err; err != nil {
				return err
			}
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

// passthrough method simply passes args through to iptables/ip6tables
func passthrough(ipv IPVersion, args ...string) ([]byte, error) {
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

// settings returns the firewalldZone struct as an interface slice, which can be
// passed to "org.fedoraproject.FirewallD1.config.addZone". Note that 'addZone',
// which is deprecated, requires this whole struct. Its replacement, 'addZone2'
// (introduced in firewalld 0.9.0) accepts a dictionary where only non-default
// values need to be specified.
func (z firewalldZone) settings() []interface{} {
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
// container networking. The bool return value is true if a firewalld reload is required.
func setupDockerZone() (bool, error) {
	var zones []string
	// Check if zone exists
	if err := connection.sysObj.Call(dbusInterface+".zone.getZones", 0).Store(&zones); err != nil {
		return false, err
	}
	if contains(zones, dockerZone) {
		log.G(context.TODO()).Infof("Firewalld: %s zone already exists, returning", dockerZone)
		return false, nil
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
		return false, err
	}
	return true, nil
}

// setupDockerForwardingPolicy creates a policy to allow forwarding to anywhere to the docker
// zone (where packets will be dealt with by docker's usual/non-firewalld configuration).
// The bool return value is true if a firewalld reload is required.
func setupDockerForwardingPolicy() (bool, error) {
	// https://firewalld.org/documentation/man-pages/firewalld.dbus.html#FirewallD1.config
	policy := map[string]interface{}{
		"version":       "1.0",
		"description":   "allow forwarding to the docker zone",
		"ingress_zones": []string{"ANY"},
		"egress_zones":  []string{dockerZone},
		"target":        "ACCEPT",
	}
	if err := connection.sysConfObj.Call(dbusInterface+".config.addPolicy", 0, dockerFwdPolicy, policy).Err; err != nil {
		var derr dbus.Error
		if errors.As(err, &derr) {
			if derr.Name == dbusInterface+".Exception" && strings.HasPrefix(err.Error(), "NAME_CONFLICT") {
				log.G(context.TODO()).Debugf("Firewalld: %s policy already exists", dockerFwdPolicy)
				return false, nil
			}
			if derr.Name == dbus.ErrMsgUnknownMethod.Name {
				log.G(context.TODO()).Debugf("Firewalld: addPolicy %s: unknown method", dockerFwdPolicy)
				return false, nil
			}
		}
		return false, err
	}
	log.G(context.TODO()).Infof("Firewalld: created %s policy", dockerFwdPolicy)
	return true, nil
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
