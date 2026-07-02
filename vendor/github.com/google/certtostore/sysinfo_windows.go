//go:build windows
// +build windows

// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package certtostore

import (
	"errors"
	"fmt"
	"os"
	"os/user"

	"github.com/StackExchange/wmi"
)

var (
	// ErrNoNetworkAdapter is returned when no network adapter is detected.
	ErrNoNetworkAdapter = errors.New("network adapter not detected")
)

// User will obtain the current user from the OS.
func User() (string, error) {
	if u := os.Getenv("USERNAME"); u != "" {
		return u, nil
	}
	return "", errors.New("could not determine the user")
}

// UserSID will obtain the SID of the current user from the OS.
func UserSID() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("could not determine the user: %v", err)
	}
	if u.Uid == "" {
		return "", fmt.Errorf("SID for %q was blank", u.Username)
	}
	return u.Uid, nil
}

// Constants for DomainRole
// https://msdn.microsoft.com/en-us/library/aa394102
const (
	StandaloneWorkstation = iota
	MemberWorkstation
	StandaloneServer
	MemberServer
	BackupDomainController
	PrimaryDomainController
)

// Win32_ComputerSystem is used to store WMI query results
type Win32_ComputerSystem struct {
	DNSHostName string
	Domain      string
	DomainRole  int
	Model       string
}

// CompInfo populates a struct with computer information through WMI
func CompInfo() (*Win32_ComputerSystem, error) {
	var result []Win32_ComputerSystem
	if err := wmi.Query(wmi.CreateQuery(&result, ""), &result); err != nil {
		return nil, err
	}
	if result[0].DNSHostName == "" {
		return nil, errors.New("could not determine the DNS Host Name")
	}
	if result[0].Domain == "" {
		return nil, errors.New("could not determine the Domain")
	}
	if result[0].Model == "" {
		return nil, errors.New("could not determine the system model")
	}
	return &result[0], nil
}

// Win32_ComputerSystemProduct is used to obtain the UUID of the computer main board
type Win32_ComputerSystemProduct struct {
	Vendor, UUID, IdentifyingNumber string
}

// CompProdInfo populates a struct with computer system product information through WMI
func CompProdInfo() (*Win32_ComputerSystemProduct, error) {
	var compProdInfo []Win32_ComputerSystemProduct
	if err := wmi.Query(wmi.CreateQuery(&compProdInfo, ""), &compProdInfo); err != nil {
		return nil, err
	}
	return &compProdInfo[0], nil
}

// Win32_NetworkAdapter is used to ID the physical local network adapter through WMI
type Win32_NetworkAdapter struct {
	MACAddress string
}

// NetInfo obtains the mac address of all local non-USB network devices
func NetInfo() ([]string, error) {
	var netInfo []Win32_NetworkAdapter
	if err := wmi.Query(wmi.CreateQuery(&netInfo, "where PNPDeviceID LIKE \"%PCI%\" AND AdapterTypeID=0"), &netInfo); err != nil {
		return nil, err
	}

	if len(netInfo) == 0 {
		return nil, fmt.Errorf("%w got: %d want: >= 1", ErrNoNetworkAdapter, len(netInfo))
	}

	var macs []string
	for _, adapter := range netInfo {
		macs = append(macs, adapter.MACAddress)
	}

	return macs, nil
}
