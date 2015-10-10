package errors

// This file contains all of the errors that can be generated from the
// docker/daemon/execdriver component.

import (
	"net/http"

	"github.com/docker/distribution/registry/api/errcode"
)

var (
	// ErrorCodeExecDriverNotSupported is generated when specified driver is not supported.
	ErrorCodeExecDriverNotSupported = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EXECDRIVERNOTSUPPORTED",
		Message:        "jail driver not yet supported on FreeBSD",
		Description:    "The specified jail driver is not supported",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeExecDriverUnknown is generated when unknown driver is specified.
	ErrorCodeExecDriverUnknown = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EXECDRIVERUNKNOWN",
		Message:        "unknown exec driver %s",
		Description:    "The specified driver is unknown",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeErrSetupUser is generated when unknown driver is specified.
	ErrorCodeErrSetupUser = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "ERRSETUPUSER",
		Message:        "setup user %s",
		Description:    "Failed to setup specified user",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeGetNetNS is generated if get network namespace is failed.
	ErrorCodeGetNetNS = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "GETNETNSFAILED",
		Message:        "failed to get network namespace %q: %v",
		Description:    "Failed to get network namespace.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeSetNetNS is generated if set network namespace is failed.
	ErrorCodeSetNetNS = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "SETiNETNSFAILED",
		Message:        "failed to set network namespace %q: %v",
		Description:    "Failed to set network namespace.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeStartNetNS is generated if start network namespace is failed.
	ErrorCodeStartNetNS = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "STARTNETNSFAILED",
		Message:        "failed to start netns process: %v",
		Description:    "Failed to start network namespace.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeEmptyNetNSPath is generated if path to  network namespace is invalid.
	ErrorCodeEmptyNetNSPath = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EMPTYNETNSPATH",
		Message:        "empty namespace path for non-container network",
		Description:    "Error due to empty path specified to network namespace.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNoPathToMemory is generated if path to CG memory is empty.
	ErrorCodeNoPathToMemory = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOPATHTOMEMORY",
		Message:        "There is no path for %q in state",
		Description:    "Path to CGroup memory, used to notify OOM, is empty.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidDeviceType is generated if device type specified is incorrect.
	ErrorCodeInvalidDeviceType = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDDEVICETYPE",
		Message:        "%c is not a valid device type for device %s",
		Description:    "Device type specifed is invalid.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDevMknodFailed is generated if call to mknod when creating device node returns an error.
	ErrorCodeDevMknodFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DEVMKNODFAILED",
		Message:        "mknod %s %s",
		Description:    "A call to mknod when creating device node has failed",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDevChownFailed is generated if call to chown when creating device node returns an error.
	ErrorCodeDevChownFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DEVCHOWNFAILED",
		Message:        "chown %s to %d:%d",
		Description:    "A call to chown when creating device node has failed",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodePauseLXCFailed is generated when lxc freeze failed with an error.
	ErrorCodePauseLXCFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "PAUSELXCFAILED",
		Message:        "Err: %s Output: %s",
		Description:    "Lxc freeze failed with an error.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeUnpauseLXCFailed is generated when lxc unfreeze failed with an error.
	ErrorCodeUnpauseLXCFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "UNPAUSELXCFAILED",
		Message:        "Err: %s Output: %s",
		Description:    "Lxc unfreeze failed with an error.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeKillLXCFailed is generated when lxc kill/stop failed with an error.
	ErrorCodeKillLXCFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "KILLLXCFAILED",
		Message:        "Err: %s Output: %s",
		Description:    "Lxc stop/kill failed with an error.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidPIDLXC is generated when lxc pid found in tasks file is invalid.
	ErrorCodeInvalidPIDLXC = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDPIDLXC",
		Message:        "Invalid pid '%s': %s",
		Description:    "Lxc pid found in tasks file is invalid",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidIDLXCStats is generated if id is not found among active containers for LXC stats.
	ErrorCodeInvalidIDLXCStats = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDIDLXCSTATS",
		Message:        "%s is not a key in active containers",
		Description:    "ID cannot be found amond active containers for LXC stats.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDockerInitFailed is generated if docker init failed to execute.
	ErrorCodeDockerInitFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DOCKERINITFAILED",
		Message:        "dockerinit unable to execute %s - %s",
		Description:    "Dockerinit is unable to execute.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLoadEnvFailed is generated when lxc start fails to load environment variables.
	ErrorCodeLoadEnvFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOADENVFAILED",
		Message:        "Unable to load environment variables: %v",
		Description:    "Lxc start fails to load environment variables",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDecodeEnvFailed is generated when lxc start fails to decode environment variables.
	ErrorCodeDecodeEnvFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DECODEENVFAILED",
		Message:        "Unable to decode environment variables: %v",
		Description:    "Lxc start fails to decode environment variables",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeCDWorkDirFailed is generated if unable to change to working directory.
	ErrorCodeCDWorkDirFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CDWORKDIRFAILED",
		Message:        "Unable to change dir to %v: %v",
		Description:    "Unable to change to working directory",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNativeDriverNotSupported is generated because native driver is not supported on non-linux.
	ErrorCodeNativeDriverNotSupported = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NATIVEDRIVERNOTSUPPORTED",
		Message:        "native driver not supported on non-linux",
		Description:    "Native driver is not supported on non-linux",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNoContainerWithID is generated if unable to find a container with given id.
	ErrorCodeNoContainerWithID = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOCONTAINERWITHID",
		Message:        "No active container exists with ID %s",
		Description:    "Unable to find a container with given id",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvContainerWithID is generated if unable to join a container with given id.
	ErrorCodeInvContainerWithID = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVCONTAINERWITHID",
		Message:        "%s is not a valid running container to join",
		Description:    "Unable to join a container with given id",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeAppArmorLoadProfileFailed is generated if unable to load AppArmor profile.
	ErrorCodeAppArmorLoadProfileFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "APPARMORLOADPROFILEFAILED",
		Message:        "AppArmor enabled on system but the %s profile could not be loaded.",
		Description:    "Unable to load AppArmor profile",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLoadAppArmorProfileFailed is generated if unable to load AppArmor profile.
	ErrorCodeLoadAppArmorProfileFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOADAPPARMORPROFILEFAILED",
		Message:        "Error loading docker apparmor profile: %s (%s)",
		Description:    "Unable to load AppArmor profile",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeUnknownCGroupDriver is generated if unable to find suitable CGroup driver.
	ErrorCodeUnknownCGroupDriver = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "UNKNOWNCGROUPDRIVER",
		Message:        "Unknown native.cgroupdriver given %q. try cgroupfs or systemd",
		Description:    "Unable to find a suitable CGroup driver",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidDriverOption is generated if option specifed to the native driver is invalid.
	ErrorCodeInvalidDriverOption = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDDRIVEROPTION",
		Message:        "Unknown option %s\n",
		Description:    "Invalid option specified to the native driver",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNoContainerWithID2 is generated if unable to find a container with given id.
	ErrorCodeNoContainerWithID2 = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOCONTAINERWITHID2",
		Message:        "active container for %s does not exist",
		Description:    "Unable to find a container with given id",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeUnknownCapDrop is generated if unable to drop a unknown capability.
	ErrorCodeUnknownCapDrop = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "UNKNOWNCAPDROPFAILED",
		Message:        "Unknown capability drop: %q",
		Description:    "Unable to drop a unknown capability",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeUnknownCapAdd is generated if unable to add a unknown capability.
	ErrorCodeUnknownCapAdd = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "UNKNOWNCAPADDFAILED",
		Message:        "Unknown capability to add: %q",
		Description:    "Unable to add a unknown capability",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinStatsNotImpl is generated to indicate stats not implemented on Windows.
	ErrorCodeWinStatsNotImpl = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINSTATSNOTIMPL",
		Message:        "Windows: Stats not implemented",
		Description:    "Indicates that stats not implemented on Windows",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinDriverNotSupported indicates that this driver is not supported on non-Wondows.
	ErrorCodeWinDriverNotSupported = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINDRIVERNOTSUPPORTED",
		Message:        "Windows driver not supported on non-Windows",
		Description:    "Indicates that this driver is not supported on non-Wondows",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinErrPauseUnpause is generated if pause or unpause command issues on a Windows container.
	ErrorCodeWinErrPauseUnpause = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINERRPAUSEUNPAUSE",
		Message:        "Windows: Containers cannot be paused",
		Description:    "Unable to pause or unpause a Windows Container",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinErrExecDriver is generated if unrecognized exec driver is used.
	ErrorCodeWinErrExecDriver = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINERREXECDRIVER",
		Message:        "Unrecognised exec driver option %s\n",
		Description:    "Unable to use the unrecognized exec driver on Windows",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinErrExecID is generated while running exec on a Windows container with a ID.
	ErrorCodeWinErrExecID = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINERREXECID",
		Message:        "Exec - No active container exists with ID %s",
		Description:    "Unable to find a active Windows container to exec using ID",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinErrGetPid is generated to indicate GetPidsForContainer is not implemented on Windows.
	ErrorCodeWinErrGetPid = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINERRGETPID",
		Message:        "GetPidsForContainer: GetPidsForContainer() not implemented",
		Description:    "Indicates that GetPidsForContainer is not implemented on Windows",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinErrInvProtocol is generated invalid network protocol is used on a Windows container.
	ErrorCodeWinErrInvProtocol = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINERRINVPROTOCOL",
		Message:        "invalid protocol %s",
		Description:    "Invalid network protocol is used on a Windows Container",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinErrMultiplePorts is generated if more than on host port is specified on a Windows container.
	ErrorCodeWinErrMultiplePorts = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINERRMULTIPLEPORTS",
		Message:        "Windows does not support more than one host port in NAT settings",
		Description:    "Indicates that mor than on host port is specified on a Windows Container",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinErrInvHostIPInNAT is generated when host ip ios specified in NAT on a Windows container.
	ErrorCodeWinErrInvHostIPInNAT = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINERRINVHOSTIPINNAT",
		Message:        "Windows does not support host IP addresses in NAT settings",
		Description:    "Indicates that it is invalid to specify host ip address in NAT setting on a Windows Container",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinErrInvPort is generated when port is incorrectly formatted.
	ErrorCodeWinErrInvPort = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINERRINVPORT",
		Message:        "invalid container port %s: %s",
		Description:    "Indicates that the port specified in incorrectly formatted and cannot be converted to a numeric value",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinErrInvInternalPort is generated when internal ports is incorrectly formatted.
	ErrorCodeWinErrInvInternalPort = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINERRINVINTERNALPORT",
		Message:        "invalid internal port %s: %s",
		Description:    "Indicates that the internal port is incorrectly formatted and cannot be converted to a numeric value",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinErrInvPortRange is generated when port specifed is not withing allowed range.
	ErrorCodeWinErrInvPortRange = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINERRINVPORTRANGE",
		Message:        "specified NAT port is not in allowed range",
		Description:    "Indicates that the port specified is not within allowed range",
		HTTPStatusCode: http.StatusInternalServerError,
	})
)
