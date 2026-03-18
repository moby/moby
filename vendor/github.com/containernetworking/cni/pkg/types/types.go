// Copyright 2015 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
)

// like net.IPNet but adds JSON marshalling and unmarshalling
type IPNet net.IPNet

// ParseCIDR takes a string like "10.2.3.1/24" and
// return IPNet with "10.2.3.1" and /24 mask
func ParseCIDR(s string) (*net.IPNet, error) {
	ip, ipn, err := net.ParseCIDR(s)
	if err != nil {
		return nil, err
	}

	ipn.IP = ip
	return ipn, nil
}

func (n IPNet) MarshalJSON() ([]byte, error) {
	return json.Marshal((*net.IPNet)(&n).String())
}

func (n *IPNet) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	tmp, err := ParseCIDR(s)
	if err != nil {
		return err
	}

	*n = IPNet(*tmp)
	return nil
}

// Use PluginConf instead of NetConf, the NetConf
// backwards-compat alias will be removed in a future release.
type NetConf = PluginConf

// PluginConf describes a plugin configuration for a specific network.
type PluginConf struct {
	CNIVersion string `json:"cniVersion,omitempty"`

	Name         string          `json:"name,omitempty"`
	Type         string          `json:"type,omitempty"`
	Capabilities map[string]bool `json:"capabilities,omitempty"`
	IPAM         IPAM            `json:"ipam,omitempty"`
	DNS          DNS             `json:"dns,omitempty"`

	RawPrevResult map[string]interface{} `json:"prevResult,omitempty"`
	PrevResult    Result                 `json:"-"`

	// ValidAttachments is only supplied when executing a GC operation
	ValidAttachments []GCAttachment `json:"cni.dev/valid-attachments,omitempty"`
}

// GCAttachment is the parameters to a GC call -- namely,
// the container ID and ifname pair that represents a
// still-valid attachment.
type GCAttachment struct {
	ContainerID string `json:"containerID"`
	IfName      string `json:"ifname"`
}

// Note: DNS should be omit if DNS is empty but default Marshal function
// will output empty structure hence need to write a Marshal function
func (n *PluginConf) MarshalJSON() ([]byte, error) {
	bytes, err := json.Marshal(*n)
	if err != nil {
		return nil, err
	}

	fixupObj := make(map[string]interface{})
	if err := json.Unmarshal(bytes, &fixupObj); err != nil {
		return nil, err
	}

	if n.DNS.IsEmpty() {
		delete(fixupObj, "dns")
	}

	return json.Marshal(fixupObj)
}

type IPAM struct {
	Type string `json:"type,omitempty"`
}

// IsEmpty returns true if IPAM structure has no value, otherwise return false
func (i *IPAM) IsEmpty() bool {
	return i.Type == ""
}

// NetConfList describes an ordered list of networks.
type NetConfList struct {
	CNIVersion string `json:"cniVersion,omitempty"`

	Name         string        `json:"name,omitempty"`
	DisableCheck bool          `json:"disableCheck,omitempty"`
	DisableGC    bool          `json:"disableGC,omitempty"`
	Plugins      []*PluginConf `json:"plugins,omitempty"`
}

// Result is an interface that provides the result of plugin execution
type Result interface {
	// The highest CNI specification result version the result supports
	// without having to convert
	Version() string

	// Returns the result converted into the requested CNI specification
	// result version, or an error if conversion failed
	GetAsVersion(version string) (Result, error)

	// Prints the result in JSON format to stdout
	Print() error

	// Prints the result in JSON format to provided writer
	PrintTo(writer io.Writer) error
}

func PrintResult(result Result, version string) error {
	newResult, err := result.GetAsVersion(version)
	if err != nil {
		return err
	}
	return newResult.Print()
}

// DNS contains values interesting for DNS resolvers
type DNS struct {
	Nameservers []string `json:"nameservers,omitempty"`
	Domain      string   `json:"domain,omitempty"`
	Search      []string `json:"search,omitempty"`
	Options     []string `json:"options,omitempty"`
}

// IsEmpty returns true if DNS structure has no value, otherwise return false
func (d *DNS) IsEmpty() bool {
	if len(d.Nameservers) == 0 && d.Domain == "" && len(d.Search) == 0 && len(d.Options) == 0 {
		return true
	}
	return false
}

func (d *DNS) Copy() *DNS {
	if d == nil {
		return nil
	}

	to := &DNS{Domain: d.Domain}
	to.Nameservers = append(to.Nameservers, d.Nameservers...)
	to.Search = append(to.Search, d.Search...)
	to.Options = append(to.Options, d.Options...)
	return to
}

type Route struct {
	Dst      net.IPNet
	GW       net.IP
	MTU      int
	AdvMSS   int
	Priority int
	Table    *int
	Scope    *int
}

func (r *Route) String() string {
	table := "<nil>"
	if r.Table != nil {
		table = fmt.Sprintf("%d", *r.Table)
	}

	scope := "<nil>"
	if r.Scope != nil {
		scope = fmt.Sprintf("%d", *r.Scope)
	}

	return fmt.Sprintf("{Dst:%+v GW:%v MTU:%d AdvMSS:%d Priority:%d Table:%s Scope:%s}", r.Dst, r.GW, r.MTU, r.AdvMSS, r.Priority, table, scope)
}

func (r *Route) Copy() *Route {
	if r == nil {
		return nil
	}

	route := &Route{
		Dst:      r.Dst,
		GW:       r.GW,
		MTU:      r.MTU,
		AdvMSS:   r.AdvMSS,
		Priority: r.Priority,
		Scope:    r.Scope,
	}

	if r.Table != nil {
		table := *r.Table
		route.Table = &table
	}

	if r.Scope != nil {
		scope := *r.Scope
		route.Scope = &scope
	}

	return route
}

// Well known error codes
// see https://github.com/containernetworking/cni/blob/main/SPEC.md#well-known-error-codes
const (
	ErrUnknown                     uint = iota // 0
	ErrIncompatibleCNIVersion                  // 1
	ErrUnsupportedField                        // 2
	ErrUnknownContainer                        // 3
	ErrInvalidEnvironmentVariables             // 4
	ErrIOFailure                               // 5
	ErrDecodingFailure                         // 6
	ErrInvalidNetworkConfig                    // 7
	ErrInvalidNetNS                            // 8
	ErrTryAgainLater               uint = 11
	ErrInternal                    uint = 999
)

type Error struct {
	Code    uint   `json:"code"`
	Msg     string `json:"msg"`
	Details string `json:"details,omitempty"`
}

func NewError(code uint, msg, details string) *Error {
	return &Error{
		Code:    code,
		Msg:     msg,
		Details: details,
	}
}

func (e *Error) Error() string {
	details := ""
	if e.Details != "" {
		details = fmt.Sprintf("; %v", e.Details)
	}
	return fmt.Sprintf("%v%v", e.Msg, details)
}

func (e *Error) Print() error {
	return prettyPrint(e)
}

// net.IPNet is not JSON (un)marshallable so this duality is needed
// for our custom IPNet type

// JSON (un)marshallable types
type route struct {
	Dst      IPNet  `json:"dst"`
	GW       net.IP `json:"gw,omitempty"`
	MTU      int    `json:"mtu,omitempty"`
	AdvMSS   int    `json:"advmss,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Table    *int   `json:"table,omitempty"`
	Scope    *int   `json:"scope,omitempty"`
}

func (r *Route) UnmarshalJSON(data []byte) error {
	rt := route{}
	if err := json.Unmarshal(data, &rt); err != nil {
		return err
	}

	r.Dst = net.IPNet(rt.Dst)
	r.GW = rt.GW
	r.MTU = rt.MTU
	r.AdvMSS = rt.AdvMSS
	r.Priority = rt.Priority
	r.Table = rt.Table
	r.Scope = rt.Scope

	return nil
}

func (r Route) MarshalJSON() ([]byte, error) {
	rt := route{
		Dst:      IPNet(r.Dst),
		GW:       r.GW,
		MTU:      r.MTU,
		AdvMSS:   r.AdvMSS,
		Priority: r.Priority,
		Table:    r.Table,
		Scope:    r.Scope,
	}

	return json.Marshal(rt)
}

func prettyPrint(obj interface{}) error {
	data, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}
