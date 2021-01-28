package winapi

import (
	"golang.org/x/sys/windows"
)

// Messages that can be received from an assigned io completion port.
// https://docs.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_associate_completion_port
const (
	JOB_OBJECT_MSG_END_OF_JOB_TIME       = 1
	JOB_OBJECT_MSG_END_OF_PROCESS_TIME   = 2
	JOB_OBJECT_MSG_ACTIVE_PROCESS_LIMIT  = 3
	JOB_OBJECT_MSG_ACTIVE_PROCESS_ZERO   = 4
	JOB_OBJECT_MSG_NEW_PROCESS           = 6
	JOB_OBJECT_MSG_EXIT_PROCESS          = 7
	JOB_OBJECT_MSG_ABNORMAL_EXIT_PROCESS = 8
	JOB_OBJECT_MSG_PROCESS_MEMORY_LIMIT  = 9
	JOB_OBJECT_MSG_JOB_MEMORY_LIMIT      = 10
	JOB_OBJECT_MSG_NOTIFICATION_LIMIT    = 11
)

// IO limit flags
//
// https://docs.microsoft.com/en-us/windows/win32/api/jobapi2/ns-jobapi2-jobobject_io_rate_control_information
const JOB_OBJECT_IO_RATE_CONTROL_ENABLE = 0x1

// https://docs.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_cpu_rate_control_information
const (
	JOB_OBJECT_CPU_RATE_CONTROL_ENABLE = 1 << iota
	JOB_OBJECT_CPU_RATE_CONTROL_WEIGHT_BASED
	JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP
	JOB_OBJECT_CPU_RATE_CONTROL_NOTIFY
	JOB_OBJECT_CPU_RATE_CONTROL_MIN_MAX_RATE
)

// JobObjectInformationClass values. Used for a call to QueryInformationJobObject
//
// https://docs.microsoft.com/en-us/windows/win32/api/jobapi2/nf-jobapi2-queryinformationjobobject
const (
	JobObjectBasicAccountingInformation      uint32 = 1
	JobObjectBasicProcessIdList              uint32 = 3
	JobObjectBasicAndIoAccountingInformation uint32 = 8
	JobObjectLimitViolationInformation       uint32 = 13
	JobObjectNotificationLimitInformation2   uint32 = 33
)

// https://docs.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_basic_limit_information
type JOBOBJECT_BASIC_LIMIT_INFORMATION struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

// https://docs.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_cpu_rate_control_information
type JOBOBJECT_CPU_RATE_CONTROL_INFORMATION struct {
	ControlFlags uint32
	Rate         uint32
}

// https://docs.microsoft.com/en-us/windows/win32/api/jobapi2/ns-jobapi2-jobobject_io_rate_control_information
type JOBOBJECT_IO_RATE_CONTROL_INFORMATION struct {
	MaxIops         int64
	MaxBandwidth    int64
	ReservationIops int64
	BaseIOSize      uint32
	VolumeName      string
	ControlFlags    uint32
}

// https://docs.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_basic_process_id_list
type JOBOBJECT_BASIC_PROCESS_ID_LIST struct {
	NumberOfAssignedProcesses uint32
	NumberOfProcessIdsInList  uint32
	ProcessIdList             [1]uintptr
}

// https://docs.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_associate_completion_port
type JOBOBJECT_ASSOCIATE_COMPLETION_PORT struct {
	CompletionKey  uintptr
	CompletionPort windows.Handle
}

// BOOL IsProcessInJob(
// 		HANDLE ProcessHandle,
// 		HANDLE JobHandle,
// 		PBOOL  Result
// );
//
//sys IsProcessInJob(procHandle windows.Handle, jobHandle windows.Handle, result *bool) (err error) = kernel32.IsProcessInJob

// BOOL QueryInformationJobObject(
//		HANDLE             hJob,
//		JOBOBJECTINFOCLASS JobObjectInformationClass,
//		LPVOID             lpJobObjectInformation,
//		DWORD              cbJobObjectInformationLength,
//		LPDWORD            lpReturnLength
// );
//
//sys QueryInformationJobObject(jobHandle windows.Handle, infoClass uint32, jobObjectInfo uintptr, jobObjectInformationLength uint32, lpReturnLength *uint32) (err error) = kernel32.QueryInformationJobObject

// HANDLE OpenJobObjectW(
//		DWORD   dwDesiredAccess,
//		BOOL    bInheritHandle,
//		LPCWSTR lpName
// );
//
//sys OpenJobObject(desiredAccess uint32, inheritHandle bool, lpName *uint16) (handle windows.Handle, err error) = kernel32.OpenJobObjectW

// DWORD SetIoRateControlInformationJobObject(
//		HANDLE                                hJob,
//		JOBOBJECT_IO_RATE_CONTROL_INFORMATION *IoRateControlInfo
// );
//
//sys SetIoRateControlInformationJobObject(jobHandle windows.Handle, ioRateControlInfo *JOBOBJECT_IO_RATE_CONTROL_INFORMATION) (ret uint32, err error) = kernel32.SetIoRateControlInformationJobObject
