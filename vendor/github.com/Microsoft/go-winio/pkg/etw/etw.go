// Package etw provides support for TraceLogging-based ETW (Event Tracing
// for Windows). TraceLogging is a format of ETW events that are self-describing
// (the event contains information on its own schema). This allows them to be
// decoded without needing a separate manifest with event information. The
// implementation here is based on the information found in
// TraceLoggingProvider.h in the Windows SDK, which implements TraceLogging as a
// set of C macros.
package etw

//go:generate go run $GOROOT/src/syscall/mksyscall_windows.go -output zsyscall_windows.go etw.go

//sys eventRegister(providerId *windows.GUID, callback uintptr, callbackContext uintptr, providerHandle *providerHandle) (win32err error) = advapi32.EventRegister
//sys eventUnregister(providerHandle providerHandle) (win32err error) = advapi32.EventUnregister
//sys eventWriteTransfer(providerHandle providerHandle, descriptor *eventDescriptor, activityID *windows.GUID, relatedActivityID *windows.GUID, dataDescriptorCount uint32, dataDescriptors *eventDataDescriptor) (win32err error) = advapi32.EventWriteTransfer
//sys eventSetInformation(providerHandle providerHandle, class eventInfoClass, information uintptr, length uint32) (win32err error) = advapi32.EventSetInformation
