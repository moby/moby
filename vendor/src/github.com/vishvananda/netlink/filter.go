package netlink

import (
	"errors"
	"fmt"
	"github.com/vishvananda/netlink/nl"
)

type Filter interface {
	Attrs() *FilterAttrs
	Type() string
}

// FilterAttrs represents a netlink filter. A filter is associated with a link,
// has a handle and a parent. The root filter of a device should have a
// parent == HANDLE_ROOT.
type FilterAttrs struct {
	LinkIndex int
	Handle    uint32
	Parent    uint32
	Priority  uint16 // lower is higher priority
	Protocol  uint16 // syscall.ETH_P_*
}

func (q FilterAttrs) String() string {
	return fmt.Sprintf("{LinkIndex: %d, Handle: %s, Parent: %s, Priority: %d, Protocol: %d}", q.LinkIndex, HandleStr(q.Handle), HandleStr(q.Parent), q.Priority, q.Protocol)
}

// Action represents an action in any supported filter.
type Action interface {
	Type() string
}

type BpfAction struct {
	nl.TcActBpf
	Fd   int
	Name string
}

func (action *BpfAction) Type() string {
	return "bpf"
}

type MirredAction struct {
	nl.TcMirred
}

func (action *MirredAction) Type() string {
	return "mirred"
}

func NewMirredAction(redirIndex int) *MirredAction {
	return &MirredAction{
		TcMirred: nl.TcMirred{
			TcGen:   nl.TcGen{Action: nl.TC_ACT_STOLEN},
			Eaction: nl.TCA_EGRESS_REDIR,
			Ifindex: uint32(redirIndex),
		},
	}
}

// U32 filters on many packet related properties
type U32 struct {
	FilterAttrs
	ClassId    uint32
	RedirIndex int
	Actions    []Action
}

func (filter *U32) Attrs() *FilterAttrs {
	return &filter.FilterAttrs
}

func (filter *U32) Type() string {
	return "u32"
}

type FilterFwAttrs struct {
	ClassId   uint32
	InDev     string
	Mask      uint32
	Index     uint32
	Buffer    uint32
	Mtu       uint32
	Mpu       uint16
	Rate      uint32
	AvRate    uint32
	PeakRate  uint32
	Action    int
	Overhead  uint16
	LinkLayer int
}

// Fw filter filters on firewall marks
type Fw struct {
	FilterAttrs
	ClassId uint32
	Police  nl.TcPolice
	InDev   string
	// TODO Action
	Mask   uint32
	AvRate uint32
	Rtab   [256]uint32
	Ptab   [256]uint32
}

func NewFw(attrs FilterAttrs, fattrs FilterFwAttrs) (*Fw, error) {
	var rtab [256]uint32
	var ptab [256]uint32
	rcellLog := -1
	pcellLog := -1
	avrate := fattrs.AvRate / 8
	police := nl.TcPolice{}
	police.Rate.Rate = fattrs.Rate / 8
	police.PeakRate.Rate = fattrs.PeakRate / 8
	buffer := fattrs.Buffer
	linklayer := nl.LINKLAYER_ETHERNET

	if fattrs.LinkLayer != nl.LINKLAYER_UNSPEC {
		linklayer = fattrs.LinkLayer
	}

	police.Action = int32(fattrs.Action)
	if police.Rate.Rate != 0 {
		police.Rate.Mpu = fattrs.Mpu
		police.Rate.Overhead = fattrs.Overhead
		if CalcRtable(&police.Rate, rtab, rcellLog, fattrs.Mtu, linklayer) < 0 {
			return nil, errors.New("TBF: failed to calculate rate table")
		}
		police.Burst = uint32(Xmittime(uint64(police.Rate.Rate), uint32(buffer)))
	}
	police.Mtu = fattrs.Mtu
	if police.PeakRate.Rate != 0 {
		police.PeakRate.Mpu = fattrs.Mpu
		police.PeakRate.Overhead = fattrs.Overhead
		if CalcRtable(&police.PeakRate, ptab, pcellLog, fattrs.Mtu, linklayer) < 0 {
			return nil, errors.New("POLICE: failed to calculate peak rate table")
		}
	}

	return &Fw{
		FilterAttrs: attrs,
		ClassId:     fattrs.ClassId,
		InDev:       fattrs.InDev,
		Mask:        fattrs.Mask,
		Police:      police,
		AvRate:      avrate,
		Rtab:        rtab,
		Ptab:        ptab,
	}, nil
}

func (filter *Fw) Attrs() *FilterAttrs {
	return &filter.FilterAttrs
}

func (filter *Fw) Type() string {
	return "fw"
}

type BpfFilter struct {
	FilterAttrs
	ClassId      uint32
	Fd           int
	Name         string
	DirectAction bool
}

func (filter *BpfFilter) Type() string {
	return "bpf"
}

func (filter *BpfFilter) Attrs() *FilterAttrs {
	return &filter.FilterAttrs
}

// GenericFilter filters represent types that are not currently understood
// by this netlink library.
type GenericFilter struct {
	FilterAttrs
	FilterType string
}

func (filter *GenericFilter) Attrs() *FilterAttrs {
	return &filter.FilterAttrs
}

func (filter *GenericFilter) Type() string {
	return filter.FilterType
}
