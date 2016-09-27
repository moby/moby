package libvirt

/*
#cgo LDFLAGS: -lvirt 
#include <libvirt/libvirt.h>
#include <libvirt/virterror.h>
#include <stdlib.h>

#ifndef VIR_FROM_BHYVE
#define VIR_FROM_BHYVE 57
#endif

#ifndef VIR_FROM_CRYPTO
#define VIR_FROM_CRYPTO 58
#endif

#ifndef VIR_FROM_FIREWALL
#define VIR_FROM_FIREWALL 59
#endif

*/
import "C"

import "fmt"

// virErrorLevel
const (
	VIR_ERR_NONE    = C.VIR_ERR_NONE
	VIR_ERR_WARNING = C.VIR_ERR_WARNING
	VIR_ERR_ERROR   = C.VIR_ERR_ERROR
)

// virErrorNumber
const (
	VIR_ERR_OK = C.VIR_ERR_OK

	// internal error
	VIR_ERR_INTERNAL_ERROR = C.VIR_ERR_INTERNAL_ERROR

	// memory allocation failure
	VIR_ERR_NO_MEMORY = C.VIR_ERR_NO_MEMORY

	// no support for this function
	VIR_ERR_NO_SUPPORT = C.VIR_ERR_NO_SUPPORT

	// could not resolve hostname
	VIR_ERR_UNKNOWN_HOST = C.VIR_ERR_UNKNOWN_HOST

	// can't connect to hypervisor
	VIR_ERR_NO_CONNECT = C.VIR_ERR_NO_CONNECT

	// invalid connection object
	VIR_ERR_INVALID_CONN = C.VIR_ERR_INVALID_CONN

	// invalid domain object
	VIR_ERR_INVALID_DOMAIN = C.VIR_ERR_INVALID_DOMAIN

	// invalid function argument
	VIR_ERR_INVALID_ARG = C.VIR_ERR_INVALID_ARG

	// a command to hypervisor failed
	VIR_ERR_OPERATION_FAILED = C.VIR_ERR_OPERATION_FAILED

	// a HTTP GET command to failed
	VIR_ERR_GET_FAILED = C.VIR_ERR_GET_FAILED

	// a HTTP POST command to failed
	VIR_ERR_POST_FAILED = C.VIR_ERR_POST_FAILED

	// unexpected HTTP error code
	VIR_ERR_HTTP_ERROR = C.VIR_ERR_HTTP_ERROR

	// failure to serialize an S-Expr
	VIR_ERR_SEXPR_SERIAL = C.VIR_ERR_SEXPR_SERIAL

	// could not open Xen hypervisor control
	VIR_ERR_NO_XEN = C.VIR_ERR_NO_XEN

	// failure doing an hypervisor call
	VIR_ERR_XEN_CALL = C.VIR_ERR_XEN_CALL

	// unknown OS type
	VIR_ERR_OS_TYPE = C.VIR_ERR_OS_TYPE

	// missing kernel information
	VIR_ERR_NO_KERNEL = C.VIR_ERR_NO_KERNEL

	// missing root device information
	VIR_ERR_NO_ROOT = C.VIR_ERR_NO_ROOT

	// missing source device information
	VIR_ERR_NO_SOURCE = C.VIR_ERR_NO_SOURCE

	// missing target device information
	VIR_ERR_NO_TARGET = C.VIR_ERR_NO_TARGET

	// missing domain name information
	VIR_ERR_NO_NAME = C.VIR_ERR_NO_NAME

	// missing domain OS information
	VIR_ERR_NO_OS = C.VIR_ERR_NO_OS

	// missing domain devices information
	VIR_ERR_NO_DEVICE = C.VIR_ERR_NO_DEVICE

	// could not open Xen Store control
	VIR_ERR_NO_XENSTORE = C.VIR_ERR_NO_XENSTORE

	// too many drivers registered
	VIR_ERR_DRIVER_FULL = C.VIR_ERR_DRIVER_FULL

	// not supported by the drivers (DEPRECATED)
	VIR_ERR_CALL_FAILED = C.VIR_ERR_CALL_FAILED

	// an XML description is not well formed or broken
	VIR_ERR_XML_ERROR = C.VIR_ERR_XML_ERROR

	// the domain already exist
	VIR_ERR_DOM_EXIST = C.VIR_ERR_DOM_EXIST

	// operation forbidden on read-only connections
	VIR_ERR_OPERATION_DENIED = C.VIR_ERR_OPERATION_DENIED

	// failed to open a conf file
	VIR_ERR_OPEN_FAILED = C.VIR_ERR_OPEN_FAILED

	// failed to read a conf file
	VIR_ERR_READ_FAILED = C.VIR_ERR_READ_FAILED

	// failed to parse a conf file
	VIR_ERR_PARSE_FAILED = C.VIR_ERR_PARSE_FAILED

	// failed to parse the syntax of a conf file
	VIR_ERR_CONF_SYNTAX = C.VIR_ERR_CONF_SYNTAX

	// failed to write a conf file
	VIR_ERR_WRITE_FAILED = C.VIR_ERR_WRITE_FAILED

	// detail of an XML error
	VIR_ERR_XML_DETAIL = C.VIR_ERR_XML_DETAIL

	// invalid network object
	VIR_ERR_INVALID_NETWORK = C.VIR_ERR_INVALID_NETWORK

	// the network already exist
	VIR_ERR_NETWORK_EXIST = C.VIR_ERR_NETWORK_EXIST

	// general system call failure
	VIR_ERR_SYSTEM_ERROR = C.VIR_ERR_SYSTEM_ERROR

	// some sort of RPC error
	VIR_ERR_RPC = C.VIR_ERR_RPC

	// error from a GNUTLS call
	VIR_ERR_GNUTLS_ERROR = C.VIR_ERR_GNUTLS_ERROR

	// failed to start network
	VIR_WAR_NO_NETWORK = C.VIR_WAR_NO_NETWORK

	// domain not found or unexpectedly disappeared
	VIR_ERR_NO_DOMAIN = C.VIR_ERR_NO_DOMAIN

	// network not found
	VIR_ERR_NO_NETWORK = C.VIR_ERR_NO_NETWORK

	// invalid MAC address
	VIR_ERR_INVALID_MAC = C.VIR_ERR_INVALID_MAC

	// authentication failed
	VIR_ERR_AUTH_FAILED = C.VIR_ERR_AUTH_FAILED

	// invalid storage pool object
	VIR_ERR_INVALID_STORAGE_POOL = C.VIR_ERR_INVALID_STORAGE_POOL

	// invalid storage vol object
	VIR_ERR_INVALID_STORAGE_VOL = C.VIR_ERR_INVALID_STORAGE_VOL

	// failed to start storage
	VIR_WAR_NO_STORAGE = C.VIR_WAR_NO_STORAGE

	// storage pool not found
	VIR_ERR_NO_STORAGE_POOL = C.VIR_ERR_NO_STORAGE_POOL

	// storage volume not found
	VIR_ERR_NO_STORAGE_VOL = C.VIR_ERR_NO_STORAGE_VOL

	// failed to start node driver
	VIR_WAR_NO_NODE = C.VIR_WAR_NO_NODE

	// invalid node device object
	VIR_ERR_INVALID_NODE_DEVICE = C.VIR_ERR_INVALID_NODE_DEVICE

	// node device not found
	VIR_ERR_NO_NODE_DEVICE = C.VIR_ERR_NO_NODE_DEVICE

	// security model not found
	VIR_ERR_NO_SECURITY_MODEL = C.VIR_ERR_NO_SECURITY_MODEL

	// operation is not applicable at this time
	VIR_ERR_OPERATION_INVALID = C.VIR_ERR_OPERATION_INVALID

	// failed to start interface driver
	VIR_WAR_NO_INTERFACE = C.VIR_WAR_NO_INTERFACE

	// interface driver not running
	VIR_ERR_NO_INTERFACE = C.VIR_ERR_NO_INTERFACE

	// invalid interface object
	VIR_ERR_INVALID_INTERFACE = C.VIR_ERR_INVALID_INTERFACE

	// more than one matching interface found
	VIR_ERR_MULTIPLE_INTERFACES = C.VIR_ERR_MULTIPLE_INTERFACES

	// failed to start nwfilter driver
	VIR_WAR_NO_NWFILTER = C.VIR_WAR_NO_NWFILTER

	// invalid nwfilter object
	VIR_ERR_INVALID_NWFILTER = C.VIR_ERR_INVALID_NWFILTER

	// nw filter pool not found
	VIR_ERR_NO_NWFILTER = C.VIR_ERR_NO_NWFILTER

	// nw filter pool not found
	VIR_ERR_BUILD_FIREWALL = C.VIR_ERR_BUILD_FIREWALL

	// failed to start secret storage
	VIR_WAR_NO_SECRET = C.VIR_WAR_NO_SECRET

	// invalid secret
	VIR_ERR_INVALID_SECRET = C.VIR_ERR_INVALID_SECRET

	// secret not found
	VIR_ERR_NO_SECRET = C.VIR_ERR_NO_SECRET

	// unsupported configuration construct
	VIR_ERR_CONFIG_UNSUPPORTED = C.VIR_ERR_CONFIG_UNSUPPORTED

	// timeout occurred during operation
	VIR_ERR_OPERATION_TIMEOUT = C.VIR_ERR_OPERATION_TIMEOUT

	// a migration worked, but making the VM persist on the dest host failed
	VIR_ERR_MIGRATE_PERSIST_FAILED = C.VIR_ERR_MIGRATE_PERSIST_FAILED

	// a synchronous hook script failed
	VIR_ERR_HOOK_SCRIPT_FAILED = C.VIR_ERR_HOOK_SCRIPT_FAILED

	// invalid domain snapshot
	VIR_ERR_INVALID_DOMAIN_SNAPSHOT = C.VIR_ERR_INVALID_DOMAIN_SNAPSHOT

	// domain snapshot not found
	VIR_ERR_NO_DOMAIN_SNAPSHOT = C.VIR_ERR_NO_DOMAIN_SNAPSHOT

	// stream pointer not valid
	VIR_ERR_INVALID_STREAM = C.VIR_ERR_INVALID_STREAM

	// valid API use but unsupported by the given driver
	VIR_ERR_ARGUMENT_UNSUPPORTED = C.VIR_ERR_ARGUMENT_UNSUPPORTED

	// storage pool probe failed
	VIR_ERR_STORAGE_PROBE_FAILED = C.VIR_ERR_STORAGE_PROBE_FAILED

	// storage pool already built
	VIR_ERR_STORAGE_POOL_BUILT = C.VIR_ERR_STORAGE_POOL_BUILT

	// force was not requested for a risky domain snapshot revert
	VIR_ERR_SNAPSHOT_REVERT_RISKY = C.VIR_ERR_SNAPSHOT_REVERT_RISKY

	// operation on a domain was canceled/aborted by user
	VIR_ERR_OPERATION_ABORTED = C.VIR_ERR_OPERATION_ABORTED

	// authentication cancelled
	VIR_ERR_AUTH_CANCELLED = C.VIR_ERR_AUTH_CANCELLED

	// The metadata is not present
	VIR_ERR_NO_DOMAIN_METADATA = C.VIR_ERR_NO_DOMAIN_METADATA

	// Migration is not safe
	VIR_ERR_MIGRATE_UNSAFE = C.VIR_ERR_MIGRATE_UNSAFE

	// integer overflow
	VIR_ERR_OVERFLOW = C.VIR_ERR_OVERFLOW

	// action prevented by block copy job
	VIR_ERR_BLOCK_COPY_ACTIVE = C.VIR_ERR_BLOCK_COPY_ACTIVE

	// The requested operation is not supported
	VIR_ERR_OPERATION_UNSUPPORTED = C.VIR_ERR_OPERATION_UNSUPPORTED

	// error in ssh transport driver
	VIR_ERR_SSH = C.VIR_ERR_SSH

	// guest agent is unresponsive, not running or not usable
	VIR_ERR_AGENT_UNRESPONSIVE = C.VIR_ERR_AGENT_UNRESPONSIVE

	// resource is already in use
	VIR_ERR_RESOURCE_BUSY = C.VIR_ERR_RESOURCE_BUSY

	// operation on the object/resource was denied
	VIR_ERR_ACCESS_DENIED = C.VIR_ERR_ACCESS_DENIED

	// error from a dbus service
	VIR_ERR_DBUS_SERVICE = C.VIR_ERR_DBUS_SERVICE

	// the storage vol already exists
	VIR_ERR_STORAGE_VOL_EXIST = C.VIR_ERR_STORAGE_VOL_EXIST

	// given CPU is incompatible with host CPU
	// added in libvirt 1.2.6
	// VIR_ERR_CPU_INCOMPATIBLE = C.VIR_ERR_CPU_INCOMPATIBLE
)

// virErrorDomain
const (
	VIR_FROM_NONE = C.VIR_FROM_NONE

	// Error at Xen hypervisor layer
	VIR_FROM_XEN = C.VIR_FROM_XEN

	// Error at connection with xend daemon
	VIR_FROM_XEND = C.VIR_FROM_XEND

	// Error at connection with xen store
	VIR_FROM_XENSTORE = C.VIR_FROM_XENSTORE

	// Error in the S-Expression code
	VIR_FROM_SEXPR = C.VIR_FROM_SEXPR

	// Error in the XML code
	VIR_FROM_XML = C.VIR_FROM_XML

	// Error when operating on a domain
	VIR_FROM_DOM = C.VIR_FROM_DOM

	// Error in the XML-RPC code
	VIR_FROM_RPC = C.VIR_FROM_RPC

	// Error in the proxy code; unused since 0.8.6
	VIR_FROM_PROXY = C.VIR_FROM_PROXY

	// Error in the configuration file handling
	VIR_FROM_CONF = C.VIR_FROM_CONF

	// Error at the QEMU daemon
	VIR_FROM_QEMU = C.VIR_FROM_QEMU

	// Error when operating on a network
	VIR_FROM_NET = C.VIR_FROM_NET

	// Error from test driver
	VIR_FROM_TEST = C.VIR_FROM_TEST

	// Error from remote driver
	VIR_FROM_REMOTE = C.VIR_FROM_REMOTE

	// Error from OpenVZ driver
	VIR_FROM_OPENVZ = C.VIR_FROM_OPENVZ

	// Error at Xen XM layer
	VIR_FROM_XENXM = C.VIR_FROM_XENXM

	// Error in the Linux Stats code
	VIR_FROM_STATS_LINUX = C.VIR_FROM_STATS_LINUX

	// Error from Linux Container driver
	VIR_FROM_LXC = C.VIR_FROM_LXC

	// Error from storage driver
	VIR_FROM_STORAGE = C.VIR_FROM_STORAGE

	// Error from network config
	VIR_FROM_NETWORK = C.VIR_FROM_NETWORK

	// Error from domain config
	VIR_FROM_DOMAIN = C.VIR_FROM_DOMAIN

	// Error at the UML driver
	VIR_FROM_UML = C.VIR_FROM_UML

	// Error from node device monitor
	VIR_FROM_NODEDEV = C.VIR_FROM_NODEDEV

	// Error from xen inotify layer
	VIR_FROM_XEN_INOTIFY = C.VIR_FROM_XEN_INOTIFY

	// Error from security framework
	VIR_FROM_SECURITY = C.VIR_FROM_SECURITY

	// Error from VirtualBox driver
	VIR_FROM_VBOX = C.VIR_FROM_VBOX

	// Error when operating on an interface
	VIR_FROM_INTERFACE = C.VIR_FROM_INTERFACE

	// The OpenNebula driver no longer exists. Retained for ABI/API compat only
	VIR_FROM_ONE = C.VIR_FROM_ONE

	// Error from ESX driver
	VIR_FROM_ESX = C.VIR_FROM_ESX

	// Error from IBM power hypervisor
	VIR_FROM_PHYP = C.VIR_FROM_PHYP

	// Error from secret storage
	VIR_FROM_SECRET = C.VIR_FROM_SECRET

	// Error from CPU driver
	VIR_FROM_CPU = C.VIR_FROM_CPU

	// Error from XenAPI
	VIR_FROM_XENAPI = C.VIR_FROM_XENAPI

	// Error from network filter driver
	VIR_FROM_NWFILTER = C.VIR_FROM_NWFILTER

	// Error from Synchronous hooks
	VIR_FROM_HOOK = C.VIR_FROM_HOOK

	// Error from domain snapshot
	VIR_FROM_DOMAIN_SNAPSHOT = C.VIR_FROM_DOMAIN_SNAPSHOT

	// Error from auditing subsystem
	VIR_FROM_AUDIT = C.VIR_FROM_AUDIT

	// Error from sysinfo/SMBIOS
	VIR_FROM_SYSINFO = C.VIR_FROM_SYSINFO

	// Error from I/O streams
	VIR_FROM_STREAMS = C.VIR_FROM_STREAMS

	// Error from VMware driver
	VIR_FROM_VMWARE = C.VIR_FROM_VMWARE

	// Error from event loop impl
	VIR_FROM_EVENT = C.VIR_FROM_EVENT

	// Error from libxenlight driver
	VIR_FROM_LIBXL = C.VIR_FROM_LIBXL

	// Error from lock manager
	VIR_FROM_LOCKING = C.VIR_FROM_LOCKING

	// Error from Hyper-V driver
	VIR_FROM_HYPERV = C.VIR_FROM_HYPERV

	// Error from capabilities
	VIR_FROM_CAPABILITIES = C.VIR_FROM_CAPABILITIES

	// Error from URI handling
	VIR_FROM_URI = C.VIR_FROM_URI

	// Error from auth handling
	VIR_FROM_AUTH = C.VIR_FROM_AUTH

	// Error from DBus
	VIR_FROM_DBUS = C.VIR_FROM_DBUS

	// Error from Parallels
	VIR_FROM_PARALLELS = C.VIR_FROM_PARALLELS

	// Error from Device
	VIR_FROM_DEVICE = C.VIR_FROM_DEVICE

	// Error from libssh2 connection transport
	VIR_FROM_SSH = C.VIR_FROM_SSH

	// Error from lockspace
	VIR_FROM_LOCKSPACE = C.VIR_FROM_LOCKSPACE

	// Error from initctl device communication
	VIR_FROM_INITCTL = C.VIR_FROM_INITCTL

	// Error from identity code
	VIR_FROM_IDENTITY = C.VIR_FROM_IDENTITY

	// Error from cgroups
	VIR_FROM_CGROUP = C.VIR_FROM_CGROUP

	// Error from access control manager
	VIR_FROM_ACCESS = C.VIR_FROM_ACCESS

	// Error from systemd code
	VIR_FROM_SYSTEMD = C.VIR_FROM_SYSTEMD

	// Error from bhyve driver
	VIR_FROM_BHYVE = C.VIR_FROM_BHYVE

	// Error from crypto code
	VIR_FROM_CRYPTO = C.VIR_FROM_CRYPTO

	// Error from firewall
	VIR_FROM_FIREWALL = C.VIR_FROM_FIREWALL
)

type VirError struct {
	Code    int
	Domain  int
	Message string
	Level   int
}

func (err VirError) Error() string {
	return fmt.Sprintf("[Code-%d] [Domain-%d] %s",
		err.Code, err.Domain, err.Message)
}
