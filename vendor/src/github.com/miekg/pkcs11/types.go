// Copyright 2013 Miek Gieben. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkcs11

/*
#define CK_PTR *
#ifndef NULL_PTR
#define NULL_PTR 0
#endif
#define CK_DEFINE_FUNCTION(returnType, name) returnType name
#define CK_DECLARE_FUNCTION(returnType, name) returnType name
#define CK_DECLARE_FUNCTION_POINTER(returnType, name) returnType (* name)
#define CK_CALLBACK_FUNCTION(returnType, name) returnType (* name)

#include <stdlib.h>
#include "pkcs11.h"

CK_ULONG Index(CK_ULONG_PTR array, CK_ULONG i)
{
	return array[i];
}

CK_ULONG Sizeof()
{
	return sizeof(CK_ULONG);
}
*/
import "C"

import (
	"fmt"
	"time"
	"unsafe"
)

// toList converts from a C style array to a []uint.
func toList(clist C.CK_ULONG_PTR, size C.CK_ULONG) []uint {
	l := make([]uint, int(size))
	for i := 0; i < len(l); i++ {
		l[i] = uint(C.Index(clist, C.CK_ULONG(i)))
	}
	defer C.free(unsafe.Pointer(clist))
	return l
}

// cBBool converts a bool to a CK_BBOOL.
func cBBool(x bool) C.CK_BBOOL {
	if x {
		return C.CK_BBOOL(C.CK_TRUE)
	}
	return C.CK_BBOOL(C.CK_FALSE)
}

// Error represents an PKCS#11 error.
type Error uint

func (e Error) Error() string {
	return fmt.Sprintf("pkcs11: 0x%X: %s", uint(e), strerror[uint(e)])
}

func toError(e C.CK_RV) error {
	if e == C.CKR_OK {
		return nil
	}
	return Error(e)
}

/* SessionHandle is a Cryptoki-assigned value that identifies a session. */
type SessionHandle uint

/* ObjectHandle is a token-specific identifier for an object.  */
type ObjectHandle uint

// Version represents any version information from the library.
type Version struct {
	Major byte
	Minor byte
}

func toVersion(version C.CK_VERSION) Version {
	return Version{byte(version.major), byte(version.minor)}
}

// SlotEvent holds the SlotID which for which an slot event (token insertion,
// removal, etc.) occurred.
type SlotEvent struct {
	SlotID uint
}

// Info provides information about the library and hardware used.
type Info struct {
	CryptokiVersion    Version
	ManufacturerID     string
	Flags              uint
	LibraryDescription string
	LibraryVersion     Version
}

/* SlotInfo provides information about a slot. */
type SlotInfo struct {
	SlotDescription string // 64 bytes.
	ManufacturerID  string // 32 bytes.
	Flags           uint
	HardwareVersion Version
	FirmwareVersion Version
}

/* TokenInfo provides information about a token. */
type TokenInfo struct {
	Label              string
	ManufacturerID     string
	Model              string
	SerialNumber       string
	Flags              uint
	MaxSessionCount    uint
	SessionCount       uint
	MaxRwSessionCount  uint
	RwSessionCount     uint
	MaxPinLen          uint
	MinPinLen          uint
	TotalPublicMemory  uint
	FreePublicMemory   uint
	TotalPrivateMemory uint
	FreePrivateMemory  uint
	HardwareVersion    Version
	FirmwareVersion    Version
	UTCTime            string
}

/* SesionInfo provides information about a session. */
type SessionInfo struct {
	SlotID      uint
	State       uint
	Flags       uint
	DeviceError uint
}

// Attribute holds an attribute type/value combination.
type Attribute struct {
	Type  uint
	Value []byte
}

// NewAttribute allocates a Attribute and returns a pointer to it.
// Note that this is merely a convience function, as values returned
// from the HSM are not converted back to Go values, those are just raw
// byte slices.
func NewAttribute(typ uint, x interface{}) *Attribute {
	// This function nicely transforms *to* an attribute, but there is
	// no corresponding function that transform back *from* an attribute,
	// which in PKCS#11 is just an byte array.
	a := new(Attribute)
	a.Type = typ
	if x == nil {
		return a
	}
	switch x.(type) {
	case bool: // create bbool
		if x.(bool) {
			a.Value = []byte{1}
			break
		}
		a.Value = []byte{0}
	case uint, int:
		var y uint
		if _, ok := x.(int); ok {
			y = uint(x.(int))
		}
		if _, ok := x.(uint); ok {
			y = x.(uint)
		}
		// TODO(miek): ugly!
		switch int(C.Sizeof()) {
		case 4:
			a.Value = make([]byte, 4)
			a.Value[0] = byte(y)
			a.Value[1] = byte(y >> 8)
			a.Value[2] = byte(y >> 16)
			a.Value[3] = byte(y >> 24)
		case 8:
			a.Value = make([]byte, 8)
			a.Value[0] = byte(y)
			a.Value[1] = byte(y >> 8)
			a.Value[2] = byte(y >> 16)
			a.Value[3] = byte(y >> 24)
			a.Value[4] = byte(y >> 32)
			a.Value[5] = byte(y >> 40)
			a.Value[6] = byte(y >> 48)
			a.Value[7] = byte(y >> 56)
		}
	case string:
		a.Value = []byte(x.(string))
	case []byte: // just copy
		a.Value = x.([]byte)
	case time.Time: // for CKA_DATE
		a.Value = cDate(x.(time.Time))
	default:
		panic("pkcs11: unhandled attribute type")
	}
	return a
}

// cAttribute returns the start address and the length of an attribute list.
func cAttributeList(a []*Attribute) (C.CK_ATTRIBUTE_PTR, C.CK_ULONG) {
	if len(a) == 0 {
		return nil, 0
	}
	pa := make([]C.CK_ATTRIBUTE, len(a))
	for i := 0; i < len(a); i++ {
		pa[i]._type = C.CK_ATTRIBUTE_TYPE(a[i].Type)
		if a[i].Value == nil {
			continue
		}
		pa[i].pValue = C.CK_VOID_PTR((&a[i].Value[0]))
		pa[i].ulValueLen = C.CK_ULONG(len(a[i].Value))
	}
	return C.CK_ATTRIBUTE_PTR(&pa[0]), C.CK_ULONG(len(a))
}

func cDate(t time.Time) []byte {
	b := make([]byte, 8)
	year, month, day := t.Date()
	y := fmt.Sprintf("%4d", year)
	m := fmt.Sprintf("%02d", month)
	d1 := fmt.Sprintf("%02d", day)
	b[0], b[1], b[2], b[3] = y[0], y[1], y[2], y[3]
	b[4], b[5] = m[0], m[1]
	b[6], b[7] = d1[0], d1[1]
	return b
}

// Mechanism holds an mechanism type/value combination.
type Mechanism struct {
	Mechanism uint
	Parameter []byte
}

func NewMechanism(mech uint, x interface{}) *Mechanism {
	m := new(Mechanism)
	m.Mechanism = mech
	if x == nil {
		return m
	}

	// Add any parameters passed (For now presume always bytes were passed in, is there another case?)
	m.Parameter = x.([]byte)

	return m
}

func cMechanismList(m []*Mechanism) (C.CK_MECHANISM_PTR, C.CK_ULONG) {
	if len(m) == 0 {
		return nil, 0
	}
	pm := make([]C.CK_MECHANISM, len(m))
	for i := 0; i < len(m); i++ {
		pm[i].mechanism = C.CK_MECHANISM_TYPE(m[i].Mechanism)
		if m[i].Parameter == nil {
			continue
		}
		pm[i].pParameter = C.CK_VOID_PTR(&(m[i].Parameter[0]))
		pm[i].ulParameterLen = C.CK_ULONG(len(m[i].Parameter))
	}
	return C.CK_MECHANISM_PTR(&pm[0]), C.CK_ULONG(len(m))
}

// MechanismInfo provides information about a particular mechanism.
type MechanismInfo struct {
	MinKeySize uint
	MaxKeySize uint
	Flags      uint
}
