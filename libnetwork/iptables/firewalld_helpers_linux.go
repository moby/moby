package iptables

// OnReloaded adds a callback to be executed when firewalld is reloaded.
// Adding a callback is idempotent; it ignores the given callback if it's
// already registered.
//
// Callbacks can be registered regardless if firewalld is currently running,
// but it will initialize firewalld before executing.
func OnReloaded(callback func()) {
	// Make sure firewalld is initialized before we register callbacks.
	// This function is also called from setupArrangeUserFilterRule,
	// which is called during controller initialization.
	_ = initCheck()
	firewalld.registerReloadCallback(callback)
}

// AddInterfaceFirewalld adds the interface to the trusted zone. It is a
// no-op if firewalld is not running.
func AddInterfaceFirewalld(intf string) error {
	return firewalld.addInterface(intf)
}

// DelInterfaceFirewalld removes the interface from the trusted zone It is a
// no-op if firewalld is not running.
func DelInterfaceFirewalld(intf string) error {
	return firewalld.delInterface(intf)
}
