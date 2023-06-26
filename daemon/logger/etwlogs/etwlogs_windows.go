// Package etwlogs provides a log driver for forwarding container logs
// as ETW events.(ETW stands for Event Tracing for Windows)
// A client can then create an ETW listener to listen for events that are sent
// by the ETW provider that we register, using the provider's GUID "a3693192-9ed6-46d2-a981-f8226c8363bd".
// Here is an example of how to do this using the logman utility:
// 1. logman start -ets DockerContainerLogs -p {a3693192-9ed6-46d2-a981-f8226c8363bd} 0 0 -o trace.etl
// 2. Run container(s) and generate log messages
// 3. logman stop -ets DockerContainerLogs
// 4. You can then convert the etl log file to XML using: tracerpt -y trace.etl
//
// Each container log message generates an ETW event that also contains:
// the container name and ID, the timestamp, and the stream type.
package etwlogs // import "github.com/docker/docker/daemon/logger/etwlogs"

import (
	"context"
	"fmt"
	"sync"
	"unsafe"

	"github.com/Microsoft/go-winio/pkg/etw"
	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/containerd/containerd/log"
	"github.com/docker/docker/daemon/logger"
	"golang.org/x/sys/windows"
)

type etwLogs struct {
	containerName string
	imageName     string
	containerID   string
	imageID       string
}

const (
	name             = "etwlogs"
	providerGUID     = `a3693192-9ed6-46d2-a981-f8226c8363bd`
	win32CallSuccess = 0
)

var (
	modAdvapi32          = windows.NewLazySystemDLL("Advapi32.dll")
	procEventWriteString = modAdvapi32.NewProc("EventWriteString")
)

var (
	providerHandle windows.Handle
	mu             sync.Mutex
	refCount       int
	provider       *etw.Provider
)

func init() {
	providerHandle = windows.InvalidHandle
	if err := logger.RegisterLogDriver(name, New); err != nil {
		panic(err)
	}
}

// New creates a new etwLogs logger for the given container and registers the EWT provider.
func New(info logger.Info) (logger.Logger, error) {
	if err := registerETWProvider(); err != nil {
		return nil, err
	}
	log.G(context.TODO()).Debugf("logging driver etwLogs configured for container: %s.", info.ContainerID)

	return &etwLogs{
		containerName: info.Name(),
		imageName:     info.ContainerImageName,
		containerID:   info.ContainerID,
		imageID:       info.ContainerImageID,
	}, nil
}

// Log logs the message to the ETW stream.
func (etwLogger *etwLogs) Log(msg *logger.Message) error {
	// TODO(thaJeztah): log structured events instead and use provider.WriteEvent().
	m := createLogMessage(etwLogger, msg)
	logger.PutMessage(msg)
	return callEventWriteString(m)
}

// Close closes the logger by unregistering the ETW provider.
func (etwLogger *etwLogs) Close() error {
	unregisterETWProvider()
	return nil
}

func (etwLogger *etwLogs) Name() string {
	return name
}

func createLogMessage(etwLogger *etwLogs, msg *logger.Message) string {
	return fmt.Sprintf("container_name: %s, image_name: %s, container_id: %s, image_id: %s, source: %s, log: %s",
		etwLogger.containerName,
		etwLogger.imageName,
		etwLogger.containerID,
		etwLogger.imageID,
		msg.Source,
		msg.Line)
}

func registerETWProvider() error {
	mu.Lock()
	defer mu.Unlock()
	if refCount == 0 {
		var err error
		provider, err = callEventRegister()
		if err != nil {
			return err
		}
	}

	refCount++
	return nil
}

func unregisterETWProvider() {
	mu.Lock()
	defer mu.Unlock()
	if refCount == 1 {
		if err := callEventUnregister(); err != nil {
			// Not returning an error if EventUnregister fails, because etwLogs will continue to work
			return
		}
		refCount--
		provider = nil
		providerHandle = windows.InvalidHandle
	} else {
		refCount--
	}
}

func callEventRegister() (*etw.Provider, error) {
	providerID, _ := guid.FromString(providerGUID)
	p, err := etw.NewProviderWithOptions("", etw.WithID(providerID))
	if err != nil {
		log.G(context.TODO()).WithError(err).Error("Failed to register ETW provider")
		return nil, fmt.Errorf("failed to register ETW provider: %v", err)
	}
	return p, nil
}

// TODO(thaJeztah): port this function to github.com/Microsoft/go-winio/pkg/etw.
func callEventWriteString(message string) error {
	utf16message, err := windows.UTF16FromString(message)
	if err != nil {
		return err
	}

	ret, _, _ := procEventWriteString.Call(uintptr(providerHandle), 0, 0, uintptr(unsafe.Pointer(&utf16message[0])))
	if ret != win32CallSuccess {
		log.G(context.TODO()).WithError(err).Error("ETWLogs provider failed to log message")
		return fmt.Errorf("ETWLogs provider failed to log message: %v", err)
	}
	return nil
}

func callEventUnregister() error {
	return provider.Close()
}
