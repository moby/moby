package errors

// This file contains all of the errors that can be generated from the
// docker/daemon component.

import (
	"net/http"

	"github.com/docker/distribution/registry/api/errcode"
)

var (
	// ErrorCodeNoSuchContainer is generated when we look for a container by
	// name or ID and we can't find it.
	ErrorCodeNoSuchContainer = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOSUCHCONTAINER",
		Message:        "no such id: %s",
		Description:    "The specified container can not be found",
		HTTPStatusCode: http.StatusNotFound,
	})

	// ErrorCodeUnregisteredContainer is generated when we try to load
	// a storage driver for an unregistered container
	ErrorCodeUnregisteredContainer = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "UNREGISTEREDCONTAINER",
		Message:        "Can't load storage driver for unregistered container %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeContainerBeingRemoved is generated when an attempt to start
	// a container is made but its in the process of being removed, or is dead.
	ErrorCodeContainerBeingRemoved = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CONTAINERBEINGREMOVED",
		Message:        "Container is marked for removal and cannot be started.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeUnpauseContainer is generated when we attempt to stop a
	// container but its paused.
	ErrorCodeUnpauseContainer = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "UNPAUSECONTAINER",
		Message:        "Container %s is paused. Unpause the container before stopping",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeAlreadyPaused is generated when we attempt to pause a
	// container when its already paused.
	ErrorCodeAlreadyPaused = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "ALREADYPAUSED",
		Message:        "Container %s is already paused",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNotPaused is generated when we attempt to unpause a
	// container when its not paused.
	ErrorCodeNotPaused = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOTPAUSED",
		Message:        "Container %s is not paused",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeImageUnregContainer is generated when we attempt to get the
	// image of an unknown/unregistered container.
	ErrorCodeImageUnregContainer = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "IMAGEUNREGCONTAINER",
		Message:        "Can't get image of unregistered container",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeEmptyID is generated when an ID is the emptry string.
	ErrorCodeEmptyID = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EMPTYID",
		Message:        "Invalid empty id",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLoggingFactory is generated when we could not load the
	// log driver.
	ErrorCodeLoggingFactory = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGGINGFACTORY",
		Message:        "Failed to get logging factory: %v",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInitLogger is generated when we could not initialize
	// the logging driver.
	ErrorCodeInitLogger = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INITLOGGER",
		Message:        "Failed to initialize logging driver: %v",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNotRunning is generated when we need to verify that
	// a container is running, but its not.
	ErrorCodeNotRunning = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOTRUNNING",
		Message:        "Container %s is not running",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLinkNotRunning is generated when we try to link to a
	// container that is not running.
	ErrorCodeLinkNotRunning = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LINKNOTRUNNING",
		Message:        "Cannot link to a non running container: %s AS %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDeviceInfo is generated when there is an error while trying
	// to get info about a custom device.
	// container that is not running.
	ErrorCodeDeviceInfo = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DEVICEINFO",
		Message:        "error gathering device information while adding custom device %q: %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeEmptyEndpoint is generated when the endpoint for a port
	// map is nil.
	ErrorCodeEmptyEndpoint = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EMPTYENDPOINT",
		Message:        "invalid endpoint while building port map info",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeEmptyNetwork is generated when the networkSettings for a port
	// map is nil.
	ErrorCodeEmptyNetwork = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EMPTYNETWORK",
		Message:        "invalid networksettings while building port map info",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeParsingPort is generated when there is an error parsing
	// a "port" string.
	ErrorCodeParsingPort = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "PARSINGPORT",
		Message:        "Error parsing Port value(%v):%v",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNoSandbox is generated when we can't find the specified
	// sandbox(network) by ID.
	ErrorCodeNoSandbox = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOSANDBOX",
		Message:        "error locating sandbox id %s: %v",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNetworkUpdate is generated when there is an error while
	// trying update a network/sandbox config.
	ErrorCodeNetworkUpdate = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NETWORKUPDATE",
		Message:        "Update network failed: %v",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNetworkRefresh is generated when there is an error while
	// trying refresh a network/sandbox config.
	ErrorCodeNetworkRefresh = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NETWORKREFRESH",
		Message:        "Update network failed: Failure in refresh sandbox %s: %v",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeHostPort is generated when there was an error while trying
	// to parse a "host/por" string.
	ErrorCodeHostPort = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "HOSTPORT",
		Message:        "Error parsing HostPort value(%s):%v",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNetworkConflict is generated when we try to public a service
	// in network mode.
	ErrorCodeNetworkConflict = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NETWORKCONFLICT",
		Message:        "conflicting options: publishing a service and network mode",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeJoinInfo is generated when we failed to update a container's
	// join info.
	ErrorCodeJoinInfo = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "JOININFO",
		Message:        "Updating join info failed: %v",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeIPCRunning is generated when we try to join a container's
	// IPC but its running.
	ErrorCodeIPCRunning = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "IPCRUNNING",
		Message:        "cannot join IPC of a non running container: %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNotADir is generated when we try to create a directory
	// but the path isn't a dir.
	ErrorCodeNotADir = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOTADIR",
		Message:        "Cannot mkdir: %s is not a directory",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeParseContainer is generated when the reference to a
	// container doesn't include a ":" (another container).
	ErrorCodeParseContainer = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "PARSECONTAINER",
		Message:        "no container specified to join network",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeJoinSelf is generated when we try to network to ourselves.
	ErrorCodeJoinSelf = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "JOINSELF",
		Message:        "cannot join own network",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeJoinRunning is generated when we try to network to ourselves.
	ErrorCodeJoinRunning = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "JOINRUNNING",
		Message:        "cannot join network of a non running container: %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeModeNotContainer is generated when we try to network to
	// another container but the mode isn't 'container'.
	ErrorCodeModeNotContainer = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "MODENOTCONTAINER",
		Message:        "network mode not set to container",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeRemovingVolume is generated when we try remove a mount
	// point (volume) but fail.
	ErrorCodeRemovingVolume = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "REMOVINGVOLUME",
		Message:        "Error removing volumes:\n%v",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidNetworkMode is generated when an invalid network
	// mode value is specified.
	ErrorCodeInvalidNetworkMode = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDNETWORKMODE",
		Message:        "invalid network mode: %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeGetGraph is generated when there was an error while
	// trying to find a graph/image.
	ErrorCodeGetGraph = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "GETGRAPH",
		Message:        "Failed to graph.Get on ImageID %s - %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeGetLayer is generated when there was an error while
	// trying to retrieve a particular layer of an image.
	ErrorCodeGetLayer = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "GETLAYER",
		Message:        "Failed to get layer path from graphdriver %s for ImageID %s - %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodePutLayer is generated when there was an error while
	// trying to 'put' a particular layer of an image.
	ErrorCodePutLayer = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "PUTLAYER",
		Message:        "Failed to put layer path from graphdriver %s for ImageID %s - %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeGetLayerMetadata is generated when there was an error while
	// trying to retrieve the metadata of a layer of an image.
	ErrorCodeGetLayerMetadata = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "GETLAYERMETADATA",
		Message:        "Failed to get layer metadata - %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeEmptyConfig is generated when the input config data
	// is empty.
	ErrorCodeEmptyConfig = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EMPTYCONFIG",
		Message:        "Config cannot be empty in order to create a container",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNoSuchImageHash is generated when we can't find the
	// specified image by its hash
	ErrorCodeNoSuchImageHash = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOSUCHIMAGEHASH",
		Message:        "No such image: %s",
		HTTPStatusCode: http.StatusNotFound,
	})

	// ErrorCodeNoSuchImageTag is generated when we can't find the
	// specified image byt its name/tag.
	ErrorCodeNoSuchImageTag = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOSUCHIMAGETAG",
		Message:        "No such image: %s:%s",
		HTTPStatusCode: http.StatusNotFound,
	})

	// ErrorCodeMountOverFile is generated when we try to mount a volume
	// over an existing file (but not a dir).
	ErrorCodeMountOverFile = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "MOUNTOVERFILE",
		Message:        "cannot mount volume over existing file, file exists %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeMountSetup is generated when we can't define a mount point
	// due to the source and destination are defined.
	ErrorCodeMountSetup = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "MOUNTSETUP",
		Message:        "Unable to setup mount point, neither source nor volume defined",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeVolumeInvalidMode is generated when we the mode of a volume
	// mount is invalid.
	ErrorCodeVolumeInvalidMode = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "VOLUMEINVALIDMODE",
		Message:        "invalid mode for volumes-from: %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeVolumeInvalid is generated when the format fo the
	// volume specification isn't valid.
	ErrorCodeVolumeInvalid = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "VOLUMEINVALID",
		Message:        "Invalid volume specification: %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeVolumeAbs is generated when path to a volume isn't absolute.
	ErrorCodeVolumeAbs = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "VOLUMEABS",
		Message:        "Invalid volume destination path: %s mount path must be absolute.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeVolumeFromBlank is generated when path to a volume is blank.
	ErrorCodeVolumeFromBlank = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "VOLUMEFROMBLANK",
		Message:        "malformed volumes-from specification: %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeVolumeMode is generated when 'mode' for a volume
	// isn't a valid.
	ErrorCodeVolumeMode = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "VOLUMEMODE",
		Message:        "invalid mode for volumes-from: %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeVolumeDup is generated when we try to mount two volumes
	// to the same path.
	ErrorCodeVolumeDup = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "VOLUMEDUP",
		Message:        "Duplicate bind mount %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeCantUnpause is generated when there's an error while trying
	// to unpause a container.
	ErrorCodeCantUnpause = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CANTUNPAUSE",
		Message:        "Cannot unpause container %s: %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodePSError is generated when trying to run 'ps'.
	ErrorCodePSError = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "PSError",
		Message:        "Error running ps: %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNoPID is generated when looking for the PID field in the
	// ps output.
	ErrorCodeNoPID = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOPID",
		Message:        "Couldn't find PID field in ps output",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeBadPID is generated when we can't convert a PID to an int.
	ErrorCodeBadPID = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "BADPID",
		Message:        "Unexpected pid '%s': %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNoTop is generated when we try to run 'top' but can't
	// because we're on windows.
	ErrorCodeNoTop = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOTOP",
		Message:        "Top is not supported on Windows",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeStopped is generated when we try to stop a container
	// that is already stopped.
	ErrorCodeStopped = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "STOPPED",
		Message:        "Container already stopped",
		HTTPStatusCode: http.StatusNotModified,
	})

	// ErrorCodeCantStop is generated when we try to stop a container
	// but failed for some reason.
	ErrorCodeCantStop = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CANTSTOP",
		Message:        "Cannot stop container %s: %s\n",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeBadCPUFields is generated the number of CPU fields is
	// less than 8.
	ErrorCodeBadCPUFields = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "BADCPUFIELDS",
		Message:        "invalid number of cpu fields",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeBadCPUInt is generated the CPU field can't be parsed as an int.
	ErrorCodeBadCPUInt = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "BADCPUINT",
		Message:        "Unable to convert value %s to int: %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeBadStatFormat is generated the output of the stat info
	// isn't parseable.
	ErrorCodeBadStatFormat = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "BADSTATFORMAT",
		Message:        "invalid stat format",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeTimedOut is generated when a timer expires.
	ErrorCodeTimedOut = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "TIMEDOUT",
		Message:        "Timed out: %v",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeAlreadyRemoving is generated when we try to remove a
	// container that is already being removed.
	ErrorCodeAlreadyRemoving = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "ALREADYREMOVING",
		Message:        "Status is already RemovalInProgress",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeStartPaused is generated when we start a paused container.
	ErrorCodeStartPaused = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "STARTPAUSED",
		Message:        "Cannot start a paused container, try unpause instead.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeAlreadyStarted is generated when we try to start a container
	// that is already running.
	ErrorCodeAlreadyStarted = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "ALREADYSTARTED",
		Message:        "Container already started",
		HTTPStatusCode: http.StatusNotModified,
	})

	// ErrorCodeHostConfigStart is generated when a HostConfig is passed
	// into the start command.
	ErrorCodeHostConfigStart = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "HOSTCONFIGSTART",
		Message:        "Supplying a hostconfig on start is not supported. It should be supplied on create",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeCantStart is generated when an error occurred while
	// trying to start a container.
	ErrorCodeCantStart = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CANTSTART",
		Message:        "Cannot start container %s: %s",
		HTTPStatusCode: http.StatusInternalServerError,
	})
)
