package libvirt

import (
	"unsafe"
)

/*
#cgo LDFLAGS: -lvirt -ldl
#include <libvirt/libvirt.h>
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
