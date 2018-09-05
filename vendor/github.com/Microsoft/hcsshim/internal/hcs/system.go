package hcs

import (
	"encoding/json"
	"os"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/Microsoft/hcsshim/internal/interop"
	"github.com/Microsoft/hcsshim/internal/schema1"
	"github.com/Microsoft/hcsshim/internal/timeout"
	"github.com/sirupsen/logrus"
)

// currentContainerStarts is used to limit the number of concurrent container
// starts.
var currentContainerStarts containerStarts

type containerStarts struct {
	maxParallel int
	inProgress  int
	sync.Mutex
}

func init() {
	mpsS := os.Getenv("HCSSHIM_MAX_PARALLEL_START")
	if len(mpsS) > 0 {
		mpsI, err := strconv.Atoi(mpsS)
		if err != nil || mpsI < 0 {
			return
		}
		currentContainerStarts.maxParallel = mpsI
	}
}

type System struct {
	handleLock     sync.RWMutex
	handle         hcsSystem
	id             string
	callbackNumber uintptr
}

// CreateComputeSystem creates a new compute system with the given configuration but does not start it.
func CreateComputeSystem(id string, hcsDocumentInterface interface{}) (*System, error) {
	operation := "CreateComputeSystem"
	title := "hcsshim::" + operation

	computeSystem := &System{
		id: id,
	}

	hcsDocumentB, err := json.Marshal(hcsDocumentInterface)
	if err != nil {
		return nil, err
	}

	hcsDocument := string(hcsDocumentB)
	logrus.Debugf(title+" ID=%s config=%s", id, hcsDocument)

	var (
		resultp  *uint16
		identity syscall.Handle
	)
	createError := hcsCreateComputeSystem(id, hcsDocument, identity, &computeSystem.handle, &resultp)

	if createError == nil || IsPending(createError) {
		if err := computeSystem.registerCallback(); err != nil {
			// Terminate the compute system if it still exists. We're okay to
			// ignore a failure here.
			computeSystem.Terminate()
			return nil, makeSystemError(computeSystem, operation, "", err, nil)
		}
	}

	events, err := processAsyncHcsResult(createError, resultp, computeSystem.callbackNumber, hcsNotificationSystemCreateCompleted, &timeout.Duration)
	if err != nil {
		if err == ErrTimeout {
			// Terminate the compute system if it still exists. We're okay to
			// ignore a failure here.
			computeSystem.Terminate()
		}
		return nil, makeSystemError(computeSystem, operation, hcsDocument, err, events)
	}

	logrus.Debugf(title+" succeeded id=%s handle=%d", id, computeSystem.handle)
	return computeSystem, nil
}

// OpenComputeSystem opens an existing compute system by ID.
func OpenComputeSystem(id string) (*System, error) {
	operation := "OpenComputeSystem"
	title := "hcsshim::" + operation
	logrus.Debugf(title+" ID=%s", id)

	computeSystem := &System{
		id: id,
	}

	var (
		handle  hcsSystem
		resultp *uint16
	)
	err := hcsOpenComputeSystem(id, &handle, &resultp)
	events := processHcsResult(resultp)
	if err != nil {
		return nil, makeSystemError(computeSystem, operation, "", err, events)
	}

	computeSystem.handle = handle

	if err := computeSystem.registerCallback(); err != nil {
		return nil, makeSystemError(computeSystem, operation, "", err, nil)
	}

	logrus.Debugf(title+" succeeded id=%s handle=%d", id, handle)
	return computeSystem, nil
}

// GetComputeSystems gets a list of the compute systems on the system that match the query
func GetComputeSystems(q schema1.ComputeSystemQuery) ([]schema1.ContainerProperties, error) {
	operation := "GetComputeSystems"
	title := "hcsshim::" + operation

	queryb, err := json.Marshal(q)
	if err != nil {
		return nil, err
	}

	query := string(queryb)
	logrus.Debugf(title+" query=%s", query)

	var (
		resultp         *uint16
		computeSystemsp *uint16
	)
	err = hcsEnumerateComputeSystems(query, &computeSystemsp, &resultp)
	events := processHcsResult(resultp)
	if err != nil {
		return nil, &HcsError{Op: operation, Err: err, Events: events}
	}

	if computeSystemsp == nil {
		return nil, ErrUnexpectedValue
	}
	computeSystemsRaw := interop.ConvertAndFreeCoTaskMemBytes(computeSystemsp)
	computeSystems := []schema1.ContainerProperties{}
	if err := json.Unmarshal(computeSystemsRaw, &computeSystems); err != nil {
		return nil, err
	}

	logrus.Debugf(title + " succeeded")
	return computeSystems, nil
}

// Start synchronously starts the computeSystem.
func (computeSystem *System) Start() error {
	computeSystem.handleLock.RLock()
	defer computeSystem.handleLock.RUnlock()
	title := "hcsshim::ComputeSystem::Start ID=" + computeSystem.ID()
	logrus.Debugf(title)

	if computeSystem.handle == 0 {
		return makeSystemError(computeSystem, "Start", "", ErrAlreadyClosed, nil)
	}

	// This is a very simple backoff-retry loop to limit the number
	// of parallel container starts if environment variable
	// HCSSHIM_MAX_PARALLEL_START is set to a positive integer.
	// It should generally only be used as a workaround to various
	// platform issues that exist between RS1 and RS4 as of Aug 2018
	if currentContainerStarts.maxParallel > 0 {
		for {
			currentContainerStarts.Lock()
			if currentContainerStarts.inProgress < currentContainerStarts.maxParallel {
				currentContainerStarts.inProgress++
				currentContainerStarts.Unlock()
				break
			}
			if currentContainerStarts.inProgress == currentContainerStarts.maxParallel {
				currentContainerStarts.Unlock()
				time.Sleep(100 * time.Millisecond)
			}
		}
		// Make sure we decrement the count when we are done.
		defer func() {
			currentContainerStarts.Lock()
			currentContainerStarts.inProgress--
			currentContainerStarts.Unlock()
		}()
	}

	var resultp *uint16
	err := hcsStartComputeSystem(computeSystem.handle, "", &resultp)
	events, err := processAsyncHcsResult(err, resultp, computeSystem.callbackNumber, hcsNotificationSystemStartCompleted, &timeout.Duration)
	if err != nil {
		return makeSystemError(computeSystem, "Start", "", err, events)
	}

	logrus.Debugf(title + " succeeded")
	return nil
}

// ID returns the compute system's identifier.
func (computeSystem *System) ID() string {
	return computeSystem.id
}

// Shutdown requests a compute system shutdown, if IsPending() on the error returned is true,
// it may not actually be shut down until Wait() succeeds.
func (computeSystem *System) Shutdown() error {
	computeSystem.handleLock.RLock()
	defer computeSystem.handleLock.RUnlock()
	title := "hcsshim::ComputeSystem::Shutdown"
	logrus.Debugf(title)
	if computeSystem.handle == 0 {
		return makeSystemError(computeSystem, "Shutdown", "", ErrAlreadyClosed, nil)
	}

	var resultp *uint16
	err := hcsShutdownComputeSystem(computeSystem.handle, "", &resultp)
	events := processHcsResult(resultp)
	if err != nil {
		return makeSystemError(computeSystem, "Shutdown", "", err, events)
	}

	logrus.Debugf(title + " succeeded")
	return nil
}

// Terminate requests a compute system terminate, if IsPending() on the error returned is true,
// it may not actually be shut down until Wait() succeeds.
func (computeSystem *System) Terminate() error {
	computeSystem.handleLock.RLock()
	defer computeSystem.handleLock.RUnlock()
	title := "hcsshim::ComputeSystem::Terminate ID=" + computeSystem.ID()
	logrus.Debugf(title)

	if computeSystem.handle == 0 {
		return makeSystemError(computeSystem, "Terminate", "", ErrAlreadyClosed, nil)
	}

	var resultp *uint16
	err := hcsTerminateComputeSystem(computeSystem.handle, "", &resultp)
	events := processHcsResult(resultp)
	if err != nil {
		return makeSystemError(computeSystem, "Terminate", "", err, events)
	}

	logrus.Debugf(title + " succeeded")
	return nil
}

// Wait synchronously waits for the compute system to shutdown or terminate.
func (computeSystem *System) Wait() error {
	title := "hcsshim::ComputeSystem::Wait ID=" + computeSystem.ID()
	logrus.Debugf(title)

	err := waitForNotification(computeSystem.callbackNumber, hcsNotificationSystemExited, nil)
	if err != nil {
		return makeSystemError(computeSystem, "Wait", "", err, nil)
	}

	logrus.Debugf(title + " succeeded")
	return nil
}

// WaitTimeout synchronously waits for the compute system to terminate or the duration to elapse.
// If the timeout expires, IsTimeout(err) == true
func (computeSystem *System) WaitTimeout(timeout time.Duration) error {
	title := "hcsshim::ComputeSystem::WaitTimeout ID=" + computeSystem.ID()
	logrus.Debugf(title)

	err := waitForNotification(computeSystem.callbackNumber, hcsNotificationSystemExited, &timeout)
	if err != nil {
		return makeSystemError(computeSystem, "WaitTimeout", "", err, nil)
	}

	logrus.Debugf(title + " succeeded")
	return nil
}

func (computeSystem *System) Properties(types ...schema1.PropertyType) (*schema1.ContainerProperties, error) {
	computeSystem.handleLock.RLock()
	defer computeSystem.handleLock.RUnlock()

	queryj, err := json.Marshal(schema1.PropertyQuery{types})
	if err != nil {
		return nil, makeSystemError(computeSystem, "Properties", "", err, nil)
	}

	var resultp, propertiesp *uint16
	err = hcsGetComputeSystemProperties(computeSystem.handle, string(queryj), &propertiesp, &resultp)
	events := processHcsResult(resultp)
	if err != nil {
		return nil, makeSystemError(computeSystem, "Properties", "", err, events)
	}

	if propertiesp == nil {
		return nil, ErrUnexpectedValue
	}
	propertiesRaw := interop.ConvertAndFreeCoTaskMemBytes(propertiesp)
	properties := &schema1.ContainerProperties{}
	if err := json.Unmarshal(propertiesRaw, properties); err != nil {
		return nil, makeSystemError(computeSystem, "Properties", "", err, nil)
	}
	return properties, nil
}

// Pause pauses the execution of the computeSystem. This feature is not enabled in TP5.
func (computeSystem *System) Pause() error {
	computeSystem.handleLock.RLock()
	defer computeSystem.handleLock.RUnlock()
	title := "hcsshim::ComputeSystem::Pause ID=" + computeSystem.ID()
	logrus.Debugf(title)

	if computeSystem.handle == 0 {
		return makeSystemError(computeSystem, "Pause", "", ErrAlreadyClosed, nil)
	}

	var resultp *uint16
	err := hcsPauseComputeSystem(computeSystem.handle, "", &resultp)
	events, err := processAsyncHcsResult(err, resultp, computeSystem.callbackNumber, hcsNotificationSystemPauseCompleted, &timeout.Duration)
	if err != nil {
		return makeSystemError(computeSystem, "Pause", "", err, events)
	}

	logrus.Debugf(title + " succeeded")
	return nil
}

// Resume resumes the execution of the computeSystem. This feature is not enabled in TP5.
func (computeSystem *System) Resume() error {
	computeSystem.handleLock.RLock()
	defer computeSystem.handleLock.RUnlock()
	title := "hcsshim::ComputeSystem::Resume ID=" + computeSystem.ID()
	logrus.Debugf(title)

	if computeSystem.handle == 0 {
		return makeSystemError(computeSystem, "Resume", "", ErrAlreadyClosed, nil)
	}

	var resultp *uint16
	err := hcsResumeComputeSystem(computeSystem.handle, "", &resultp)
	events, err := processAsyncHcsResult(err, resultp, computeSystem.callbackNumber, hcsNotificationSystemResumeCompleted, &timeout.Duration)
	if err != nil {
		return makeSystemError(computeSystem, "Resume", "", err, events)
	}

	logrus.Debugf(title + " succeeded")
	return nil
}

// CreateProcess launches a new process within the computeSystem.
func (computeSystem *System) CreateProcess(c interface{}) (*Process, error) {
	computeSystem.handleLock.RLock()
	defer computeSystem.handleLock.RUnlock()
	title := "hcsshim::ComputeSystem::CreateProcess ID=" + computeSystem.ID()
	var (
		processInfo   hcsProcessInformation
		processHandle hcsProcess
		resultp       *uint16
	)

	if computeSystem.handle == 0 {
		return nil, makeSystemError(computeSystem, "CreateProcess", "", ErrAlreadyClosed, nil)
	}

	configurationb, err := json.Marshal(c)
	if err != nil {
		return nil, makeSystemError(computeSystem, "CreateProcess", "", err, nil)
	}

	configuration := string(configurationb)
	logrus.Debugf(title+" config=%s", configuration)

	err = hcsCreateProcess(computeSystem.handle, configuration, &processInfo, &processHandle, &resultp)
	events := processHcsResult(resultp)
	if err != nil {
		return nil, makeSystemError(computeSystem, "CreateProcess", configuration, err, events)
	}

	process := &Process{
		handle:    processHandle,
		processID: int(processInfo.ProcessId),
		system:    computeSystem,
		cachedPipes: &cachedPipes{
			stdIn:  processInfo.StdInput,
			stdOut: processInfo.StdOutput,
			stdErr: processInfo.StdError,
		},
	}

	if err := process.registerCallback(); err != nil {
		return nil, makeSystemError(computeSystem, "CreateProcess", "", err, nil)
	}

	logrus.Debugf(title+" succeeded processid=%d", process.processID)
	return process, nil
}

// OpenProcess gets an interface to an existing process within the computeSystem.
func (computeSystem *System) OpenProcess(pid int) (*Process, error) {
	computeSystem.handleLock.RLock()
	defer computeSystem.handleLock.RUnlock()
	title := "hcsshim::ComputeSystem::OpenProcess ID=" + computeSystem.ID()
	logrus.Debugf(title+" processid=%d", pid)
	var (
		processHandle hcsProcess
		resultp       *uint16
	)

	if computeSystem.handle == 0 {
		return nil, makeSystemError(computeSystem, "OpenProcess", "", ErrAlreadyClosed, nil)
	}

	err := hcsOpenProcess(computeSystem.handle, uint32(pid), &processHandle, &resultp)
	events := processHcsResult(resultp)
	if err != nil {
		return nil, makeSystemError(computeSystem, "OpenProcess", "", err, events)
	}

	process := &Process{
		handle:    processHandle,
		processID: pid,
		system:    computeSystem,
	}

	if err := process.registerCallback(); err != nil {
		return nil, makeSystemError(computeSystem, "OpenProcess", "", err, nil)
	}

	logrus.Debugf(title+" succeeded processid=%s", process.processID)
	return process, nil
}

// Close cleans up any state associated with the compute system but does not terminate or wait for it.
func (computeSystem *System) Close() error {
	computeSystem.handleLock.Lock()
	defer computeSystem.handleLock.Unlock()
	title := "hcsshim::ComputeSystem::Close ID=" + computeSystem.ID()
	logrus.Debugf(title)

	// Don't double free this
	if computeSystem.handle == 0 {
		return nil
	}

	if err := computeSystem.unregisterCallback(); err != nil {
		return makeSystemError(computeSystem, "Close", "", err, nil)
	}

	if err := hcsCloseComputeSystem(computeSystem.handle); err != nil {
		return makeSystemError(computeSystem, "Close", "", err, nil)
	}

	computeSystem.handle = 0

	logrus.Debugf(title + " succeeded")
	return nil
}

func (computeSystem *System) registerCallback() error {
	context := &notifcationWatcherContext{
		channels: newChannels(),
	}

	callbackMapLock.Lock()
	callbackNumber := nextCallback
	nextCallback++
	callbackMap[callbackNumber] = context
	callbackMapLock.Unlock()

	var callbackHandle hcsCallback
	err := hcsRegisterComputeSystemCallback(computeSystem.handle, notificationWatcherCallback, callbackNumber, &callbackHandle)
	if err != nil {
		return err
	}
	context.handle = callbackHandle
	computeSystem.callbackNumber = callbackNumber

	return nil
}

func (computeSystem *System) unregisterCallback() error {
	callbackNumber := computeSystem.callbackNumber

	callbackMapLock.RLock()
	context := callbackMap[callbackNumber]
	callbackMapLock.RUnlock()

	if context == nil {
		return nil
	}

	handle := context.handle

	if handle == 0 {
		return nil
	}

	// hcsUnregisterComputeSystemCallback has its own syncronization
	// to wait for all callbacks to complete. We must NOT hold the callbackMapLock.
	err := hcsUnregisterComputeSystemCallback(handle)
	if err != nil {
		return err
	}

	closeChannels(context.channels)

	callbackMapLock.Lock()
	callbackMap[callbackNumber] = nil
	callbackMapLock.Unlock()

	handle = 0

	return nil
}

// Modifies the System by sending a request to HCS
func (computeSystem *System) Modify(config interface{}) error {
	computeSystem.handleLock.RLock()
	defer computeSystem.handleLock.RUnlock()
	title := "hcsshim::Modify ID=" + computeSystem.id

	if computeSystem.handle == 0 {
		return makeSystemError(computeSystem, "Modify", "", ErrAlreadyClosed, nil)
	}

	requestJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	requestString := string(requestJSON)
	logrus.Debugf(title + " " + requestString)

	var resultp *uint16
	err = hcsModifyComputeSystem(computeSystem.handle, requestString, &resultp)
	events := processHcsResult(resultp)
	if err != nil {
		return makeSystemError(computeSystem, "Modify", requestString, err, events)
	}
	logrus.Debugf(title + " succeeded ")
	return nil
}
