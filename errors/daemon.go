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
		Description:    "An attempt was made to load the storage driver for a container that is not registered with the daemon",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeContainerBeingRemoved is generated when an attempt to start
	// a container is made but its in the process of being removed, or is dead.
	ErrorCodeContainerBeingRemoved = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CONTAINERBEINGREMOVED",
		Message:        "Container is marked for removal and cannot be started.",
		Description:    "An attempt was made to start a container that is in the process of being deleted",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeUnpauseContainer is generated when we attempt to stop a
	// container but its paused.
	ErrorCodeUnpauseContainer = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "UNPAUSECONTAINER",
		Message:        "Container %s is paused. Unpause the container before stopping",
		Description:    "The specified container is paused, before it can be stopped it must be unpaused",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeAlreadyPaused is generated when we attempt to pause a
	// container when its already paused.
	ErrorCodeAlreadyPaused = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "ALREADYPAUSED",
		Message:        "Container %s is already paused",
		Description:    "The specified container is already in the paused state",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNotPaused is generated when we attempt to unpause a
	// container when its not paused.
	ErrorCodeNotPaused = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOTPAUSED",
		Message:        "Container %s is not paused",
		Description:    "The specified container can not be unpaused because it is not in a paused state",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeImageUnregContainer is generated when we attempt to get the
	// image of an unknown/unregistered container.
	ErrorCodeImageUnregContainer = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "IMAGEUNREGCONTAINER",
		Message:        "Can't get image of unregistered container",
		Description:    "An attempt to retrieve the image of a container was made but the container is not registered",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeEmptyID is generated when an ID is the emptry string.
	ErrorCodeEmptyID = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EMPTYID",
		Message:        "Invalid empty id",
		Description:    "An attempt was made to register a container but the container's ID can not be an empty string",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLoggingFactory is generated when we could not load the
	// log driver.
	ErrorCodeLoggingFactory = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGGINGFACTORY",
		Message:        "Failed to get logging factory: %v",
		Description:    "An attempt was made to register a container but the container's ID can not be an empty string",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInitLogger is generated when we could not initialize
	// the logging driver.
	ErrorCodeInitLogger = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INITLOGGER",
		Message:        "Failed to initialize logging driver: %v",
		Description:    "An error occurred while trying to initialize the logging driver",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNotRunning is generated when we need to verify that
	// a container is running, but its not.
	ErrorCodeNotRunning = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOTRUNNING",
		Message:        "Container %s is not running",
		Description:    "The specified action can not be taken due to the container not being in a running state",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLinkNotRunning is generated when we try to link to a
	// container that is not running.
	ErrorCodeLinkNotRunning = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LINKNOTRUNNING",
		Message:        "Cannot link to a non running container: %s AS %s",
		Description:    "An attempt was made to link to a container but the container is not in a running state",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDeviceInfo is generated when there is an error while trying
	// to get info about a custom device.
	// container that is not running.
	ErrorCodeDeviceInfo = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DEVICEINFO",
		Message:        "error gathering device information while adding custom device %q: %s",
		Description:    "There was an error while trying to retrieve the information about a custom device",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeEmptyEndpoint is generated when the endpoint for a port
	// map is nil.
	ErrorCodeEmptyEndpoint = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EMPTYENDPOINT",
		Message:        "invalid endpoint while building port map info",
		Description:    "The specified endpoint for the port mapping is empty",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeEmptyNetwork is generated when the networkSettings for a port
	// map is nil.
	ErrorCodeEmptyNetwork = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EMPTYNETWORK",
		Message:        "invalid networksettings while building port map info",
		Description:    "The specified endpoint for the port mapping is empty",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeParsingPort is generated when there is an error parsing
	// a "port" string.
	ErrorCodeParsingPort = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "PARSINGPORT",
		Message:        "Error parsing Port value(%v):%v",
		Description:    "There was an error while trying to parse the specified 'port' value",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNoSandbox is generated when we can't find the specified
	// sandbox(network) by ID.
	ErrorCodeNoSandbox = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOSANDBOX",
		Message:        "error locating sandbox id %s: %v",
		Description:    "There was an error trying to located the specified networking sandbox",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNetworkUpdate is generated when there is an error while
	// trying update a network/sandbox config.
	ErrorCodeNetworkUpdate = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NETWORKUPDATE",
		Message:        "Update network failed: %v",
		Description:    "There was an error trying to update the configuration information of the specified network sandbox",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNetworkRefresh is generated when there is an error while
	// trying refresh a network/sandbox config.
	ErrorCodeNetworkRefresh = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NETWORKREFRESH",
		Message:        "Update network failed: Failure in refresh sandbox %s: %v",
		Description:    "There was an error trying to refresh the configuration information of the specified network sandbox",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeHostPort is generated when there was an error while trying
	// to parse a "host/port" string.
	ErrorCodeHostPort = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "HOSTPORT",
		Message:        "Error parsing HostPort value(%s):%v",
		Description:    "There was an error trying to parse the specified 'HostPort' value",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNetworkConflict is generated when we try to publish a service
	// in network mode.
	ErrorCodeNetworkConflict = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NETWORKCONFLICT",
		Message:        "conflicting options: publishing a service and network mode",
		Description:    "It is not possible to publish a service when it is in network mode",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeJoinInfo is generated when we failed to update a container's
	// join info.
	ErrorCodeJoinInfo = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "JOININFO",
		Message:        "Updating join info failed: %v",
		Description:    "There was an error during an attempt update a container's join information",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeIPCRunning is generated when we try to join a container's
	// IPC but its not running.
	ErrorCodeIPCRunning = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "IPCRUNNING",
		Message:        "cannot join IPC of a non running container: %s",
		Description:    "An attempt was made to join the IPC of a container, but the container is not running",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNotADir is generated when we try to create a directory
	// but the path isn't a dir.
	ErrorCodeNotADir = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOTADIR",
		Message:        "Cannot mkdir: %s is not a directory",
		Description:    "An attempt was made create a directory, but the location in which it is being created is not a directory",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeParseContainer is generated when the reference to a
	// container doesn't include a ":" (another container).
	ErrorCodeParseContainer = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "PARSECONTAINER",
		Message:        "no container specified to join network",
		Description:    "The specified reference to a container is missing a ':' as a separator between 'container' and 'name'/'id'",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeJoinSelf is generated when we try to network to ourselves.
	ErrorCodeJoinSelf = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "JOINSELF",
		Message:        "cannot join own network",
		Description:    "An attempt was made to have a container join its own network",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeJoinRunning is generated when we try to network to ourselves.
	ErrorCodeJoinRunning = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "JOINRUNNING",
		Message:        "cannot join network of a non running container: %s",
		Description:    "An attempt to join the network of a container, but that container isn't running",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeModeNotContainer is generated when we try to network to
	// another container but the mode isn't 'container'.
	ErrorCodeModeNotContainer = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "MODENOTCONTAINER",
		Message:        "network mode not set to container",
		Description:    "An attempt was made to connect to a container's network but the mode wasn't set to 'container'",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeRemovingVolume is generated when we try remove a mount
	// point (volume) but fail.
	ErrorCodeRemovingVolume = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "REMOVINGVOLUME",
		Message:        "Error removing volumes:\n%v",
		Description:    "There was an error while trying to remove the mount point (volume) of a container",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidNetworkMode is generated when an invalid network
	// mode value is specified.
	ErrorCodeInvalidNetworkMode = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDNETWORKMODE",
		Message:        "invalid network mode: %s",
		Description:    "The specified networking mode is not valid",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeGetGraph is generated when there was an error while
	// trying to find a graph/image.
	ErrorCodeGetGraph = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "GETGRAPH",
		Message:        "Failed to graph.Get on ImageID %s - %s",
		Description:    "There was an error trying to retrieve the image for the specified image ID",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeGetLayer is generated when there was an error while
	// trying to retrieve a particular layer of an image.
	ErrorCodeGetLayer = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "GETLAYER",
		Message:        "Failed to get layer path from graphdriver %s for ImageID %s - %s",
		Description:    "There was an error trying to retrieve the layer of the specified image",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodePutLayer is generated when there was an error while
	// trying to 'put' a particular layer of an image.
	ErrorCodePutLayer = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "PUTLAYER",
		Message:        "Failed to put layer path from graphdriver %s for ImageID %s - %s",
		Description:    "There was an error trying to store a layer for the specified image",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeGetLayerMetadata is generated when there was an error while
	// trying to retrieve the metadata of a layer of an image.
	ErrorCodeGetLayerMetadata = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "GETLAYERMETADATA",
		Message:        "Failed to get layer metadata - %s",
		Description:    "There was an error trying to retrieve the metadata of a layer for the specified image",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeEmptyConfig is generated when the input config data
	// is empty.
	ErrorCodeEmptyConfig = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EMPTYCONFIG",
		Message:        "Config cannot be empty in order to create a container",
		Description:    "While trying to create a container, the specified configuration information was empty",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNoSuchImageHash is generated when we can't find the
	// specified image by its hash
	ErrorCodeNoSuchImageHash = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOSUCHIMAGEHASH",
		Message:        "No such image: %s",
		Description:    "An attempt was made to find an image by its hash, but the lookup failed",
		HTTPStatusCode: http.StatusNotFound,
	})

	// ErrorCodeNoSuchImageTag is generated when we can't find the
	// specified image byt its name/tag.
	ErrorCodeNoSuchImageTag = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOSUCHIMAGETAG",
		Message:        "No such image: %s:%s",
		Description:    "An attempt was made to find an image by its name/tag, but the lookup failed",
		HTTPStatusCode: http.StatusNotFound,
	})

	// ErrorCodeMountOverFile is generated when we try to mount a volume
	// over an existing file (but not a dir).
	ErrorCodeMountOverFile = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "MOUNTOVERFILE",
		Message:        "cannot mount volume over existing file, file exists %s",
		Description:    "An attempt was made to mount a volume at the same location as a pre-existing file",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeMountSetup is generated when we can't define a mount point
	// due to the source and destination being undefined.
	ErrorCodeMountSetup = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "MOUNTSETUP",
		Message:        "Unable to setup mount point, neither source nor volume defined",
		Description:    "An attempt was made to setup a mount point, but the source and destination are undefined",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeVolumeInvalidMode is generated when we the mode of a volume
	// mount is invalid.
	ErrorCodeVolumeInvalidMode = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "VOLUMEINVALIDMODE",
		Message:        "invalid mode for volumes-from: %s",
		Description:    "An invalid 'mode' was specified in the mount request",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeVolumeInvalid is generated when the format fo the
	// volume specification isn't valid.
	ErrorCodeVolumeInvalid = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "VOLUMEINVALID",
		Message:        "Invalid volume specification: %s",
		Description:    "An invalid 'volume' was specified in the mount request",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeVolumeAbs is generated when path to a volume isn't absolute.
	ErrorCodeVolumeAbs = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "VOLUMEABS",
		Message:        "Invalid volume destination path: %s mount path must be absolute.",
		Description:    "An invalid 'destination' path was specified in the mount request, it must be an absolute path",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeVolumeName is generated when the name of named volume isn't valid.
	ErrorCodeVolumeName = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "VOLUME_NAME_INVALID",
		Message:        "%s includes invalid characters for a local volume name, only %s are allowed",
		Description:    "The name of volume is invalid",
		HTTPStatusCode: http.StatusBadRequest,
	})

	// ErrorCodeVolumeFromBlank is generated when path to a volume is blank.
	ErrorCodeVolumeFromBlank = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "VOLUMEFROMBLANK",
		Message:        "malformed volumes-from specification: %s",
		Description:    "An invalid 'destination' path was specified in the mount request, it must not be blank",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeVolumeMode is generated when 'mode' for a volume
	// isn't a valid.
	ErrorCodeVolumeMode = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "VOLUMEMODE",
		Message:        "invalid mode for volumes-from: %s",
		Description:    "An invalid 'mode' path was specified in the mount request",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeVolumeDup is generated when we try to mount two volumes
	// to the same path.
	ErrorCodeVolumeDup = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "VOLUMEDUP",
		Message:        "Duplicate bind mount %s",
		Description:    "An attempt was made to mount a volume but the specified destination location is already used in a previous mount",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeCantUnpause is generated when there's an error while trying
	// to unpause a container.
	ErrorCodeCantUnpause = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CANTUNPAUSE",
		Message:        "Cannot unpause container %s: %s",
		Description:    "An error occurred while trying to unpause the specified container",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodePSError is generated when trying to run 'ps'.
	ErrorCodePSError = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "PSError",
		Message:        "Error running ps: %s",
		Description:    "There was an error trying to run the 'ps' command in the specified container",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNoPID is generated when looking for the PID field in the
	// ps output.
	ErrorCodeNoPID = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOPID",
		Message:        "Couldn't find PID field in ps output",
		Description:    "There was no 'PID' field in the output of the 'ps' command that was executed",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeBadPID is generated when we can't convert a PID to an int.
	ErrorCodeBadPID = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "BADPID",
		Message:        "Unexpected pid '%s': %s",
		Description:    "While trying to parse the output of the 'ps' command, the 'PID' field was not an integer",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNoTop is generated when we try to run 'top' but can't
	// because we're on windows.
	ErrorCodeNoTop = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOTOP",
		Message:        "Top is not supported on Windows",
		Description:    "The 'top' command is not supported on Windows",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeStopped is generated when we try to stop a container
	// that is already stopped.
	ErrorCodeStopped = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "STOPPED",
		Message:        "Container already stopped",
		Description:    "An attempt was made to stop a container, but the container is already stopped",
		HTTPStatusCode: http.StatusNotModified,
	})

	// ErrorCodeCantStop is generated when we try to stop a container
	// but failed for some reason.
	ErrorCodeCantStop = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CANTSTOP",
		Message:        "Cannot stop container %s: %s\n",
		Description:    "An error occurred while tring to stop the specified container",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeBadCPUFields is generated when the number of CPU fields is
	// less than 8.
	ErrorCodeBadCPUFields = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "BADCPUFIELDS",
		Message:        "invalid number of cpu fields",
		Description:    "While reading the '/proc/stat' file, the number of 'cpu' fields is less than 8",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeBadCPUInt is generated the CPU field can't be parsed as an int.
	ErrorCodeBadCPUInt = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "BADCPUINT",
		Message:        "Unable to convert value %s to int: %s",
		Description:    "While reading the '/proc/stat' file, the 'CPU' field could not be parsed as an integer",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeBadStatFormat is generated the output of the stat info
	// isn't parseable.
	ErrorCodeBadStatFormat = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "BADSTATFORMAT",
		Message:        "invalid stat format",
		Description:    "There was an error trying to parse the '/proc/stat' file",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeTimedOut is generated when a timer expires.
	ErrorCodeTimedOut = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "TIMEDOUT",
		Message:        "Timed out: %v",
		Description:    "A timer expired",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeAlreadyRemoving is generated when we try to remove a
	// container that is already being removed.
	ErrorCodeAlreadyRemoving = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "ALREADYREMOVING",
		Message:        "Status is already RemovalInProgress",
		Description:    "An attempt to remove a container was made, but the container is already in the process of being removed",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeStartPaused is generated when we start a paused container.
	ErrorCodeStartPaused = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "STARTPAUSED",
		Message:        "Cannot start a paused container, try unpause instead.",
		Description:    "An attempt to start a container was made, but the container is paused. Unpause it first",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeAlreadyStarted is generated when we try to start a container
	// that is already running.
	ErrorCodeAlreadyStarted = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "ALREADYSTARTED",
		Message:        "Container already started",
		Description:    "An attempt to start a container was made, but the container is already started",
		HTTPStatusCode: http.StatusNotModified,
	})

	// ErrorCodeHostConfigStart is generated when a HostConfig is passed
	// into the start command.
	ErrorCodeHostConfigStart = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "HOSTCONFIGSTART",
		Message:        "Supplying a hostconfig on start is not supported. It should be supplied on create",
		Description:    "The 'start' command does not accept 'HostConfig' data, try using the 'create' command instead",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeCantStart is generated when an error occurred while
	// trying to start a container.
	ErrorCodeCantStart = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CANTSTART",
		Message:        "Cannot start container %s: %s",
		Description:    "There was an error while trying to start a container",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeCantRestart is generated when an error occurred while
	// trying to restart a container.
	ErrorCodeCantRestart = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CANTRESTART",
		Message:        "Cannot restart container %s: %s",
		Description:    "There was an error while trying to restart a container",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeEmptyRename is generated when one of the names on a
	// rename is empty.
	ErrorCodeEmptyRename = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EMPTYRENAME",
		Message:        "Neither old nor new names may be empty",
		Description:    "An attempt was made to rename a container but either the old or new names were blank",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeRenameTaken is generated when we try to rename but the
	// new name isn't available.
	ErrorCodeRenameTaken = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "RENAMETAKEN",
		Message:        "Error when allocating new name: %s",
		Description:    "The new name specified on the 'rename' command is already being used",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeRenameDelete is generated when we try to rename but
	// failed trying to delete the old container.
	ErrorCodeRenameDelete = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "RENAMEDELETE",
		Message:        "Failed to delete container %q: %v",
		Description:    "There was an error trying to delete the specified container",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodePauseError is generated when we try to pause a container
	// but failed.
	ErrorCodePauseError = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "PAUSEERROR",
		Message:        "Cannot pause container %s: %s",
		Description:    "There was an error trying to pause the specified container",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNeedStream is generated when we try to stream a container's
	// logs but no output stream was specified.
	ErrorCodeNeedStream = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NEEDSTREAM",
		Message:        "You must choose at least one stream",
		Description:    "While trying to stream a container's logs, no output stream was specified",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDanglingOne is generated when we try to specify more than one
	// 'dangling' specifier.
	ErrorCodeDanglingOne = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DANLGINGONE",
		Message:        "Conflict: cannot use more than 1 value for `dangling` filter",
		Description:    "The specified 'dangling' filter may not have more than one value",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeImgDelUsed is generated when we try to delete an image
	// but it is being used.
	ErrorCodeImgDelUsed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "IMGDELUSED",
		Message:        "conflict: unable to remove repository reference %q (must force) - container %s is using its referenced image %s",
		Description:    "An attempt was made to delete an image but it is currently being used",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeImgNoParent is generated when we try to find an image's
	// parent but its not in the graph.
	ErrorCodeImgNoParent = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "IMGNOPARENT",
		Message:        "unable to get parent image: %v",
		Description:    "There was an error trying to find an image's parent, it was not in the graph",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeExportFailed is generated when an export fails.
	ErrorCodeExportFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EXPORTFAILED",
		Message:        "%s: %s",
		Description:    "There was an error during an export operation",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeExecResize is generated when we try to resize an exec
	// but its not running.
	ErrorCodeExecResize = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EXECRESIZE",
		Message:        "Exec %s is not running, so it can not be resized.",
		Description:    "An attempt was made to resize an 'exec', but the 'exec' is not running",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeContainerNotRunning is generated when we try to get the info
	// on an exec but the container is not running.
	ErrorCodeContainerNotRunning = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CONTAINERNOTRUNNING",
		Message:        "Container %s is not running: %s",
		Description:    "An attempt was made to retrieve the information about an 'exec' but the container is not running",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeNoExecID is generated when we try to get the info
	// on an exec but it can't be found.
	ErrorCodeNoExecID = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOEXECID",
		Message:        "No such exec instance '%s' found in daemon",
		Description:    "The specified 'exec' instance could not be found",
		HTTPStatusCode: http.StatusNotFound,
	})

	// ErrorCodeExecPaused is generated when we try to start an exec
	// but the container is paused.
	ErrorCodeExecPaused = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EXECPAUSED",
		Message:        "Container %s is paused, unpause the container before exec",
		Description:    "An attempt to start an 'exec' was made, but the owning container is paused",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeExecRunning is generated when we try to start an exec
	// but its already running.
	ErrorCodeExecRunning = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EXECRUNNING",
		Message:        "Error: Exec command %s is already running",
		Description:    "An attempt to start an 'exec' was made, but 'exec' is already running",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeExecCantRun is generated when we try to start an exec
	// but it failed for some reason.
	ErrorCodeExecCantRun = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EXECCANTRUN",
		Message:        "Cannot run exec command %s in container %s: %s",
		Description:    "An attempt to start an 'exec' was made, but an error occurred",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeExecAttach is generated when we try to attach to an exec
	// but failed.
	ErrorCodeExecAttach = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EXECATTACH",
		Message:        "attach failed with error: %s",
		Description:    "There was an error while trying to attach to an 'exec'",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeExecContainerStopped is generated when we try to start
	// an exec but then the container stopped.
	ErrorCodeExecContainerStopped = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EXECCONTAINERSTOPPED",
		Message:        "container stopped while running exec",
		Description:    "An attempt was made to start an 'exec' but the owning container is in the 'stopped' state",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDefaultName is generated when we try to delete the
	// default name of a container.
	ErrorCodeDefaultName = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DEFAULTNAME",
		Message:        "Conflict, cannot remove the default name of the container",
		Description:    "An attempt to delete the default name of a container was made, but that is not allowed",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeNoParent is generated when we try to delete a container
	// but we can't find its parent image.
	ErrorCodeNoParent = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOPARENT",
		Message:        "Cannot get parent %s for name %s",
		Description:    "An attempt was made to delete a container but its parent image could not be found",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeCantDestroy is generated when we try to delete a container
	// but failed for some reason.
	ErrorCodeCantDestroy = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CANTDESTROY",
		Message:        "Cannot destroy container %s: %v",
		Description:    "An attempt was made to delete a container but it failed",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeRmRunning is generated when we try to delete a container
	// but its still running.
	ErrorCodeRmRunning = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "RMRUNNING",
		Message:        "Conflict, You cannot remove a running container. Stop the container before attempting removal or use -f",
		Description:    "An attempt was made to delete a container but the container is still running, try to either stop it first or use '-f'",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeRmFailed is generated when we try to delete a container
	// but it failed for some reason.
	ErrorCodeRmFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "RMFAILED",
		Message:        "Could not kill running container, cannot remove - %v",
		Description:    "An error occurred while trying to delete a running container",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeRmNotFound is generated when we try to delete a container
	// but couldn't find it.
	ErrorCodeRmNotFound = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "RMNOTFOUND",
		Message:        "Could not kill running container, cannot remove - %v",
		Description:    "An attempt to delete a container was made but the container could not be found",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeRmState is generated when we try to delete a container
	// but couldn't set its state to RemovalInProgress.
	ErrorCodeRmState = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "RMSTATE",
		Message:        "Failed to set container state to RemovalInProgress: %s",
		Description:    "An attempt to delete a container was made, but there as an error trying to set its state to 'RemovalInProgress'",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeRmDriverFS is generated when we try to delete a container
	// but the driver failed to delete its filesystem.
	ErrorCodeRmDriverFS = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "RMDRIVERFS",
		Message:        "Driver %s failed to remove root filesystem %s: %s",
		Description:    "While trying to delete a container, the driver failed to remove the root filesystem",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeRmInit is generated when we try to delete a container
	// but failed deleting its init filesystem.
	ErrorCodeRmInit = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "RMINIT",
		Message:        "Driver %s failed to remove init filesystem %s: %s",
		Description:    "While trying to delete a container, the driver failed to remove the init filesystem",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeRmFS is generated when we try to delete a container
	// but failed deleting its filesystem.
	ErrorCodeRmFS = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "RMFS",
		Message:        "Unable to remove filesystem for %v: %v",
		Description:    "While trying to delete a container, the driver failed to remove the filesystem",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeRmExecDriver is generated when we try to delete a container
	// but failed deleting its exec driver data.
	ErrorCodeRmExecDriver = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "RMEXECDRIVER",
		Message:        "Unable to remove execdriver data for %s: %s",
		Description:    "While trying to delete a container, there was an error trying to remove th exec driver data",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeRmVolumeInUse is generated when we try to delete a container
	// but failed deleting a volume because its being used.
	ErrorCodeRmVolumeInUse = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "RMVOLUMEINUSE",
		Message:        "Conflict: %v",
		Description:    "While trying to delete a container, one of its volumes is still being used",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeRmVolume is generated when we try to delete a container
	// but failed deleting a volume.
	ErrorCodeRmVolume = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "RMVOLUME",
		Message:        "Error while removing volume %s: %v",
		Description:    "While trying to delete a container, there was an error trying to delete one of its volumes",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidCpusetCpus is generated when user provided cpuset CPUs
	// are invalid.
	ErrorCodeInvalidCpusetCpus = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDCPUSETCPUS",
		Message:        "Invalid value %s for cpuset cpus.",
		Description:    "While verifying the container's 'HostConfig', CpusetCpus value was in an incorrect format",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeInvalidCpusetMems is generated when user provided cpuset mems
	// are invalid.
	ErrorCodeInvalidCpusetMems = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "INVALIDCPUSETMEMS",
		Message:        "Invalid value %s for cpuset mems.",
		Description:    "While verifying the container's 'HostConfig', CpusetMems value was in an incorrect format",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNotAvailableCpusetCpus is generated when user provided cpuset
	// CPUs aren't available in the container's cgroup.
	ErrorCodeNotAvailableCpusetCpus = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOTAVAILABLECPUSETCPUS",
		Message:        "Requested CPUs are not available - requested %s, available: %s.",
		Description:    "While verifying the container's 'HostConfig', cpuset CPUs provided aren't available in the container's cgroup available set",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNotAvailableCpusetMems is generated when user provided cpuset
	// memory nodes aren't available in the container's cgroup.
	ErrorCodeNotAvailableCpusetMems = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOTAVAILABLECPUSETMEMS",
		Message:        "Requested memory nodes are not available - requested %s, available: %s.",
		Description:    "While verifying the container's 'HostConfig', cpuset memory nodes provided aren't available in the container's cgroup available set",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorVolumeNameTaken is generated when an error occurred while
	// trying to create a volume that has existed using different driver.
	ErrorVolumeNameTaken = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "VOLUME_NAME_TAKEN",
		Message:        "A volume name %s already exists with the %s driver. Choose a different volume name.",
		Description:    "An attempt to create a volume using a driver but the volume already exists with a different driver",
		HTTPStatusCode: http.StatusInternalServerError,
	})
)
