//go:build solaris
// +build solaris

package zfs

import "strings"

var (
	// List of ZFS properties to retrieve from zfs list command on a Solaris platform
	dsPropList = []string{"name", "origin", "used", "available", "mountpoint", "compression", "type", "volsize", "quota", "referenced"}

	dsPropListOptions = strings.Join(dsPropList, ",")

	// List of Zpool properties to retrieve from zpool list command on a non-Solaris platform
	zpoolPropList = []string{"name", "health", "allocated", "size", "free", "readonly", "dedupratio"}

	zpoolPropListOptions = strings.Join(zpoolPropList, ",")
	zpoolArgs            = []string{"get", "-Hp", zpoolPropListOptions}
)
