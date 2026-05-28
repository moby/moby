// Copyright 2023 The Capability Authors.
// Copyright 2013 Suryandaru Triandana <syndtr@gmail.com>
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package capability provides utilities for manipulating POSIX capabilities.
package capability

type Capabilities interface {
	// Get check whether a capability present in the given
	// capabilities set. The 'which' value should be one of EFFECTIVE,
	// PERMITTED, INHERITABLE, BOUNDING or AMBIENT.
	Get(which CapType, what Cap) bool

	// Empty check whether all capability bits of the given capabilities
	// set are zero. The 'which' value should be one of EFFECTIVE,
	// PERMITTED, INHERITABLE, BOUNDING or AMBIENT.
	Empty(which CapType) bool

	// Full check whether all capability bits of the given capabilities
	// set are one. The 'which' value should be one of EFFECTIVE,
	// PERMITTED, INHERITABLE, BOUNDING or AMBIENT.
	Full(which CapType) bool

	// Set sets capabilities of the given capabilities sets. The
	// 'which' value should be one or combination (OR'ed) of EFFECTIVE,
	// PERMITTED, INHERITABLE, BOUNDING or AMBIENT.
	Set(which CapType, caps ...Cap)

	// Unset unsets capabilities of the given capabilities sets. The
	// 'which' value should be one or combination (OR'ed) of EFFECTIVE,
	// PERMITTED, INHERITABLE, BOUNDING or AMBIENT.
	Unset(which CapType, caps ...Cap)

	// Fill sets all bits of the given capabilities kind to one. The
	// 'kind' value should be one or combination (OR'ed) of CAPS,
	// BOUNDS or AMBS.
	Fill(kind CapType)

	// Clear sets all bits of the given capabilities kind to zero. The
	// 'kind' value should be one or combination (OR'ed) of CAPS,
	// BOUNDS or AMBS.
	Clear(kind CapType)

	// String return current capabilities state of the given capabilities
	// set as string. The 'which' value should be one of EFFECTIVE,
	// PERMITTED, INHERITABLE BOUNDING or AMBIENT
	StringCap(which CapType) string

	// String return current capabilities state as string.
	String() string

	// Load load actual capabilities value. This will overwrite all
	// outstanding changes.
	Load() error

	// Apply apply the capabilities settings, so all changes made by
	// [Set], [Unset], [Fill], or [Clear] will take effect.
	Apply(kind CapType) error
}

// NewPid initializes a new [Capabilities] object for given pid when
// it is nonzero, or for the current process if pid is 0.
//
// Deprecated: replace with [NewPid2] followed by optional [Capabilities.Load]
// (only if needed). For example, replace:
//
//	c, err := NewPid(0)
//	if err != nil {
//		return err
//	}
//
// with:
//
//	c, err := NewPid2(0)
//	if err != nil {
//		return err
//	}
//	err = c.Load()
//	if err != nil {
//		return err
//	}
func NewPid(pid int) (Capabilities, error) {
	c, err := newPid(pid)
	if err != nil {
		return c, err
	}
	err = c.Load()
	return c, err
}

// NewPid2 initializes a new [Capabilities] object for given pid when
// it is nonzero, or for the current process if pid is 0. This
// does not load the process's current capabilities; if needed,
// call [Capabilities.Load].
func NewPid2(pid int) (Capabilities, error) {
	return newPid(pid)
}

// NewFile initializes a new Capabilities object for given file path.
//
// Deprecated: replace with [NewFile2] followed by optional [Capabilities.Load]
// (only if needed). For example, replace:
//
//	c, err := NewFile(path)
//	if err != nil {
//		return err
//	}
//
// with:
//
//	c, err := NewFile2(path)
//	if err != nil {
//		return err
//	}
//	err = c.Load()
//	if err != nil {
//		return err
//	}
func NewFile(path string) (Capabilities, error) {
	c, err := newFile(path)
	if err != nil {
		return c, err
	}
	err = c.Load()
	return c, err
}

// NewFile2 creates a new initialized [Capabilities] object for given
// file path. This does not load the process's current capabilities;
// if needed, call [Capabilities.Load].
func NewFile2(path string) (Capabilities, error) {
	return newFile(path)
}

// LastCap returns highest valid capability of the running kernel,
// or an error if it can not be obtained.
//
// See also: [ListSupported].
func LastCap() (Cap, error) {
	return lastCap()
}

// GetAmbient determines if a specific ambient capability is raised in the
// calling thread.
func GetAmbient(c Cap) (bool, error) {
	return getAmbient(c)
}

// SetAmbient raises or lowers specified ambient capabilities for the calling
// thread. To complete successfully, the prevailing effective capability set
// must have a raised CAP_SETPCAP. Further, to raise a specific ambient
// capability the inheritable and permitted sets of the calling thread must
// already contain the specified capability.
func SetAmbient(raise bool, caps ...Cap) error {
	return setAmbient(raise, caps...)
}

// ResetAmbient resets all of the ambient capabilities for the calling thread
// to their lowered value.
func ResetAmbient() error {
	return resetAmbient()
}

// GetBound determines if a specific bounding capability is raised in the
// calling thread.
func GetBound(c Cap) (bool, error) {
	return getBound(c)
}

// DropBound lowers the specified bounding set capability.
func DropBound(caps ...Cap) error {
	return dropBound(caps...)
}
