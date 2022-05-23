package jobobject

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/Microsoft/hcsshim/internal/queue"
	"github.com/Microsoft/hcsshim/internal/winapi"
	"golang.org/x/sys/windows"
)

// This file provides higher level constructs for the win32 job object API.
// Most of the core creation and management functions are already present in "golang.org/x/sys/windows"
// (CreateJobObject, AssignProcessToJobObject, etc.) as well as most of the limit information
// structs and associated limit flags. Whatever is not present from the job object API
// in golang.org/x/sys/windows is located in /internal/winapi.
//
// https://docs.microsoft.com/en-us/windows/win32/procthread/job-objects

// JobObject is a high level wrapper around a Windows job object. Holds a handle to
// the job, a queue to receive iocp notifications about the lifecycle
// of the job and a mutex for synchronized handle access.
type JobObject struct {
	handle     windows.Handle
	mq         *queue.MessageQueue
	handleLock sync.RWMutex
}

// JobLimits represents the resource constraints that can be applied to a job object.
type JobLimits struct {
	CPULimit           uint32
	CPUWeight          uint32
	MemoryLimitInBytes uint64
	MaxIOPS            int64
	MaxBandwidth       int64
}

type CPURateControlType uint32

const (
	WeightBased CPURateControlType = iota
	RateBased
)

// Processor resource controls
const (
	cpuLimitMin  = 1
	cpuLimitMax  = 10000
	cpuWeightMin = 1
	cpuWeightMax = 9
)

var (
	ErrAlreadyClosed = errors.New("the handle has already been closed")
	ErrNotRegistered = errors.New("job is not registered to receive notifications")
)

// Options represents the set of configurable options when making or opening a job object.
type Options struct {
	// `Name` specifies the name of the job object if a named job object is desired.
	Name string
	// `Notifications` specifies if the job will be registered to receive notifications.
	// Defaults to false.
	Notifications bool
	// `UseNTVariant` specifies if we should use the `Nt` variant of Open/CreateJobObject.
	// Defaults to false.
	UseNTVariant bool
}

// Create creates a job object.
//
// If options.Name is an empty string, the job will not be assigned a name.
//
// If options.Notifications are not enabled `PollNotifications` will return immediately with error `errNotRegistered`.
//
// If `options` is nil, use default option values.
//
// Returns a JobObject structure and an error if there is one.
func Create(ctx context.Context, options *Options) (_ *JobObject, err error) {
	if options == nil {
		options = &Options{}
	}

	var jobName *winapi.UnicodeString
	if options.Name != "" {
		jobName, err = winapi.NewUnicodeString(options.Name)
		if err != nil {
			return nil, err
		}
	}

	var jobHandle windows.Handle
	if options.UseNTVariant {
		oa := winapi.ObjectAttributes{
			Length:     unsafe.Sizeof(winapi.ObjectAttributes{}),
			ObjectName: jobName,
			Attributes: 0,
		}
		status := winapi.NtCreateJobObject(&jobHandle, winapi.JOB_OBJECT_ALL_ACCESS, &oa)
		if status != 0 {
			return nil, winapi.RtlNtStatusToDosError(status)
		}
	} else {
		var jobNameBuf *uint16
		if jobName != nil && jobName.Buffer != nil {
			jobNameBuf = jobName.Buffer
		}
		jobHandle, err = windows.CreateJobObject(nil, jobNameBuf)
		if err != nil {
			return nil, err
		}
	}

	defer func() {
		if err != nil {
			windows.Close(jobHandle)
		}
	}()

	job := &JobObject{
		handle: jobHandle,
	}

	// If the IOCP we'll be using to receive messages for all jobs hasn't been
	// created, create it and start polling.
	if options.Notifications {
		mq, err := setupNotifications(ctx, job)
		if err != nil {
			return nil, err
		}
		job.mq = mq
	}

	return job, nil
}

// Open opens an existing job object with name provided in `options`. If no name is provided
// return an error since we need to know what job object to open.
//
// If options.Notifications is false `PollNotifications` will return immediately with error `errNotRegistered`.
//
// Returns a JobObject structure and an error if there is one.
func Open(ctx context.Context, options *Options) (_ *JobObject, err error) {
	if options == nil || (options != nil && options.Name == "") {
		return nil, errors.New("no job object name specified to open")
	}

	unicodeJobName, err := winapi.NewUnicodeString(options.Name)
	if err != nil {
		return nil, err
	}

	var jobHandle windows.Handle
	if options != nil && options.UseNTVariant {
		oa := winapi.ObjectAttributes{
			Length:     unsafe.Sizeof(winapi.ObjectAttributes{}),
			ObjectName: unicodeJobName,
			Attributes: 0,
		}
		status := winapi.NtOpenJobObject(&jobHandle, winapi.JOB_OBJECT_ALL_ACCESS, &oa)
		if status != 0 {
			return nil, winapi.RtlNtStatusToDosError(status)
		}
	} else {
		jobHandle, err = winapi.OpenJobObject(winapi.JOB_OBJECT_ALL_ACCESS, false, unicodeJobName.Buffer)
		if err != nil {
			return nil, err
		}
	}

	defer func() {
		if err != nil {
			windows.Close(jobHandle)
		}
	}()

	job := &JobObject{
		handle: jobHandle,
	}

	// If the IOCP we'll be using to receive messages for all jobs hasn't been
	// created, create it and start polling.
	if options != nil && options.Notifications {
		mq, err := setupNotifications(ctx, job)
		if err != nil {
			return nil, err
		}
		job.mq = mq
	}

	return job, nil
}

// helper function to setup notifications for creating/opening a job object
func setupNotifications(ctx context.Context, job *JobObject) (*queue.MessageQueue, error) {
	job.handleLock.RLock()
	defer job.handleLock.RUnlock()

	if job.handle == 0 {
		return nil, ErrAlreadyClosed
	}

	ioInitOnce.Do(func() {
		h, err := windows.CreateIoCompletionPort(windows.InvalidHandle, 0, 0, 0xffffffff)
		if err != nil {
			initIOErr = err
			return
		}
		ioCompletionPort = h
		go pollIOCP(ctx, h)
	})

	if initIOErr != nil {
		return nil, initIOErr
	}

	mq := queue.NewMessageQueue()
	jobMap.Store(uintptr(job.handle), mq)
	if err := attachIOCP(job.handle, ioCompletionPort); err != nil {
		jobMap.Delete(uintptr(job.handle))
		return nil, fmt.Errorf("failed to attach job to IO completion port: %w", err)
	}
	return mq, nil
}

// PollNotification will poll for a job object notification. This call should only be called once
// per job (ideally in a goroutine loop) and will block if there is not a notification ready.
// This call will return immediately with error `ErrNotRegistered` if the job was not registered
// to receive notifications during `Create`. Internally, messages will be queued and there
// is no worry of messages being dropped.
func (job *JobObject) PollNotification() (interface{}, error) {
	if job.mq == nil {
		return nil, ErrNotRegistered
	}
	return job.mq.ReadOrWait()
}

// UpdateProcThreadAttribute updates the passed in ProcThreadAttributeList to contain what is necessary to
// launch a process in a job at creation time. This can be used to avoid having to call Assign() after a process
// has already started running.
func (job *JobObject) UpdateProcThreadAttribute(attrList *windows.ProcThreadAttributeListContainer) error {
	job.handleLock.RLock()
	defer job.handleLock.RUnlock()

	if job.handle == 0 {
		return ErrAlreadyClosed
	}

	if err := attrList.Update(
		winapi.PROC_THREAD_ATTRIBUTE_JOB_LIST,
		unsafe.Pointer(&job.handle),
		unsafe.Sizeof(job.handle),
	); err != nil {
		return fmt.Errorf("failed to update proc thread attributes for job object: %w", err)
	}

	return nil
}

// Close closes the job object handle.
func (job *JobObject) Close() error {
	job.handleLock.Lock()
	defer job.handleLock.Unlock()

	if job.handle == 0 {
		return ErrAlreadyClosed
	}

	if err := windows.Close(job.handle); err != nil {
		return err
	}

	if job.mq != nil {
		job.mq.Close()
	}
	// Handles now invalid so if the map entry to receive notifications for this job still
	// exists remove it so we can stop receiving notifications.
	if _, ok := jobMap.Load(uintptr(job.handle)); ok {
		jobMap.Delete(uintptr(job.handle))
	}

	job.handle = 0
	return nil
}

// Assign assigns a process to the job object.
func (job *JobObject) Assign(pid uint32) error {
	job.handleLock.RLock()
	defer job.handleLock.RUnlock()

	if job.handle == 0 {
		return ErrAlreadyClosed
	}

	if pid == 0 {
		return errors.New("invalid pid: 0")
	}
	hProc, err := windows.OpenProcess(winapi.PROCESS_ALL_ACCESS, true, pid)
	if err != nil {
		return err
	}
	defer windows.Close(hProc)
	return windows.AssignProcessToJobObject(job.handle, hProc)
}

// Terminate terminates the job, essentially calls TerminateProcess on every process in the
// job.
func (job *JobObject) Terminate(exitCode uint32) error {
	job.handleLock.RLock()
	defer job.handleLock.RUnlock()
	if job.handle == 0 {
		return ErrAlreadyClosed
	}
	return windows.TerminateJobObject(job.handle, exitCode)
}

// Pids returns all of the process IDs in the job object.
func (job *JobObject) Pids() ([]uint32, error) {
	job.handleLock.RLock()
	defer job.handleLock.RUnlock()

	if job.handle == 0 {
		return nil, ErrAlreadyClosed
	}

	info := winapi.JOBOBJECT_BASIC_PROCESS_ID_LIST{}
	err := winapi.QueryInformationJobObject(
		job.handle,
		winapi.JobObjectBasicProcessIdList,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
		nil,
	)

	// This is either the case where there is only one process or no processes in
	// the job. Any other case will result in ERROR_MORE_DATA. Check if info.NumberOfProcessIdsInList
	// is 1 and just return this, otherwise return an empty slice.
	if err == nil {
		if info.NumberOfProcessIdsInList == 1 {
			return []uint32{uint32(info.ProcessIdList[0])}, nil
		}
		// Return empty slice instead of nil to play well with the caller of this.
		// Do not return an error if no processes are running inside the job
		return []uint32{}, nil
	}

	if err != winapi.ERROR_MORE_DATA {
		return nil, fmt.Errorf("failed initial query for PIDs in job object: %w", err)
	}

	jobBasicProcessIDListSize := unsafe.Sizeof(info) + (unsafe.Sizeof(info.ProcessIdList[0]) * uintptr(info.NumberOfAssignedProcesses-1))
	buf := make([]byte, jobBasicProcessIDListSize)
	if err = winapi.QueryInformationJobObject(
		job.handle,
		winapi.JobObjectBasicProcessIdList,
		uintptr(unsafe.Pointer(&buf[0])),
		uint32(len(buf)),
		nil,
	); err != nil {
		return nil, fmt.Errorf("failed to query for PIDs in job object: %w", err)
	}

	bufInfo := (*winapi.JOBOBJECT_BASIC_PROCESS_ID_LIST)(unsafe.Pointer(&buf[0]))
	pids := make([]uint32, bufInfo.NumberOfProcessIdsInList)
	for i, bufPid := range bufInfo.AllPids() {
		pids[i] = uint32(bufPid)
	}
	return pids, nil
}

// QueryMemoryStats gets the memory stats for the job object.
func (job *JobObject) QueryMemoryStats() (*winapi.JOBOBJECT_MEMORY_USAGE_INFORMATION, error) {
	job.handleLock.RLock()
	defer job.handleLock.RUnlock()

	if job.handle == 0 {
		return nil, ErrAlreadyClosed
	}

	info := winapi.JOBOBJECT_MEMORY_USAGE_INFORMATION{}
	if err := winapi.QueryInformationJobObject(
		job.handle,
		winapi.JobObjectMemoryUsageInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
		nil,
	); err != nil {
		return nil, fmt.Errorf("failed to query for job object memory stats: %w", err)
	}
	return &info, nil
}

// QueryProcessorStats gets the processor stats for the job object.
func (job *JobObject) QueryProcessorStats() (*winapi.JOBOBJECT_BASIC_ACCOUNTING_INFORMATION, error) {
	job.handleLock.RLock()
	defer job.handleLock.RUnlock()

	if job.handle == 0 {
		return nil, ErrAlreadyClosed
	}

	info := winapi.JOBOBJECT_BASIC_ACCOUNTING_INFORMATION{}
	if err := winapi.QueryInformationJobObject(
		job.handle,
		winapi.JobObjectBasicAccountingInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
		nil,
	); err != nil {
		return nil, fmt.Errorf("failed to query for job object process stats: %w", err)
	}
	return &info, nil
}

// QueryStorageStats gets the storage (I/O) stats for the job object.
func (job *JobObject) QueryStorageStats() (*winapi.JOBOBJECT_IO_ATTRIBUTION_INFORMATION, error) {
	job.handleLock.RLock()
	defer job.handleLock.RUnlock()

	if job.handle == 0 {
		return nil, ErrAlreadyClosed
	}

	info := winapi.JOBOBJECT_IO_ATTRIBUTION_INFORMATION{
		ControlFlags: winapi.JOBOBJECT_IO_ATTRIBUTION_CONTROL_ENABLE,
	}
	if err := winapi.QueryInformationJobObject(
		job.handle,
		winapi.JobObjectIoAttribution,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
		nil,
	); err != nil {
		return nil, fmt.Errorf("failed to query for job object storage stats: %w", err)
	}
	return &info, nil
}

// QueryPrivateWorkingSet returns the private working set size for the job. This is calculated by adding up the
// private working set for every process running in the job.
func (job *JobObject) QueryPrivateWorkingSet() (uint64, error) {
	pids, err := job.Pids()
	if err != nil {
		return 0, err
	}

	openAndQueryWorkingSet := func(pid uint32) (uint64, error) {
		h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
		if err != nil {
			// Continue to the next if OpenProcess doesn't return a valid handle (fails). Handles a
			// case where one of the pids in the job exited before we open.
			return 0, nil
		}
		defer func() {
			_ = windows.Close(h)
		}()
		// Check if the process is actually running in the job still. There's a small chance
		// that the process could have exited and had its pid re-used between grabbing the pids
		// in the job and opening the handle to it above.
		var inJob int32
		if err := winapi.IsProcessInJob(h, job.handle, &inJob); err != nil {
			// This shouldn't fail unless we have incorrect access rights which we control
			// here so probably best to error out if this failed.
			return 0, err
		}
		// Don't report stats for this process as it's not running in the job. This shouldn't be
		// an error condition though.
		if inJob == 0 {
			return 0, nil
		}

		var vmCounters winapi.VM_COUNTERS_EX2
		status := winapi.NtQueryInformationProcess(
			h,
			winapi.ProcessVmCounters,
			uintptr(unsafe.Pointer(&vmCounters)),
			uint32(unsafe.Sizeof(vmCounters)),
			nil,
		)
		if !winapi.NTSuccess(status) {
			return 0, fmt.Errorf("failed to query information for process: %w", winapi.RtlNtStatusToDosError(status))
		}
		return uint64(vmCounters.PrivateWorkingSetSize), nil
	}

	var jobWorkingSetSize uint64
	for _, pid := range pids {
		workingSet, err := openAndQueryWorkingSet(pid)
		if err != nil {
			return 0, err
		}
		jobWorkingSetSize += workingSet
	}

	return jobWorkingSetSize, nil
}
