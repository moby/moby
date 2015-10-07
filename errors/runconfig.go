package errors

// This file contains all of the errors that can be generated from the
// docker/runconfig component.

import (
	"net/http"

	"github.com/docker/distribution/registry/api/errcode"
)

var (
	// ErrorCodeInvalidMacAddress is generated when invalid Mac Address is provided.
	ErrorCodeInvalidMacAddress = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDMACADDRESS",
		Message:        "%s is not a valid mac address",
		Description:    "The specified Mac Address is invalid",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidMemorySwappiness is generated when invalid Memory Swappiness value is provided.
	ErrorCodeInvalidMemorySwappiness = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDMEMORYSWAPPINESS",
		Message:        "Invalid value: %d. Valid memory swappiness range is 0-100",
		Description:    "The specified Memory swappiness is invalid",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidBindMount is generated when invalid Bind mount is provided.
	ErrorCodeInvalidBindMount = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDBINDMOUNT",
		Message:        "Invalid bind mount: destination can't be '/'",
		Description:    "The specified Volume Bind Mount is invalid",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidVolume is generated when invalid Volume is provided.
	ErrorCodeInvalidVolume = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDVOLUME",
		Message:        "Invalid volume: path can't be '/'",
		Description:    "The specified Volume path is invalid",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidPortFormat is generated when invalid format used to speficy port mapping.
	ErrorCodeInvalidPortFormat = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDPORTFORMAT",
		Message:        "Invalid port format for --expose: %s",
		Description:    "The specified port format is invalid",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidPortRange is generated when invalid format used to specify port range.
	ErrorCodeInvalidPortRange = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDPORTRANGE",
		Message:        "Invalid range format for --expose: %s, error: %s",
		Description:    "The specified port range is invalid",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidIPCMode is generated when invalid IPC mode is provided.
	ErrorCodeInvalidIPCMode = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDIPCMODE",
		Message:        "--ipc: invalid IPC mode",
		Description:    "The specified IPC Mode is invalid",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidPIDMode is generated when invalid PID mode is provided.
	ErrorCodeInvalidPIDMode = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDPIDMODE",
		Message:        "--pid: invalid PID mode",
		Description:    "The specified PID Mode is invalid",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidUTSMode is generated when invalid UTS mode is provided.
	ErrorCodeInvalidUTSMode = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDUTSMODE",
		Message:        "--uts: invalid UTS mode",
		Description:    "The specified UTS Mode is invalid",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidLogDriverOpts is generated when invalid options specified to log driver.
	ErrorCodeInvalidLogDriverOpts = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDLOGDRIVEROPTS",
		Message:        "Invalid logging opts for driver %s",
		Description:    "The specified options to log driver are invalid",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidRestartCount is generated when invalid count specified with the restart policy.
	ErrorCodeInvalidRestartCount = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDRESTARTCOUNT",
		Message:        "maximum restart count not valid with restart policy of \"%s\"",
		Description:    "The specified count with restart policy is invalid",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidRestartFormat is generated when invalid format use to specify restart count.
	ErrorCodeInvalidRestartFormat = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDRESTARTFORMAT",
		Message:        "restart count format is not valid, usage: 'on-failure:N' or 'on-failure'",
		Description:    "The specified format for count with restart policy is invalid",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidRestartPolicy is generated when invalid restart policy is specified.
	ErrorCodeInvalidRestartPolicy = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDRESTARTPOLICY",
		Message:        "invalid restart policy %s",
		Description:    "The specified restart policy is invalid",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidDeviceSpecs is generated when device specifications are incorrectly specified.
	ErrorCodeInvalidDeviceSpecs = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDDEVICESPEC",
		Message:        "Invalid device specification: %s",
		Description:    "The device specifications are invalid",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidNetworkOption specified network model -net=<mode> is invalid
	ErrorCodeInvalidNetworkOption = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDNETWORKOPTION",
		Message:        "invalid --net: %s",
		Description:    "Network model specified is invalid.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidNetworkFormat specified network model -net=<mode> with container is invalid
	ErrorCodeInvalidNetworkFormat = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDNETWORKFORMAT",
		Message:        "--net: invalid net mode: invalid container format container:<name|id>",
		Description:    "Network model specified with container name/id is invalid.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeConflictContainerNetworkAndLinks conflict between --net=container and links
	ErrorCodeConflictContainerNetworkAndLinks = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CONFLICTCONTAINERNETWORKANDLINKS",
		Message:        "Conflicting options: --net=container can't be used with links. This would result in undefined behavior",
		Description:    "Containers network model cannot be used with links.",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeConflictNetworkAndDNS conflict between --dns and the network mode
	ErrorCodeConflictNetworkAndDNS = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CONFLICTCONTAINERNETWORKANDDNS",
		Message:        "Conflicting options: --dns and the network mode (--net)",
		Description:    "Containers network model cannot be used with DNS option.",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeConflictNetworkHostname conflict between the hostname and the network mode
	ErrorCodeConflictNetworkHostname = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CONFLICTNETWORKHOSTNAME",
		Message:        "Conflicting options: -h and the network mode (--net)",
		Description:    "Network model cannot be used with hostname option.",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeConflictHostNetworkAndLinks conflict between --net=host and links
	ErrorCodeConflictHostNetworkAndLinks = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CONFLICTHOSTNETWORKANDLINKS",
		Message:        "Conflicting options: --net=host can't be used with links. This would result in undefined behavior",
		Description:    "Host network model cannot be used with links option.",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeConflictContainerNetworkAndMac conflict between the mac address and the network mode
	ErrorCodeConflictContainerNetworkAndMac = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CONFLICTCONTAINERNETWORKANDMAC",
		Message:        "Conflicting options: --mac-address and the network mode (--net)",
		Description:    "Containers network model cannot be used with mac address option.",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeConflictNetworkHosts conflict between add-host and the network mode
	ErrorCodeConflictNetworkHosts = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CONFLICTNETWORKHOSTS",
		Message:        "Conflicting options: --add-host and the network mode (--net)",
		Description:    "Network model cannot be used when using host option.",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeConflictNetworkPublishPorts conflict between the publish options and the network mode
	ErrorCodeConflictNetworkPublishPorts = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CONFLICTNETWORKPUBLISHPORTS",
		Message:        "Conflicting options: -p, -P, --publish-all, --publish and the network mode (--net)",
		Description:    "Network model cannot be used with port publish option.",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeConflictNetworkExposePorts conflict between the expose option and the network mode
	ErrorCodeConflictNetworkExposePorts = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CONFLICTNETWORKEXPOSEPORTS",
		Message:        "Conflicting options: --expose and the network mode (--net)",
		Description:    "Network model cannot be used with port expose option.",
		HTTPStatusCode: http.StatusConflict,
	})
)
