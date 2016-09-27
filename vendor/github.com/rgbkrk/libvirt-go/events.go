//+build !go1.6

package libvirt

import (
	"unsafe"
)

/*
 * Golang 1.6 doesn't support C pointers to go memory.
 * A hacky-solution might be some multi-threaded approach to support domain events, but let's make it work
 * without domain events for now.
 */

/*
#cgo LDFLAGS: -lvirt 
#include <libvirt/libvirt.h>

int domainEventLifecycleCallback_cgo(virConnectPtr c, virDomainPtr d,
                                     int event, int detail, void* data);

int domainEventGenericCallback_cgo(virConnectPtr c, virDomainPtr d, void* data);

int domainEventRTCChangeCallback_cgo(virConnectPtr c, virDomainPtr d,
                                     long long utcoffset, void* data);

int domainEventWatchdogCallback_cgo(virConnectPtr c, virDomainPtr d,
                                    int action, void* data);

int domainEventIOErrorCallback_cgo(virConnectPtr c, virDomainPtr d,
                                   const char *srcPath, const char *devAlias,
                                   int action, void* data);

int domainEventGraphicsCallback_cgo(virConnectPtr c, virDomainPtr d,
                                    int phase, const virDomainEventGraphicsAddress *local,
                                    const virDomainEventGraphicsAddress *remote,
                                    const char *authScheme,
                                    const virDomainEventGraphicsSubject *subject, void* data);

int domainEventIOErrorReasonCallback_cgo(virConnectPtr c, virDomainPtr d,
                                         const char *srcPath, const char *devAlias,
                                         int action, const char *reason, void* data);

int domainEventBlockJobCallback_cgo(virConnectPtr c, virDomainPtr d,
                                    const char *disk, int type, int status, void* data);

int domainEventDiskChangeCallback_cgo(virConnectPtr c, virDomainPtr d,
                                      const char *oldSrcPath, const char *newSrcPath,
                                      const char *devAlias, int reason, void* data);

int domainEventTrayChangeCallback_cgo(virConnectPtr c, virDomainPtr d,
                                      const char *devAlias, int reason, void* data);

int domainEventReasonCallback_cgo(virConnectPtr c, virDomainPtr d,
                                  int reason, void* data);

int domainEventBalloonChangeCallback_cgo(virConnectPtr c, virDomainPtr d,
                                         unsigned long long actual, void* data);

int domainEventDeviceRemovedCallback_cgo(virConnectPtr c, virDomainPtr d,
                                         const char *devAlias, void* data);
*/
import "C"

type DomainLifecycleEvent struct {
	Event  int
	Detail int
}

type DomainRTCChangeEvent struct {
	Utcoffset int64
}

type DomainWatchdogEvent struct {
	Action int
}

type DomainIOErrorEvent struct {
	SrcPath  string
	DevAlias string
	Action   int
}

type DomainEventGraphicsAddress struct {
	Family  int
	Node    string
	Service string
}

type DomainEventGraphicsSubjectIdentity struct {
	Type string
	Name string
}

type DomainGraphicsEvent struct {
	Phase      int
	Local      DomainEventGraphicsAddress
	Remote     DomainEventGraphicsAddress
	AuthScheme string
	Subject    []DomainEventGraphicsSubjectIdentity
}

type DomainIOErrorReasonEvent struct {
	DomainIOErrorEvent
	Reason string
}

type DomainBlockJobEvent struct {
	Disk   string
	Type   int
	Status int
}

type DomainDiskChangeEvent struct {
	OldSrcPath string
	NewSrcPath string
	DevAlias   string
	Reason     int
}

type DomainTrayChangeEvent struct {
	DevAlias string
	Reason   int
}

type DomainReasonEvent struct {
	Reason int
}

type DomainBalloonChangeEvent struct {
	Actual uint64
}

type DomainDeviceRemovedEvent struct {
	DevAlias string
}

//export domainEventLifecycleCallback
func domainEventLifecycleCallback(c C.virConnectPtr, d C.virDomainPtr,
	event int, detail int,
	opaque unsafe.Pointer) int {

	context := *(*domainCallbackContext)(opaque)
	domain := VirDomain{ptr: d}
	connection := VirConnection{ptr: c}

	eventDetails := DomainLifecycleEvent{
		Event:  event,
		Detail: detail,
	}

	return (*context.cb)(&connection, &domain, eventDetails, context.f)
}

//export domainEventGenericCallback
func domainEventGenericCallback(c C.virConnectPtr, d C.virDomainPtr,
	opaque unsafe.Pointer) int {

	context := *(*domainCallbackContext)(opaque)
	domain := VirDomain{ptr: d}
	connection := VirConnection{ptr: c}

	return (*context.cb)(&connection, &domain, nil, context.f)
}

//export domainEventRTCChangeCallback
func domainEventRTCChangeCallback(c C.virConnectPtr, d C.virDomainPtr,
	utcoffset int64, opaque unsafe.Pointer) int {

	context := *(*domainCallbackContext)(opaque)
	domain := VirDomain{ptr: d}
	connection := VirConnection{ptr: c}

	eventDetails := DomainRTCChangeEvent{
		Utcoffset: utcoffset,
	}

	return (*context.cb)(&connection, &domain, eventDetails, context.f)
}

//export domainEventWatchdogCallback
func domainEventWatchdogCallback(c C.virConnectPtr, d C.virDomainPtr,
	action int, opaque unsafe.Pointer) int {

	context := *(*domainCallbackContext)(opaque)
	domain := VirDomain{ptr: d}
	connection := VirConnection{ptr: c}

	eventDetails := DomainWatchdogEvent{
		Action: action,
	}

	return (*context.cb)(&connection, &domain, eventDetails, context.f)
}

//export domainEventIOErrorCallback
func domainEventIOErrorCallback(c C.virConnectPtr, d C.virDomainPtr,
	srcPath string, devAlias string, action int, opaque unsafe.Pointer) int {

	context := *(*domainCallbackContext)(opaque)
	domain := VirDomain{ptr: d}
	connection := VirConnection{ptr: c}

	eventDetails := DomainIOErrorEvent{
		SrcPath:  srcPath,
		DevAlias: devAlias,
		Action:   action,
	}

	return (*context.cb)(&connection, &domain, eventDetails, context.f)
}

//export domainEventGraphicsCallback
func domainEventGraphicsCallback(c C.virConnectPtr, d C.virDomainPtr,
	phase int,
	local C.virDomainEventGraphicsAddressPtr,
	remote C.virDomainEventGraphicsAddressPtr,
	authScheme string,
	subject C.virDomainEventGraphicsSubjectPtr,
	opaque unsafe.Pointer) int {

	context := *(*domainCallbackContext)(opaque)
	domain := VirDomain{ptr: d}
	connection := VirConnection{ptr: c}

	subjectGo := make([]DomainEventGraphicsSubjectIdentity, subject.nidentity)
	nidentities := int(subject.nidentity)
	identities := (*[1 << 30]C.virDomainEventGraphicsSubjectIdentity)(unsafe.Pointer(&subject.identities))[:nidentities:nidentities]
	for _, identity := range identities {
		subjectGo = append(subjectGo,
			DomainEventGraphicsSubjectIdentity{
				Type: C.GoString(identity._type),
				Name: C.GoString(identity.name),
			},
		)
	}

	eventDetails := DomainGraphicsEvent{
		Phase: phase,
		Local: DomainEventGraphicsAddress{
			Family:  int(local.family),
			Node:    C.GoString(local.node),
			Service: C.GoString(local.service),
		},
		Remote: DomainEventGraphicsAddress{
			Family:  int(remote.family),
			Node:    C.GoString(remote.node),
			Service: C.GoString(remote.service),
		},
		AuthScheme: authScheme,
		Subject:    subjectGo,
	}

	return (*context.cb)(&connection, &domain, eventDetails, context.f)
}

//export domainEventIOErrorReasonCallback
func domainEventIOErrorReasonCallback(c C.virConnectPtr, d C.virDomainPtr,
	srcPath string, devAlias string, action int, reason string,
	opaque unsafe.Pointer) int {

	context := *(*domainCallbackContext)(opaque)
	domain := VirDomain{ptr: d}
	connection := VirConnection{ptr: c}

	eventDetails := DomainIOErrorReasonEvent{
		DomainIOErrorEvent: DomainIOErrorEvent{
			SrcPath:  srcPath,
			DevAlias: devAlias,
			Action:   action,
		},
		Reason: reason,
	}

	return (*context.cb)(&connection, &domain, eventDetails, context.f)
}

//export domainEventBlockJobCallback
func domainEventBlockJobCallback(c C.virConnectPtr, d C.virDomainPtr,
	disk string, _type int, status int, opaque unsafe.Pointer) int {

	context := *(*domainCallbackContext)(opaque)
	domain := VirDomain{ptr: d}
	connection := VirConnection{ptr: c}

	eventDetails := DomainBlockJobEvent{
		Disk:   disk,
		Type:   _type,
		Status: status,
	}

	return (*context.cb)(&connection, &domain, eventDetails, context.f)
}

//export domainEventDiskChangeCallback
func domainEventDiskChangeCallback(c C.virConnectPtr, d C.virDomainPtr,
	oldSrcPath string, newSrcPath string, devAlias string,
	reason int, opaque unsafe.Pointer) int {

	context := *(*domainCallbackContext)(opaque)
	domain := VirDomain{ptr: d}
	connection := VirConnection{ptr: c}

	eventDetails := DomainDiskChangeEvent{
		OldSrcPath: oldSrcPath,
		NewSrcPath: newSrcPath,
		DevAlias:   devAlias,
		Reason:     reason,
	}

	return (*context.cb)(&connection, &domain, eventDetails, context.f)
}

//export domainEventTrayChangeCallback
func domainEventTrayChangeCallback(c C.virConnectPtr, d C.virDomainPtr,
	devAlias string, reason int, opaque unsafe.Pointer) int {

	context := *(*domainCallbackContext)(opaque)
	domain := VirDomain{ptr: d}
	connection := VirConnection{ptr: c}

	eventDetails := DomainTrayChangeEvent{
		DevAlias: devAlias,
		Reason:   reason,
	}

	return (*context.cb)(&connection, &domain, eventDetails, context.f)
}

//export domainEventReasonCallback
func domainEventReasonCallback(c C.virConnectPtr, d C.virDomainPtr,
	reason int, opaque unsafe.Pointer) int {

	context := *(*domainCallbackContext)(opaque)
	domain := VirDomain{ptr: d}
	connection := VirConnection{ptr: c}

	eventDetails := DomainReasonEvent{
		Reason: reason,
	}

	return (*context.cb)(&connection, &domain, eventDetails, context.f)
}

//export domainEventBalloonChangeCallback
func domainEventBalloonChangeCallback(c C.virConnectPtr, d C.virDomainPtr,
	actual uint64, opaque unsafe.Pointer) int {

	context := *(*domainCallbackContext)(opaque)
	domain := VirDomain{ptr: d}
	connection := VirConnection{ptr: c}

	eventDetails := DomainBalloonChangeEvent{
		Actual: actual,
	}

	return (*context.cb)(&connection, &domain, eventDetails, context.f)
}

//export domainEventDeviceRemovedCallback
func domainEventDeviceRemovedCallback(c C.virConnectPtr, d C.virDomainPtr,
	devAlias string, opaque unsafe.Pointer) int {

	context := *(*domainCallbackContext)(opaque)
	domain := VirDomain{ptr: d}
	connection := VirConnection{ptr: c}

	eventDetails := DomainDeviceRemovedEvent{
		DevAlias: devAlias,
	}

	return (*context.cb)(&connection, &domain, eventDetails, context.f)
}

type DomainEventCallback func(c *VirConnection, d *VirDomain,
	event interface{}, f func()) int

type domainCallbackContext struct {
	cb *DomainEventCallback
	f  func()
}

func (c *VirConnection) DomainEventRegister(dom VirDomain,
	eventId int,
	callback *DomainEventCallback,
	opaque func()) int {
	var callbackPtr unsafe.Pointer
	context := domainCallbackContext{
		cb: callback,
		f:  opaque,
	}

	switch eventId {
	case VIR_DOMAIN_EVENT_ID_LIFECYCLE:
		callbackPtr = unsafe.Pointer(C.domainEventLifecycleCallback_cgo)
	case VIR_DOMAIN_EVENT_ID_REBOOT:
	case VIR_DOMAIN_EVENT_ID_CONTROL_ERROR:
		callbackPtr = unsafe.Pointer(C.domainEventGenericCallback_cgo)
	case VIR_DOMAIN_EVENT_ID_RTC_CHANGE:
		callbackPtr = unsafe.Pointer(C.domainEventRTCChangeCallback_cgo)
	case VIR_DOMAIN_EVENT_ID_WATCHDOG:
		callbackPtr = unsafe.Pointer(C.domainEventWatchdogCallback_cgo)
	case VIR_DOMAIN_EVENT_ID_IO_ERROR:
		callbackPtr = unsafe.Pointer(C.domainEventIOErrorCallback_cgo)
	case VIR_DOMAIN_EVENT_ID_GRAPHICS:
		callbackPtr = unsafe.Pointer(C.domainEventGraphicsCallback_cgo)
	case VIR_DOMAIN_EVENT_ID_IO_ERROR_REASON:
		callbackPtr = unsafe.Pointer(C.domainEventIOErrorReasonCallback_cgo)
	case VIR_DOMAIN_EVENT_ID_BLOCK_JOB:
		// TODO Post 1.2.4, uncomment later
		// case VIR_DOMAIN_EVENT_ID_BLOCK_JOB_2:
		callbackPtr = unsafe.Pointer(C.domainEventBlockJobCallback_cgo)
	case VIR_DOMAIN_EVENT_ID_DISK_CHANGE:
		callbackPtr = unsafe.Pointer(C.domainEventDiskChangeCallback_cgo)
	case VIR_DOMAIN_EVENT_ID_TRAY_CHANGE:
		callbackPtr = unsafe.Pointer(C.domainEventTrayChangeCallback_cgo)
	case VIR_DOMAIN_EVENT_ID_PMWAKEUP:
	case VIR_DOMAIN_EVENT_ID_PMSUSPEND:
	case VIR_DOMAIN_EVENT_ID_PMSUSPEND_DISK:
		callbackPtr = unsafe.Pointer(C.domainEventReasonCallback_cgo)
	case VIR_DOMAIN_EVENT_ID_BALLOON_CHANGE:
		callbackPtr = unsafe.Pointer(C.domainEventBalloonChangeCallback_cgo)
	case VIR_DOMAIN_EVENT_ID_DEVICE_REMOVED:
		callbackPtr = unsafe.Pointer(C.domainEventDeviceRemovedCallback_cgo)
	default:
	}
	ret := C.virConnectDomainEventRegisterAny(c.ptr, dom.ptr, C.VIR_DOMAIN_EVENT_ID_LIFECYCLE,
		C.virConnectDomainEventGenericCallback(callbackPtr),
		unsafe.Pointer(&context),
		nil)
	return int(ret)
}

func (c *VirConnection) DomainEventDeregister(callbackId int) int {
	// Deregister the callback
	return int(C.virConnectDomainEventDeregisterAny(c.ptr, C.int(callbackId)))
}

func EventRegisterDefaultImpl() int {
	return int(C.virEventRegisterDefaultImpl())
}

func EventRunDefaultImpl() int {
	return int(C.virEventRunDefaultImpl())
}
