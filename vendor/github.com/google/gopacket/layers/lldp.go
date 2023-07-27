// Copyright 2012 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/google/gopacket"
)

// LLDPTLVType is the type of each TLV value in a LinkLayerDiscovery packet.
type LLDPTLVType byte

const (
	LLDPTLVEnd             LLDPTLVType = 0
	LLDPTLVChassisID       LLDPTLVType = 1
	LLDPTLVPortID          LLDPTLVType = 2
	LLDPTLVTTL             LLDPTLVType = 3
	LLDPTLVPortDescription LLDPTLVType = 4
	LLDPTLVSysName         LLDPTLVType = 5
	LLDPTLVSysDescription  LLDPTLVType = 6
	LLDPTLVSysCapabilities LLDPTLVType = 7
	LLDPTLVMgmtAddress     LLDPTLVType = 8
	LLDPTLVOrgSpecific     LLDPTLVType = 127
)

// LinkLayerDiscoveryValue is a TLV value inside a LinkLayerDiscovery packet layer.
type LinkLayerDiscoveryValue struct {
	Type   LLDPTLVType
	Length uint16
	Value  []byte
}

func (c *LinkLayerDiscoveryValue) len() int {
	return 0
}

// LLDPChassisIDSubType specifies the value type for a single LLDPChassisID.ID
type LLDPChassisIDSubType byte

// LLDP Chassis Types
const (
	LLDPChassisIDSubTypeReserved    LLDPChassisIDSubType = 0
	LLDPChassisIDSubTypeChassisComp LLDPChassisIDSubType = 1
	LLDPChassisIDSubtypeIfaceAlias  LLDPChassisIDSubType = 2
	LLDPChassisIDSubTypePortComp    LLDPChassisIDSubType = 3
	LLDPChassisIDSubTypeMACAddr     LLDPChassisIDSubType = 4
	LLDPChassisIDSubTypeNetworkAddr LLDPChassisIDSubType = 5
	LLDPChassisIDSubtypeIfaceName   LLDPChassisIDSubType = 6
	LLDPChassisIDSubTypeLocal       LLDPChassisIDSubType = 7
)

type LLDPChassisID struct {
	Subtype LLDPChassisIDSubType
	ID      []byte
}

func (c *LLDPChassisID) serialize() []byte {

	var buf = make([]byte, c.serializedLen())
	idLen := uint16(LLDPTLVChassisID)<<9 | uint16(len(c.ID)+1) //id should take 7 bits, length should take 9 bits, +1 for subtype
	binary.BigEndian.PutUint16(buf[0:2], idLen)
	buf[2] = byte(c.Subtype)
	copy(buf[3:], c.ID)
	return buf
}

func (c *LLDPChassisID) serializedLen() int {
	return len(c.ID) + 3 // +2 for id and length, +1 for subtype
}

// LLDPPortIDSubType specifies the value type for a single LLDPPortID.ID
type LLDPPortIDSubType byte

// LLDP PortID types
const (
	LLDPPortIDSubtypeReserved       LLDPPortIDSubType = 0
	LLDPPortIDSubtypeIfaceAlias     LLDPPortIDSubType = 1
	LLDPPortIDSubtypePortComp       LLDPPortIDSubType = 2
	LLDPPortIDSubtypeMACAddr        LLDPPortIDSubType = 3
	LLDPPortIDSubtypeNetworkAddr    LLDPPortIDSubType = 4
	LLDPPortIDSubtypeIfaceName      LLDPPortIDSubType = 5
	LLDPPortIDSubtypeAgentCircuitID LLDPPortIDSubType = 6
	LLDPPortIDSubtypeLocal          LLDPPortIDSubType = 7
)

type LLDPPortID struct {
	Subtype LLDPPortIDSubType
	ID      []byte
}

func (c *LLDPPortID) serialize() []byte {

	var buf = make([]byte, c.serializedLen())
	idLen := uint16(LLDPTLVPortID)<<9 | uint16(len(c.ID)+1) //id should take 7 bits, length should take 9 bits, +1 for subtype
	binary.BigEndian.PutUint16(buf[0:2], idLen)
	buf[2] = byte(c.Subtype)
	copy(buf[3:], c.ID)
	return buf
}

func (c *LLDPPortID) serializedLen() int {
	return len(c.ID) + 3 // +2 for id and length, +1 for subtype
}

// LinkLayerDiscovery is a packet layer containing the LinkLayer Discovery Protocol.
// See http:http://standards.ieee.org/getieee802/download/802.1AB-2009.pdf
// ChassisID, PortID and TTL are mandatory TLV's. Other values can be decoded
// with DecodeValues()
type LinkLayerDiscovery struct {
	BaseLayer
	ChassisID LLDPChassisID
	PortID    LLDPPortID
	TTL       uint16
	Values    []LinkLayerDiscoveryValue
}

type IEEEOUI uint32

// http://standards.ieee.org/develop/regauth/oui/oui.txt
const (
	IEEEOUI8021     IEEEOUI = 0x0080c2
	IEEEOUI8023     IEEEOUI = 0x00120f
	IEEEOUI80211    IEEEOUI = 0x000fac
	IEEEOUI8021Qbg  IEEEOUI = 0x0013BF
	IEEEOUICisco2   IEEEOUI = 0x000142
	IEEEOUIMedia    IEEEOUI = 0x0012bb // TR-41
	IEEEOUIProfinet IEEEOUI = 0x000ecf
	IEEEOUIDCBX     IEEEOUI = 0x001b21
)

// LLDPOrgSpecificTLV is an Organisation-specific TLV
type LLDPOrgSpecificTLV struct {
	OUI     IEEEOUI
	SubType uint8
	Info    []byte
}

// LLDPCapabilities Types
const (
	LLDPCapsOther       uint16 = 1 << 0
	LLDPCapsRepeater    uint16 = 1 << 1
	LLDPCapsBridge      uint16 = 1 << 2
	LLDPCapsWLANAP      uint16 = 1 << 3
	LLDPCapsRouter      uint16 = 1 << 4
	LLDPCapsPhone       uint16 = 1 << 5
	LLDPCapsDocSis      uint16 = 1 << 6
	LLDPCapsStationOnly uint16 = 1 << 7
	LLDPCapsCVLAN       uint16 = 1 << 8
	LLDPCapsSVLAN       uint16 = 1 << 9
	LLDPCapsTmpr        uint16 = 1 << 10
)

// LLDPCapabilities represents the capabilities of a device
type LLDPCapabilities struct {
	Other       bool
	Repeater    bool
	Bridge      bool
	WLANAP      bool
	Router      bool
	Phone       bool
	DocSis      bool
	StationOnly bool
	CVLAN       bool
	SVLAN       bool
	TMPR        bool
}

type LLDPSysCapabilities struct {
	SystemCap  LLDPCapabilities
	EnabledCap LLDPCapabilities
}

type IANAAddressFamily byte

// LLDP Management Address Subtypes
// http://www.iana.org/assignments/address-family-numbers/address-family-numbers.xml
const (
	IANAAddressFamilyReserved IANAAddressFamily = 0
	IANAAddressFamilyIPV4     IANAAddressFamily = 1
	IANAAddressFamilyIPV6     IANAAddressFamily = 2
	IANAAddressFamilyNSAP     IANAAddressFamily = 3
	IANAAddressFamilyHDLC     IANAAddressFamily = 4
	IANAAddressFamilyBBN1822  IANAAddressFamily = 5
	IANAAddressFamily802      IANAAddressFamily = 6
	IANAAddressFamilyE163     IANAAddressFamily = 7
	IANAAddressFamilyE164     IANAAddressFamily = 8
	IANAAddressFamilyF69      IANAAddressFamily = 9
	IANAAddressFamilyX121     IANAAddressFamily = 10
	IANAAddressFamilyIPX      IANAAddressFamily = 11
	IANAAddressFamilyAtalk    IANAAddressFamily = 12
	IANAAddressFamilyDecnet   IANAAddressFamily = 13
	IANAAddressFamilyBanyan   IANAAddressFamily = 14
	IANAAddressFamilyE164NSAP IANAAddressFamily = 15
	IANAAddressFamilyDNS      IANAAddressFamily = 16
	IANAAddressFamilyDistname IANAAddressFamily = 17
	IANAAddressFamilyASNumber IANAAddressFamily = 18
	IANAAddressFamilyXTPIPV4  IANAAddressFamily = 19
	IANAAddressFamilyXTPIPV6  IANAAddressFamily = 20
	IANAAddressFamilyXTP      IANAAddressFamily = 21
	IANAAddressFamilyFcWWPN   IANAAddressFamily = 22
	IANAAddressFamilyFcWWNN   IANAAddressFamily = 23
	IANAAddressFamilyGWID     IANAAddressFamily = 24
	IANAAddressFamilyL2VPN    IANAAddressFamily = 25
)

type LLDPInterfaceSubtype byte

// LLDP Interface Subtypes
const (
	LLDPInterfaceSubtypeUnknown LLDPInterfaceSubtype = 1
	LLDPInterfaceSubtypeifIndex LLDPInterfaceSubtype = 2
	LLDPInterfaceSubtypeSysPort LLDPInterfaceSubtype = 3
)

type LLDPMgmtAddress struct {
	Subtype          IANAAddressFamily
	Address          []byte
	InterfaceSubtype LLDPInterfaceSubtype
	InterfaceNumber  uint32
	OID              string
}

// LinkLayerDiscoveryInfo represents the decoded details for a set of LinkLayerDiscoveryValues
// Organisation-specific TLV's can be decoded using the various Decode() methods
type LinkLayerDiscoveryInfo struct {
	BaseLayer
	PortDescription string
	SysName         string
	SysDescription  string
	SysCapabilities LLDPSysCapabilities
	MgmtAddress     LLDPMgmtAddress
	OrgTLVs         []LLDPOrgSpecificTLV      // Private TLVs
	Unknown         []LinkLayerDiscoveryValue // undecoded TLVs
}

/// IEEE 802.1 TLV Subtypes
const (
	LLDP8021SubtypePortVLANID       uint8 = 1
	LLDP8021SubtypeProtocolVLANID   uint8 = 2
	LLDP8021SubtypeVLANName         uint8 = 3
	LLDP8021SubtypeProtocolIdentity uint8 = 4
	LLDP8021SubtypeVDIUsageDigest   uint8 = 5
	LLDP8021SubtypeManagementVID    uint8 = 6
	LLDP8021SubtypeLinkAggregation  uint8 = 7
)

// VLAN Port Protocol ID options
const (
	LLDPProtocolVLANIDCapability byte = 1 << 1
	LLDPProtocolVLANIDStatus     byte = 1 << 2
)

type PortProtocolVLANID struct {
	Supported bool
	Enabled   bool
	ID        uint16
}

type VLANName struct {
	ID   uint16
	Name string
}

type ProtocolIdentity []byte

// LACP options
const (
	LLDPAggregationCapability byte = 1 << 0
	LLDPAggregationStatus     byte = 1 << 1
)

// IEEE 802 Link Aggregation parameters
type LLDPLinkAggregation struct {
	Supported bool
	Enabled   bool
	PortID    uint32
}

// LLDPInfo8021 represents the information carried in 802.1 Org-specific TLVs
type LLDPInfo8021 struct {
	PVID               uint16
	PPVIDs             []PortProtocolVLANID
	VLANNames          []VLANName
	ProtocolIdentities []ProtocolIdentity
	VIDUsageDigest     uint32
	ManagementVID      uint16
	LinkAggregation    LLDPLinkAggregation
}

// IEEE 802.3 TLV Subtypes
const (
	LLDP8023SubtypeMACPHY          uint8 = 1
	LLDP8023SubtypeMDIPower        uint8 = 2
	LLDP8023SubtypeLinkAggregation uint8 = 3
	LLDP8023SubtypeMTU             uint8 = 4
)

// MACPHY options
const (
	LLDPMACPHYCapability byte = 1 << 0
	LLDPMACPHYStatus     byte = 1 << 1
)

// From IANA-MAU-MIB (introduced by RFC 4836) - dot3MauType
const (
	LLDPMAUTypeUnknown         uint16 = 0
	LLDPMAUTypeAUI             uint16 = 1
	LLDPMAUType10Base5         uint16 = 2
	LLDPMAUTypeFOIRL           uint16 = 3
	LLDPMAUType10Base2         uint16 = 4
	LLDPMAUType10BaseT         uint16 = 5
	LLDPMAUType10BaseFP        uint16 = 6
	LLDPMAUType10BaseFB        uint16 = 7
	LLDPMAUType10BaseFL        uint16 = 8
	LLDPMAUType10BROAD36       uint16 = 9
	LLDPMAUType10BaseT_HD      uint16 = 10
	LLDPMAUType10BaseT_FD      uint16 = 11
	LLDPMAUType10BaseFL_HD     uint16 = 12
	LLDPMAUType10BaseFL_FD     uint16 = 13
	LLDPMAUType100BaseT4       uint16 = 14
	LLDPMAUType100BaseTX_HD    uint16 = 15
	LLDPMAUType100BaseTX_FD    uint16 = 16
	LLDPMAUType100BaseFX_HD    uint16 = 17
	LLDPMAUType100BaseFX_FD    uint16 = 18
	LLDPMAUType100BaseT2_HD    uint16 = 19
	LLDPMAUType100BaseT2_FD    uint16 = 20
	LLDPMAUType1000BaseX_HD    uint16 = 21
	LLDPMAUType1000BaseX_FD    uint16 = 22
	LLDPMAUType1000BaseLX_HD   uint16 = 23
	LLDPMAUType1000BaseLX_FD   uint16 = 24
	LLDPMAUType1000BaseSX_HD   uint16 = 25
	LLDPMAUType1000BaseSX_FD   uint16 = 26
	LLDPMAUType1000BaseCX_HD   uint16 = 27
	LLDPMAUType1000BaseCX_FD   uint16 = 28
	LLDPMAUType1000BaseT_HD    uint16 = 29
	LLDPMAUType1000BaseT_FD    uint16 = 30
	LLDPMAUType10GBaseX        uint16 = 31
	LLDPMAUType10GBaseLX4      uint16 = 32
	LLDPMAUType10GBaseR        uint16 = 33
	LLDPMAUType10GBaseER       uint16 = 34
	LLDPMAUType10GBaseLR       uint16 = 35
	LLDPMAUType10GBaseSR       uint16 = 36
	LLDPMAUType10GBaseW        uint16 = 37
	LLDPMAUType10GBaseEW       uint16 = 38
	LLDPMAUType10GBaseLW       uint16 = 39
	LLDPMAUType10GBaseSW       uint16 = 40
	LLDPMAUType10GBaseCX4      uint16 = 41
	LLDPMAUType2BaseTL         uint16 = 42
	LLDPMAUType10PASS_TS       uint16 = 43
	LLDPMAUType100BaseBX10D    uint16 = 44
	LLDPMAUType100BaseBX10U    uint16 = 45
	LLDPMAUType100BaseLX10     uint16 = 46
	LLDPMAUType1000BaseBX10D   uint16 = 47
	LLDPMAUType1000BaseBX10U   uint16 = 48
	LLDPMAUType1000BaseLX10    uint16 = 49
	LLDPMAUType1000BasePX10D   uint16 = 50
	LLDPMAUType1000BasePX10U   uint16 = 51
	LLDPMAUType1000BasePX20D   uint16 = 52
	LLDPMAUType1000BasePX20U   uint16 = 53
	LLDPMAUType10GBaseT        uint16 = 54
	LLDPMAUType10GBaseLRM      uint16 = 55
	LLDPMAUType1000BaseKX      uint16 = 56
	LLDPMAUType10GBaseKX4      uint16 = 57
	LLDPMAUType10GBaseKR       uint16 = 58
	LLDPMAUType10_1GBasePRX_D1 uint16 = 59
	LLDPMAUType10_1GBasePRX_D2 uint16 = 60
	LLDPMAUType10_1GBasePRX_D3 uint16 = 61
	LLDPMAUType10_1GBasePRX_U1 uint16 = 62
	LLDPMAUType10_1GBasePRX_U2 uint16 = 63
	LLDPMAUType10_1GBasePRX_U3 uint16 = 64
	LLDPMAUType10GBasePR_D1    uint16 = 65
	LLDPMAUType10GBasePR_D2    uint16 = 66
	LLDPMAUType10GBasePR_D3    uint16 = 67
	LLDPMAUType10GBasePR_U1    uint16 = 68
	LLDPMAUType10GBasePR_U3    uint16 = 69
)

// From RFC 3636 - ifMauAutoNegCapAdvertisedBits
const (
	LLDPMAUPMDOther        uint16 = 1 << 15
	LLDPMAUPMD10BaseT      uint16 = 1 << 14
	LLDPMAUPMD10BaseT_FD   uint16 = 1 << 13
	LLDPMAUPMD100BaseT4    uint16 = 1 << 12
	LLDPMAUPMD100BaseTX    uint16 = 1 << 11
	LLDPMAUPMD100BaseTX_FD uint16 = 1 << 10
	LLDPMAUPMD100BaseT2    uint16 = 1 << 9
	LLDPMAUPMD100BaseT2_FD uint16 = 1 << 8
	LLDPMAUPMDFDXPAUSE     uint16 = 1 << 7
	LLDPMAUPMDFDXAPAUSE    uint16 = 1 << 6
	LLDPMAUPMDFDXSPAUSE    uint16 = 1 << 5
	LLDPMAUPMDFDXBPAUSE    uint16 = 1 << 4
	LLDPMAUPMD1000BaseX    uint16 = 1 << 3
	LLDPMAUPMD1000BaseX_FD uint16 = 1 << 2
	LLDPMAUPMD1000BaseT    uint16 = 1 << 1
	LLDPMAUPMD1000BaseT_FD uint16 = 1 << 0
)

// Inverted ifMauAutoNegCapAdvertisedBits if required
// (Some manufacturers misinterpreted the spec -
// see https://bugs.wireshark.org/bugzilla/show_bug.cgi?id=1455)
const (
	LLDPMAUPMDOtherInv        uint16 = 1 << 0
	LLDPMAUPMD10BaseTInv      uint16 = 1 << 1
	LLDPMAUPMD10BaseT_FDInv   uint16 = 1 << 2
	LLDPMAUPMD100BaseT4Inv    uint16 = 1 << 3
	LLDPMAUPMD100BaseTXInv    uint16 = 1 << 4
	LLDPMAUPMD100BaseTX_FDInv uint16 = 1 << 5
	LLDPMAUPMD100BaseT2Inv    uint16 = 1 << 6
	LLDPMAUPMD100BaseT2_FDInv uint16 = 1 << 7
	LLDPMAUPMDFDXPAUSEInv     uint16 = 1 << 8
	LLDPMAUPMDFDXAPAUSEInv    uint16 = 1 << 9
	LLDPMAUPMDFDXSPAUSEInv    uint16 = 1 << 10
	LLDPMAUPMDFDXBPAUSEInv    uint16 = 1 << 11
	LLDPMAUPMD1000BaseXInv    uint16 = 1 << 12
	LLDPMAUPMD1000BaseX_FDInv uint16 = 1 << 13
	LLDPMAUPMD1000BaseTInv    uint16 = 1 << 14
	LLDPMAUPMD1000BaseT_FDInv uint16 = 1 << 15
)

type LLDPMACPHYConfigStatus struct {
	AutoNegSupported  bool
	AutoNegEnabled    bool
	AutoNegCapability uint16
	MAUType           uint16
}

// MDI Power options
const (
	LLDPMDIPowerPortClass    byte = 1 << 0
	LLDPMDIPowerCapability   byte = 1 << 1
	LLDPMDIPowerStatus       byte = 1 << 2
	LLDPMDIPowerPairsAbility byte = 1 << 3
)

type LLDPPowerType byte

type LLDPPowerSource byte

type LLDPPowerPriority byte

const (
	LLDPPowerPriorityUnknown LLDPPowerPriority = 0
	LLDPPowerPriorityMedium  LLDPPowerPriority = 1
	LLDPPowerPriorityHigh    LLDPPowerPriority = 2
	LLDPPowerPriorityLow     LLDPPowerPriority = 3
)

type LLDPPowerViaMDI8023 struct {
	PortClassPSE    bool // false = PD
	PSESupported    bool
	PSEEnabled      bool
	PSEPairsAbility bool
	PSEPowerPair    uint8
	PSEClass        uint8
	Type            LLDPPowerType
	Source          LLDPPowerSource
	Priority        LLDPPowerPriority
	Requested       uint16 // 1-510 Watts
	Allocated       uint16 // 1-510 Watts
}

// LLDPInfo8023 represents the information carried in 802.3 Org-specific TLVs
type LLDPInfo8023 struct {
	MACPHYConfigStatus LLDPMACPHYConfigStatus
	PowerViaMDI        LLDPPowerViaMDI8023
	LinkAggregation    LLDPLinkAggregation
	MTU                uint16
}

// IEEE 802.1Qbg TLV Subtypes
const (
	LLDP8021QbgEVB   uint8 = 0
	LLDP8021QbgCDCP  uint8 = 1
	LLDP8021QbgVDP   uint8 = 2
	LLDP8021QbgEVB22 uint8 = 13
)

// LLDPEVBCapabilities Types
const (
	LLDPEVBCapsSTD uint16 = 1 << 7
	LLDPEVBCapsRR  uint16 = 1 << 6
	LLDPEVBCapsRTE uint16 = 1 << 2
	LLDPEVBCapsECP uint16 = 1 << 1
	LLDPEVBCapsVDP uint16 = 1 << 0
)

// LLDPEVBCapabilities represents the EVB capabilities of a device
type LLDPEVBCapabilities struct {
	StandardBridging            bool
	ReflectiveRelay             bool
	RetransmissionTimerExponent bool
	EdgeControlProtocol         bool
	VSIDiscoveryProtocol        bool
}

type LLDPEVBSettings struct {
	Supported      LLDPEVBCapabilities
	Enabled        LLDPEVBCapabilities
	SupportedVSIs  uint16
	ConfiguredVSIs uint16
	RTEExponent    uint8
}

// LLDPInfo8021Qbg represents the information carried in 802.1Qbg Org-specific TLVs
type LLDPInfo8021Qbg struct {
	EVBSettings LLDPEVBSettings
}

type LLDPMediaSubtype uint8

// Media TLV Subtypes
const (
	LLDPMediaTypeCapabilities LLDPMediaSubtype = 1
	LLDPMediaTypeNetwork      LLDPMediaSubtype = 2
	LLDPMediaTypeLocation     LLDPMediaSubtype = 3
	LLDPMediaTypePower        LLDPMediaSubtype = 4
	LLDPMediaTypeHardware     LLDPMediaSubtype = 5
	LLDPMediaTypeFirmware     LLDPMediaSubtype = 6
	LLDPMediaTypeSoftware     LLDPMediaSubtype = 7
	LLDPMediaTypeSerial       LLDPMediaSubtype = 8
	LLDPMediaTypeManufacturer LLDPMediaSubtype = 9
	LLDPMediaTypeModel        LLDPMediaSubtype = 10
	LLDPMediaTypeAssetID      LLDPMediaSubtype = 11
)

type LLDPMediaClass uint8

// Media Class Values
const (
	LLDPMediaClassUndefined   LLDPMediaClass = 0
	LLDPMediaClassEndpointI   LLDPMediaClass = 1
	LLDPMediaClassEndpointII  LLDPMediaClass = 2
	LLDPMediaClassEndpointIII LLDPMediaClass = 3
	LLDPMediaClassNetwork     LLDPMediaClass = 4
)

// LLDPMediaCapabilities Types
const (
	LLDPMediaCapsLLDP      uint16 = 1 << 0
	LLDPMediaCapsNetwork   uint16 = 1 << 1
	LLDPMediaCapsLocation  uint16 = 1 << 2
	LLDPMediaCapsPowerPSE  uint16 = 1 << 3
	LLDPMediaCapsPowerPD   uint16 = 1 << 4
	LLDPMediaCapsInventory uint16 = 1 << 5
)

// LLDPMediaCapabilities represents the LLDP Media capabilities of a device
type LLDPMediaCapabilities struct {
	Capabilities  bool
	NetworkPolicy bool
	Location      bool
	PowerPSE      bool
	PowerPD       bool
	Inventory     bool
	Class         LLDPMediaClass
}

type LLDPApplicationType uint8

const (
	LLDPAppTypeReserved            LLDPApplicationType = 0
	LLDPAppTypeVoice               LLDPApplicationType = 1
	LLDPappTypeVoiceSignaling      LLDPApplicationType = 2
	LLDPappTypeGuestVoice          LLDPApplicationType = 3
	LLDPappTypeGuestVoiceSignaling LLDPApplicationType = 4
	LLDPappTypeSoftphoneVoice      LLDPApplicationType = 5
	LLDPappTypeVideoConferencing   LLDPApplicationType = 6
	LLDPappTypeStreamingVideo      LLDPApplicationType = 7
	LLDPappTypeVideoSignaling      LLDPApplicationType = 8
)

type LLDPNetworkPolicy struct {
	ApplicationType LLDPApplicationType
	Defined         bool
	Tagged          bool
	VLANId          uint16
	L2Priority      uint16
	DSCPValue       uint8
}

type LLDPLocationFormat uint8

const (
	LLDPLocationFormatInvalid    LLDPLocationFormat = 0
	LLDPLocationFormatCoordinate LLDPLocationFormat = 1
	LLDPLocationFormatAddress    LLDPLocationFormat = 2
	LLDPLocationFormatECS        LLDPLocationFormat = 3
)

type LLDPLocationAddressWhat uint8

const (
	LLDPLocationAddressWhatDHCP    LLDPLocationAddressWhat = 0
	LLDPLocationAddressWhatNetwork LLDPLocationAddressWhat = 1
	LLDPLocationAddressWhatClient  LLDPLocationAddressWhat = 2
)

type LLDPLocationAddressType uint8

const (
	LLDPLocationAddressTypeLanguage       LLDPLocationAddressType = 0
	LLDPLocationAddressTypeNational       LLDPLocationAddressType = 1
	LLDPLocationAddressTypeCounty         LLDPLocationAddressType = 2
	LLDPLocationAddressTypeCity           LLDPLocationAddressType = 3
	LLDPLocationAddressTypeCityDivision   LLDPLocationAddressType = 4
	LLDPLocationAddressTypeNeighborhood   LLDPLocationAddressType = 5
	LLDPLocationAddressTypeStreet         LLDPLocationAddressType = 6
	LLDPLocationAddressTypeLeadingStreet  LLDPLocationAddressType = 16
	LLDPLocationAddressTypeTrailingStreet LLDPLocationAddressType = 17
	LLDPLocationAddressTypeStreetSuffix   LLDPLocationAddressType = 18
	LLDPLocationAddressTypeHouseNum       LLDPLocationAddressType = 19
	LLDPLocationAddressTypeHouseSuffix    LLDPLocationAddressType = 20
	LLDPLocationAddressTypeLandmark       LLDPLocationAddressType = 21
	LLDPLocationAddressTypeAdditional     LLDPLocationAddressType = 22
	LLDPLocationAddressTypeName           LLDPLocationAddressType = 23
	LLDPLocationAddressTypePostal         LLDPLocationAddressType = 24
	LLDPLocationAddressTypeBuilding       LLDPLocationAddressType = 25
	LLDPLocationAddressTypeUnit           LLDPLocationAddressType = 26
	LLDPLocationAddressTypeFloor          LLDPLocationAddressType = 27
	LLDPLocationAddressTypeRoom           LLDPLocationAddressType = 28
	LLDPLocationAddressTypePlace          LLDPLocationAddressType = 29
	LLDPLocationAddressTypeScript         LLDPLocationAddressType = 128
)

type LLDPLocationCoordinate struct {
	LatitudeResolution  uint8
	Latitude            uint64
	LongitudeResolution uint8
	Longitude           uint64
	AltitudeType        uint8
	AltitudeResolution  uint16
	Altitude            uint32
	Datum               uint8
}

type LLDPLocationAddressLine struct {
	Type  LLDPLocationAddressType
	Value string
}

type LLDPLocationAddress struct {
	What         LLDPLocationAddressWhat
	CountryCode  string
	AddressLines []LLDPLocationAddressLine
}

type LLDPLocationECS struct {
	ELIN string
}

// LLDP represents a physical location.
// Only one of the embedded types will contain values, depending on Format.
type LLDPLocation struct {
	Format     LLDPLocationFormat
	Coordinate LLDPLocationCoordinate
	Address    LLDPLocationAddress
	ECS        LLDPLocationECS
}

type LLDPPowerViaMDI struct {
	Type     LLDPPowerType
	Source   LLDPPowerSource
	Priority LLDPPowerPriority
	Value    uint16
}

// LLDPInfoMedia represents the information carried in TR-41 Org-specific TLVs
type LLDPInfoMedia struct {
	MediaCapabilities LLDPMediaCapabilities
	NetworkPolicy     LLDPNetworkPolicy
	Location          LLDPLocation
	PowerViaMDI       LLDPPowerViaMDI
	HardwareRevision  string
	FirmwareRevision  string
	SoftwareRevision  string
	SerialNumber      string
	Manufacturer      string
	Model             string
	AssetID           string
}

type LLDPCisco2Subtype uint8

// Cisco2 TLV Subtypes
const (
	LLDPCisco2PowerViaMDI LLDPCisco2Subtype = 1
)

const (
	LLDPCiscoPSESupport   uint8 = 1 << 0
	LLDPCiscoArchShared   uint8 = 1 << 1
	LLDPCiscoPDSparePair  uint8 = 1 << 2
	LLDPCiscoPSESparePair uint8 = 1 << 3
)

// LLDPInfoCisco2 represents the information carried in Cisco Org-specific TLVs
type LLDPInfoCisco2 struct {
	PSEFourWirePoESupported       bool
	PDSparePairArchitectureShared bool
	PDRequestSparePairPoEOn       bool
	PSESparePairPoEOn             bool
}

// Profinet Subtypes
type LLDPProfinetSubtype uint8

const (
	LLDPProfinetPNIODelay         LLDPProfinetSubtype = 1
	LLDPProfinetPNIOPortStatus    LLDPProfinetSubtype = 2
	LLDPProfinetPNIOMRPPortStatus LLDPProfinetSubtype = 4
	LLDPProfinetPNIOChassisMAC    LLDPProfinetSubtype = 5
	LLDPProfinetPNIOPTCPStatus    LLDPProfinetSubtype = 6
)

type LLDPPNIODelay struct {
	RXLocal    uint32
	RXRemote   uint32
	TXLocal    uint32
	TXRemote   uint32
	CableLocal uint32
}

type LLDPPNIOPortStatus struct {
	Class2 uint16
	Class3 uint16
}

type LLDPPNIOMRPPortStatus struct {
	UUID   []byte
	Status uint16
}

type LLDPPNIOPTCPStatus struct {
	MasterAddress     []byte
	SubdomainUUID     []byte
	IRDataUUID        []byte
	PeriodValid       bool
	PeriodLength      uint32
	RedPeriodValid    bool
	RedPeriodBegin    uint32
	OrangePeriodValid bool
	OrangePeriodBegin uint32
	GreenPeriodValid  bool
	GreenPeriodBegin  uint32
}

// LLDPInfoProfinet represents the information carried in Profinet Org-specific TLVs
type LLDPInfoProfinet struct {
	PNIODelay         LLDPPNIODelay
	PNIOPortStatus    LLDPPNIOPortStatus
	PNIOMRPPortStatus LLDPPNIOMRPPortStatus
	ChassisMAC        []byte
	PNIOPTCPStatus    LLDPPNIOPTCPStatus
}

// LayerType returns gopacket.LayerTypeLinkLayerDiscovery.
func (c *LinkLayerDiscovery) LayerType() gopacket.LayerType {
	return LayerTypeLinkLayerDiscovery
}

// SerializeTo serializes LLDP packet to bytes and writes on SerializeBuffer.
func (c *LinkLayerDiscovery) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	chassIDLen := c.ChassisID.serializedLen()
	portIDLen := c.PortID.serializedLen()
	vb, err := b.AppendBytes(chassIDLen + portIDLen + 4) // +4 for TTL
	if err != nil {
		return err
	}
	copy(vb[:chassIDLen], c.ChassisID.serialize())
	copy(vb[chassIDLen:], c.PortID.serialize())
	ttlIDLen := uint16(LLDPTLVTTL)<<9 | uint16(2)
	binary.BigEndian.PutUint16(vb[chassIDLen+portIDLen:], ttlIDLen)
	binary.BigEndian.PutUint16(vb[chassIDLen+portIDLen+2:], c.TTL)

	for _, v := range c.Values {
		vb, err := b.AppendBytes(int(v.Length) + 2) // +2 for TLV type and length; 1 byte for subtype is included in v.Value
		if err != nil {
			return err
		}
		idLen := ((uint16(v.Type) << 9) | v.Length)
		binary.BigEndian.PutUint16(vb[0:2], idLen)
		copy(vb[2:], v.Value)
	}

	vb, err = b.AppendBytes(2) // End Tlv, 2 bytes
	if err != nil {
		return err
	}
	binary.BigEndian.PutUint16(vb[len(vb)-2:], uint16(0)) //End tlv, 2 bytes, all zero
	return nil

}

func decodeLinkLayerDiscovery(data []byte, p gopacket.PacketBuilder) error {
	var vals []LinkLayerDiscoveryValue
	vData := data[0:]
	for len(vData) > 0 {
		if len(vData) < 2 {
			p.SetTruncated()
			return errors.New("LLDP vdata < 2 bytes")
		}
		nbit := vData[0] & 0x01
		t := LLDPTLVType(vData[0] >> 1)
		val := LinkLayerDiscoveryValue{Type: t, Length: uint16(nbit)<<8 + uint16(vData[1])}
		if val.Length > 0 {
			if len(vData) < int(val.Length+2) {
				p.SetTruncated()
				return fmt.Errorf("LLDP VData < %d bytes", val.Length+2)
			}
			val.Value = vData[2 : val.Length+2]
		}
		vals = append(vals, val)
		if t == LLDPTLVEnd {
			break
		}
		if len(vData) < int(2+val.Length) {
			return errors.New("Malformed LinkLayerDiscovery Header")
		}
		vData = vData[2+val.Length:]
	}
	if len(vals) < 4 {
		return errors.New("Missing mandatory LinkLayerDiscovery TLV")
	}
	c := &LinkLayerDiscovery{}
	gotEnd := false
	for _, v := range vals {
		switch v.Type {
		case LLDPTLVEnd:
			gotEnd = true
		case LLDPTLVChassisID:
			if len(v.Value) < 2 {
				return errors.New("Malformed LinkLayerDiscovery ChassisID TLV")
			}
			c.ChassisID.Subtype = LLDPChassisIDSubType(v.Value[0])
			c.ChassisID.ID = v.Value[1:]
		case LLDPTLVPortID:
			if len(v.Value) < 2 {
				return errors.New("Malformed LinkLayerDiscovery PortID TLV")
			}
			c.PortID.Subtype = LLDPPortIDSubType(v.Value[0])
			c.PortID.ID = v.Value[1:]
		case LLDPTLVTTL:
			if len(v.Value) < 2 {
				return errors.New("Malformed LinkLayerDiscovery TTL TLV")
			}
			c.TTL = binary.BigEndian.Uint16(v.Value[0:2])
		default:
			c.Values = append(c.Values, v)
		}
	}
	if c.ChassisID.Subtype == 0 || c.PortID.Subtype == 0 || !gotEnd {
		return errors.New("Missing mandatory LinkLayerDiscovery TLV")
	}
	c.Contents = data
	p.AddLayer(c)

	info := &LinkLayerDiscoveryInfo{}
	p.AddLayer(info)
	for _, v := range c.Values {
		switch v.Type {
		case LLDPTLVPortDescription:
			info.PortDescription = string(v.Value)
		case LLDPTLVSysName:
			info.SysName = string(v.Value)
		case LLDPTLVSysDescription:
			info.SysDescription = string(v.Value)
		case LLDPTLVSysCapabilities:
			if err := checkLLDPTLVLen(v, 4); err != nil {
				return err
			}
			info.SysCapabilities.SystemCap = getCapabilities(binary.BigEndian.Uint16(v.Value[0:2]))
			info.SysCapabilities.EnabledCap = getCapabilities(binary.BigEndian.Uint16(v.Value[2:4]))
		case LLDPTLVMgmtAddress:
			if err := checkLLDPTLVLen(v, 9); err != nil {
				return err
			}
			mlen := v.Value[0]
			if err := checkLLDPTLVLen(v, int(mlen+7)); err != nil {
				return err
			}
			info.MgmtAddress.Subtype = IANAAddressFamily(v.Value[1])
			info.MgmtAddress.Address = v.Value[2 : mlen+1]
			info.MgmtAddress.InterfaceSubtype = LLDPInterfaceSubtype(v.Value[mlen+1])
			info.MgmtAddress.InterfaceNumber = binary.BigEndian.Uint32(v.Value[mlen+2 : mlen+6])
			olen := v.Value[mlen+6]
			if err := checkLLDPTLVLen(v, int(mlen+7+olen)); err != nil {
				return err
			}
			info.MgmtAddress.OID = string(v.Value[mlen+7 : mlen+7+olen])
		case LLDPTLVOrgSpecific:
			if err := checkLLDPTLVLen(v, 4); err != nil {
				return err
			}
			info.OrgTLVs = append(info.OrgTLVs, LLDPOrgSpecificTLV{IEEEOUI(binary.BigEndian.Uint32(append([]byte{byte(0)}, v.Value[0:3]...))), uint8(v.Value[3]), v.Value[4:]})
		}
	}
	return nil
}

func (l *LinkLayerDiscoveryInfo) Decode8021() (info LLDPInfo8021, err error) {
	for _, o := range l.OrgTLVs {
		if o.OUI != IEEEOUI8021 {
			continue
		}
		switch o.SubType {
		case LLDP8021SubtypePortVLANID:
			if err = checkLLDPOrgSpecificLen(o, 2); err != nil {
				return
			}
			info.PVID = binary.BigEndian.Uint16(o.Info[0:2])
		case LLDP8021SubtypeProtocolVLANID:
			if err = checkLLDPOrgSpecificLen(o, 3); err != nil {
				return
			}
			sup := (o.Info[0]&LLDPProtocolVLANIDCapability > 0)
			en := (o.Info[0]&LLDPProtocolVLANIDStatus > 0)
			id := binary.BigEndian.Uint16(o.Info[1:3])
			info.PPVIDs = append(info.PPVIDs, PortProtocolVLANID{sup, en, id})
		case LLDP8021SubtypeVLANName:
			if err = checkLLDPOrgSpecificLen(o, 2); err != nil {
				return
			}
			id := binary.BigEndian.Uint16(o.Info[0:2])
			info.VLANNames = append(info.VLANNames, VLANName{id, string(o.Info[3:])})
		case LLDP8021SubtypeProtocolIdentity:
			if err = checkLLDPOrgSpecificLen(o, 1); err != nil {
				return
			}
			l := int(o.Info[0])
			if l > 0 {
				info.ProtocolIdentities = append(info.ProtocolIdentities, o.Info[1:1+l])
			}
		case LLDP8021SubtypeVDIUsageDigest:
			if err = checkLLDPOrgSpecificLen(o, 4); err != nil {
				return
			}
			info.VIDUsageDigest = binary.BigEndian.Uint32(o.Info[0:4])
		case LLDP8021SubtypeManagementVID:
			if err = checkLLDPOrgSpecificLen(o, 2); err != nil {
				return
			}
			info.ManagementVID = binary.BigEndian.Uint16(o.Info[0:2])
		case LLDP8021SubtypeLinkAggregation:
			if err = checkLLDPOrgSpecificLen(o, 5); err != nil {
				return
			}
			sup := (o.Info[0]&LLDPAggregationCapability > 0)
			en := (o.Info[0]&LLDPAggregationStatus > 0)
			info.LinkAggregation = LLDPLinkAggregation{sup, en, binary.BigEndian.Uint32(o.Info[1:5])}
		}
	}
	return
}

func (l *LinkLayerDiscoveryInfo) Decode8023() (info LLDPInfo8023, err error) {
	for _, o := range l.OrgTLVs {
		if o.OUI != IEEEOUI8023 {
			continue
		}
		switch o.SubType {
		case LLDP8023SubtypeMACPHY:
			if err = checkLLDPOrgSpecificLen(o, 5); err != nil {
				return
			}
			sup := (o.Info[0]&LLDPMACPHYCapability > 0)
			en := (o.Info[0]&LLDPMACPHYStatus > 0)
			ca := binary.BigEndian.Uint16(o.Info[1:3])
			mau := binary.BigEndian.Uint16(o.Info[3:5])
			info.MACPHYConfigStatus = LLDPMACPHYConfigStatus{sup, en, ca, mau}
		case LLDP8023SubtypeMDIPower:
			if err = checkLLDPOrgSpecificLen(o, 3); err != nil {
				return
			}
			info.PowerViaMDI.PortClassPSE = (o.Info[0]&LLDPMDIPowerPortClass > 0)
			info.PowerViaMDI.PSESupported = (o.Info[0]&LLDPMDIPowerCapability > 0)
			info.PowerViaMDI.PSEEnabled = (o.Info[0]&LLDPMDIPowerStatus > 0)
			info.PowerViaMDI.PSEPairsAbility = (o.Info[0]&LLDPMDIPowerPairsAbility > 0)
			info.PowerViaMDI.PSEPowerPair = uint8(o.Info[1])
			info.PowerViaMDI.PSEClass = uint8(o.Info[2])
			if len(o.Info) >= 7 {
				info.PowerViaMDI.Type = LLDPPowerType((o.Info[3] & 0xc0) >> 6)
				info.PowerViaMDI.Source = LLDPPowerSource((o.Info[3] & 0x30) >> 4)
				if info.PowerViaMDI.Type == 1 || info.PowerViaMDI.Type == 3 {
					info.PowerViaMDI.Source += 128 // For Stringify purposes
				}
				info.PowerViaMDI.Priority = LLDPPowerPriority(o.Info[3] & 0x0f)
				info.PowerViaMDI.Requested = binary.BigEndian.Uint16(o.Info[4:6])
				info.PowerViaMDI.Allocated = binary.BigEndian.Uint16(o.Info[6:8])
			}
		case LLDP8023SubtypeLinkAggregation:
			if err = checkLLDPOrgSpecificLen(o, 5); err != nil {
				return
			}
			sup := (o.Info[0]&LLDPAggregationCapability > 0)
			en := (o.Info[0]&LLDPAggregationStatus > 0)
			info.LinkAggregation = LLDPLinkAggregation{sup, en, binary.BigEndian.Uint32(o.Info[1:5])}
		case LLDP8023SubtypeMTU:
			if err = checkLLDPOrgSpecificLen(o, 2); err != nil {
				return
			}
			info.MTU = binary.BigEndian.Uint16(o.Info[0:2])
		}
	}
	return
}

func (l *LinkLayerDiscoveryInfo) Decode8021Qbg() (info LLDPInfo8021Qbg, err error) {
	for _, o := range l.OrgTLVs {
		if o.OUI != IEEEOUI8021Qbg {
			continue
		}
		switch o.SubType {
		case LLDP8021QbgEVB:
			if err = checkLLDPOrgSpecificLen(o, 9); err != nil {
				return
			}
			info.EVBSettings.Supported = getEVBCapabilities(binary.BigEndian.Uint16(o.Info[0:2]))
			info.EVBSettings.Enabled = getEVBCapabilities(binary.BigEndian.Uint16(o.Info[2:4]))
			info.EVBSettings.SupportedVSIs = binary.BigEndian.Uint16(o.Info[4:6])
			info.EVBSettings.ConfiguredVSIs = binary.BigEndian.Uint16(o.Info[6:8])
			info.EVBSettings.RTEExponent = uint8(o.Info[8])
		}
	}
	return
}

func (l *LinkLayerDiscoveryInfo) DecodeMedia() (info LLDPInfoMedia, err error) {
	for _, o := range l.OrgTLVs {
		if o.OUI != IEEEOUIMedia {
			continue
		}
		switch LLDPMediaSubtype(o.SubType) {
		case LLDPMediaTypeCapabilities:
			if err = checkLLDPOrgSpecificLen(o, 3); err != nil {
				return
			}
			b := binary.BigEndian.Uint16(o.Info[0:2])
			info.MediaCapabilities.Capabilities = (b & LLDPMediaCapsLLDP) > 0
			info.MediaCapabilities.NetworkPolicy = (b & LLDPMediaCapsNetwork) > 0
			info.MediaCapabilities.Location = (b & LLDPMediaCapsLocation) > 0
			info.MediaCapabilities.PowerPSE = (b & LLDPMediaCapsPowerPSE) > 0
			info.MediaCapabilities.PowerPD = (b & LLDPMediaCapsPowerPD) > 0
			info.MediaCapabilities.Inventory = (b & LLDPMediaCapsInventory) > 0
			info.MediaCapabilities.Class = LLDPMediaClass(o.Info[2])
		case LLDPMediaTypeNetwork:
			if err = checkLLDPOrgSpecificLen(o, 4); err != nil {
				return
			}
			info.NetworkPolicy.ApplicationType = LLDPApplicationType(o.Info[0])
			b := binary.BigEndian.Uint16(o.Info[1:3])
			info.NetworkPolicy.Defined = (b & 0x8000) == 0
			info.NetworkPolicy.Tagged = (b & 0x4000) > 0
			info.NetworkPolicy.VLANId = (b & 0x1ffe) >> 1
			b = binary.BigEndian.Uint16(o.Info[2:4])
			info.NetworkPolicy.L2Priority = (b & 0x01c0) >> 6
			info.NetworkPolicy.DSCPValue = uint8(o.Info[3] & 0x3f)
		case LLDPMediaTypeLocation:
			if err = checkLLDPOrgSpecificLen(o, 1); err != nil {
				return
			}
			info.Location.Format = LLDPLocationFormat(o.Info[0])
			o.Info = o.Info[1:]
			switch info.Location.Format {
			case LLDPLocationFormatCoordinate:
				if err = checkLLDPOrgSpecificLen(o, 16); err != nil {
					return
				}
				info.Location.Coordinate.LatitudeResolution = uint8(o.Info[0]&0xfc) >> 2
				b := binary.BigEndian.Uint64(o.Info[0:8])
				info.Location.Coordinate.Latitude = (b & 0x03ffffffff000000) >> 24
				info.Location.Coordinate.LongitudeResolution = uint8(o.Info[5]&0xfc) >> 2
				b = binary.BigEndian.Uint64(o.Info[5:13])
				info.Location.Coordinate.Longitude = (b & 0x03ffffffff000000) >> 24
				info.Location.Coordinate.AltitudeType = uint8((o.Info[10] & 0x30) >> 4)
				b1 := binary.BigEndian.Uint16(o.Info[10:12])
				info.Location.Coordinate.AltitudeResolution = (b1 & 0xfc0) >> 6
				b2 := binary.BigEndian.Uint32(o.Info[11:15])
				info.Location.Coordinate.Altitude = b2 & 0x3fffffff
				info.Location.Coordinate.Datum = uint8(o.Info[15])
			case LLDPLocationFormatAddress:
				if err = checkLLDPOrgSpecificLen(o, 3); err != nil {
					return
				}
				//ll := uint8(o.Info[0])
				info.Location.Address.What = LLDPLocationAddressWhat(o.Info[1])
				info.Location.Address.CountryCode = string(o.Info[2:4])
				data := o.Info[4:]
				for len(data) > 1 {
					aType := LLDPLocationAddressType(data[0])
					aLen := int(data[1])
					if len(data) >= aLen+2 {
						info.Location.Address.AddressLines = append(info.Location.Address.AddressLines, LLDPLocationAddressLine{aType, string(data[2 : aLen+2])})
						data = data[aLen+2:]
					} else {
						break
					}
				}
			case LLDPLocationFormatECS:
				info.Location.ECS.ELIN = string(o.Info)
			}
		case LLDPMediaTypePower:
			if err = checkLLDPOrgSpecificLen(o, 3); err != nil {
				return
			}
			info.PowerViaMDI.Type = LLDPPowerType((o.Info[0] & 0xc0) >> 6)
			info.PowerViaMDI.Source = LLDPPowerSource((o.Info[0] & 0x30) >> 4)
			if info.PowerViaMDI.Type == 1 || info.PowerViaMDI.Type == 3 {
				info.PowerViaMDI.Source += 128 // For Stringify purposes
			}
			info.PowerViaMDI.Priority = LLDPPowerPriority(o.Info[0] & 0x0f)
			info.PowerViaMDI.Value = binary.BigEndian.Uint16(o.Info[1:3]) * 100 // 0 to 102.3 w, 0.1W increments
		case LLDPMediaTypeHardware:
			info.HardwareRevision = string(o.Info)
		case LLDPMediaTypeFirmware:
			info.FirmwareRevision = string(o.Info)
		case LLDPMediaTypeSoftware:
			info.SoftwareRevision = string(o.Info)
		case LLDPMediaTypeSerial:
			info.SerialNumber = string(o.Info)
		case LLDPMediaTypeManufacturer:
			info.Manufacturer = string(o.Info)
		case LLDPMediaTypeModel:
			info.Model = string(o.Info)
		case LLDPMediaTypeAssetID:
			info.AssetID = string(o.Info)
		}
	}
	return
}

func (l *LinkLayerDiscoveryInfo) DecodeCisco2() (info LLDPInfoCisco2, err error) {
	for _, o := range l.OrgTLVs {
		if o.OUI != IEEEOUICisco2 {
			continue
		}
		switch LLDPCisco2Subtype(o.SubType) {
		case LLDPCisco2PowerViaMDI:
			if err = checkLLDPOrgSpecificLen(o, 1); err != nil {
				return
			}
			info.PSEFourWirePoESupported = (o.Info[0] & LLDPCiscoPSESupport) > 0
			info.PDSparePairArchitectureShared = (o.Info[0] & LLDPCiscoArchShared) > 0
			info.PDRequestSparePairPoEOn = (o.Info[0] & LLDPCiscoPDSparePair) > 0
			info.PSESparePairPoEOn = (o.Info[0] & LLDPCiscoPSESparePair) > 0
		}
	}
	return
}

func (l *LinkLayerDiscoveryInfo) DecodeProfinet() (info LLDPInfoProfinet, err error) {
	for _, o := range l.OrgTLVs {
		if o.OUI != IEEEOUIProfinet {
			continue
		}
		switch LLDPProfinetSubtype(o.SubType) {
		case LLDPProfinetPNIODelay:
			if err = checkLLDPOrgSpecificLen(o, 20); err != nil {
				return
			}
			info.PNIODelay.RXLocal = binary.BigEndian.Uint32(o.Info[0:4])
			info.PNIODelay.RXRemote = binary.BigEndian.Uint32(o.Info[4:8])
			info.PNIODelay.TXLocal = binary.BigEndian.Uint32(o.Info[8:12])
			info.PNIODelay.TXRemote = binary.BigEndian.Uint32(o.Info[12:16])
			info.PNIODelay.CableLocal = binary.BigEndian.Uint32(o.Info[16:20])
		case LLDPProfinetPNIOPortStatus:
			if err = checkLLDPOrgSpecificLen(o, 4); err != nil {
				return
			}
			info.PNIOPortStatus.Class2 = binary.BigEndian.Uint16(o.Info[0:2])
			info.PNIOPortStatus.Class3 = binary.BigEndian.Uint16(o.Info[2:4])
		case LLDPProfinetPNIOMRPPortStatus:
			if err = checkLLDPOrgSpecificLen(o, 18); err != nil {
				return
			}
			info.PNIOMRPPortStatus.UUID = o.Info[0:16]
			info.PNIOMRPPortStatus.Status = binary.BigEndian.Uint16(o.Info[16:18])
		case LLDPProfinetPNIOChassisMAC:
			if err = checkLLDPOrgSpecificLen(o, 6); err != nil {
				return
			}
			info.ChassisMAC = o.Info[0:6]
		case LLDPProfinetPNIOPTCPStatus:
			if err = checkLLDPOrgSpecificLen(o, 54); err != nil {
				return
			}
			info.PNIOPTCPStatus.MasterAddress = o.Info[0:6]
			info.PNIOPTCPStatus.SubdomainUUID = o.Info[6:22]
			info.PNIOPTCPStatus.IRDataUUID = o.Info[22:38]
			b := binary.BigEndian.Uint32(o.Info[38:42])
			info.PNIOPTCPStatus.PeriodValid = (b & 0x80000000) > 0
			info.PNIOPTCPStatus.PeriodLength = b & 0x7fffffff
			b = binary.BigEndian.Uint32(o.Info[42:46])
			info.PNIOPTCPStatus.RedPeriodValid = (b & 0x80000000) > 0
			info.PNIOPTCPStatus.RedPeriodBegin = b & 0x7fffffff
			b = binary.BigEndian.Uint32(o.Info[46:50])
			info.PNIOPTCPStatus.OrangePeriodValid = (b & 0x80000000) > 0
			info.PNIOPTCPStatus.OrangePeriodBegin = b & 0x7fffffff
			b = binary.BigEndian.Uint32(o.Info[50:54])
			info.PNIOPTCPStatus.GreenPeriodValid = (b & 0x80000000) > 0
			info.PNIOPTCPStatus.GreenPeriodBegin = b & 0x7fffffff
		}
	}
	return
}

// LayerType returns gopacket.LayerTypeLinkLayerDiscoveryInfo.
func (c *LinkLayerDiscoveryInfo) LayerType() gopacket.LayerType {
	return LayerTypeLinkLayerDiscoveryInfo
}

func getCapabilities(v uint16) (c LLDPCapabilities) {
	c.Other = (v&LLDPCapsOther > 0)
	c.Repeater = (v&LLDPCapsRepeater > 0)
	c.Bridge = (v&LLDPCapsBridge > 0)
	c.WLANAP = (v&LLDPCapsWLANAP > 0)
	c.Router = (v&LLDPCapsRouter > 0)
	c.Phone = (v&LLDPCapsPhone > 0)
	c.DocSis = (v&LLDPCapsDocSis > 0)
	c.StationOnly = (v&LLDPCapsStationOnly > 0)
	c.CVLAN = (v&LLDPCapsCVLAN > 0)
	c.SVLAN = (v&LLDPCapsSVLAN > 0)
	c.TMPR = (v&LLDPCapsTmpr > 0)
	return
}

func getEVBCapabilities(v uint16) (c LLDPEVBCapabilities) {
	c.StandardBridging = (v & LLDPEVBCapsSTD) > 0
	c.StandardBridging = (v & LLDPEVBCapsSTD) > 0
	c.ReflectiveRelay = (v & LLDPEVBCapsRR) > 0
	c.RetransmissionTimerExponent = (v & LLDPEVBCapsRTE) > 0
	c.EdgeControlProtocol = (v & LLDPEVBCapsECP) > 0
	c.VSIDiscoveryProtocol = (v & LLDPEVBCapsVDP) > 0
	return
}

func (t LLDPTLVType) String() (s string) {
	switch t {
	case LLDPTLVEnd:
		s = "TLV End"
	case LLDPTLVChassisID:
		s = "Chassis ID"
	case LLDPTLVPortID:
		s = "Port ID"
	case LLDPTLVTTL:
		s = "TTL"
	case LLDPTLVPortDescription:
		s = "Port Description"
	case LLDPTLVSysName:
		s = "System Name"
	case LLDPTLVSysDescription:
		s = "System Description"
	case LLDPTLVSysCapabilities:
		s = "System Capabilities"
	case LLDPTLVMgmtAddress:
		s = "Management Address"
	case LLDPTLVOrgSpecific:
		s = "Organisation Specific"
	default:
		s = "Unknown"
	}
	return
}

func (t LLDPChassisIDSubType) String() (s string) {
	switch t {
	case LLDPChassisIDSubTypeReserved:
		s = "Reserved"
	case LLDPChassisIDSubTypeChassisComp:
		s = "Chassis Component"
	case LLDPChassisIDSubtypeIfaceAlias:
		s = "Interface Alias"
	case LLDPChassisIDSubTypePortComp:
		s = "Port Component"
	case LLDPChassisIDSubTypeMACAddr:
		s = "MAC Address"
	case LLDPChassisIDSubTypeNetworkAddr:
		s = "Network Address"
	case LLDPChassisIDSubtypeIfaceName:
		s = "Interface Name"
	case LLDPChassisIDSubTypeLocal:
		s = "Local"
	default:
		s = "Unknown"
	}
	return
}

func (t LLDPPortIDSubType) String() (s string) {
	switch t {
	case LLDPPortIDSubtypeReserved:
		s = "Reserved"
	case LLDPPortIDSubtypeIfaceAlias:
		s = "Interface Alias"
	case LLDPPortIDSubtypePortComp:
		s = "Port Component"
	case LLDPPortIDSubtypeMACAddr:
		s = "MAC Address"
	case LLDPPortIDSubtypeNetworkAddr:
		s = "Network Address"
	case LLDPPortIDSubtypeIfaceName:
		s = "Interface Name"
	case LLDPPortIDSubtypeAgentCircuitID:
		s = "Agent Circuit ID"
	case LLDPPortIDSubtypeLocal:
		s = "Local"
	default:
		s = "Unknown"
	}
	return
}

func (t IANAAddressFamily) String() (s string) {
	switch t {
	case IANAAddressFamilyReserved:
		s = "Reserved"
	case IANAAddressFamilyIPV4:
		s = "IPv4"
	case IANAAddressFamilyIPV6:
		s = "IPv6"
	case IANAAddressFamilyNSAP:
		s = "NSAP"
	case IANAAddressFamilyHDLC:
		s = "HDLC"
	case IANAAddressFamilyBBN1822:
		s = "BBN 1822"
	case IANAAddressFamily802:
		s = "802 media plus Ethernet 'canonical format'"
	case IANAAddressFamilyE163:
		s = "E.163"
	case IANAAddressFamilyE164:
		s = "E.164 (SMDS, Frame Relay, ATM)"
	case IANAAddressFamilyF69:
		s = "F.69 (Telex)"
	case IANAAddressFamilyX121:
		s = "X.121, X.25, Frame Relay"
	case IANAAddressFamilyIPX:
		s = "IPX"
	case IANAAddressFamilyAtalk:
		s = "Appletalk"
	case IANAAddressFamilyDecnet:
		s = "Decnet IV"
	case IANAAddressFamilyBanyan:
		s = "Banyan Vines"
	case IANAAddressFamilyE164NSAP:
		s = "E.164 with NSAP format subaddress"
	case IANAAddressFamilyDNS:
		s = "DNS"
	case IANAAddressFamilyDistname:
		s = "Distinguished Name"
	case IANAAddressFamilyASNumber:
		s = "AS Number"
	case IANAAddressFamilyXTPIPV4:
		s = "XTP over IP version 4"
	case IANAAddressFamilyXTPIPV6:
		s = "XTP over IP version 6"
	case IANAAddressFamilyXTP:
		s = "XTP native mode XTP"
	case IANAAddressFamilyFcWWPN:
		s = "Fibre Channel World-Wide Port Name"
	case IANAAddressFamilyFcWWNN:
		s = "Fibre Channel World-Wide Node Name"
	case IANAAddressFamilyGWID:
		s = "GWID"
	case IANAAddressFamilyL2VPN:
		s = "AFI for Layer 2 VPN"
	default:
		s = "Unknown"
	}
	return
}

func (t LLDPInterfaceSubtype) String() (s string) {
	switch t {
	case LLDPInterfaceSubtypeUnknown:
		s = "Unknown"
	case LLDPInterfaceSubtypeifIndex:
		s = "IfIndex"
	case LLDPInterfaceSubtypeSysPort:
		s = "System Port Number"
	default:
		s = "Unknown"
	}
	return
}

func (t LLDPPowerType) String() (s string) {
	switch t {
	case 0:
		s = "Type 2 PSE Device"
	case 1:
		s = "Type 2 PD Device"
	case 2:
		s = "Type 1 PSE Device"
	case 3:
		s = "Type 1 PD Device"
	default:
		s = "Unknown"
	}
	return
}

func (t LLDPPowerSource) String() (s string) {
	switch t {
	// PD Device
	case 0:
		s = "Unknown"
	case 1:
		s = "PSE"
	case 2:
		s = "Local"
	case 3:
		s = "PSE and Local"
	// PSE Device  (Actual value  + 128)
	case 128:
		s = "Unknown"
	case 129:
		s = "Primary Power Source"
	case 130:
		s = "Backup Power Source"
	default:
		s = "Unknown"
	}
	return
}

func (t LLDPPowerPriority) String() (s string) {
	switch t {
	case 0:
		s = "Unknown"
	case 1:
		s = "Critical"
	case 2:
		s = "High"
	case 3:
		s = "Low"
	default:
		s = "Unknown"
	}
	return
}

func (t LLDPMediaSubtype) String() (s string) {
	switch t {
	case LLDPMediaTypeCapabilities:
		s = "Media Capabilities "
	case LLDPMediaTypeNetwork:
		s = "Network Policy"
	case LLDPMediaTypeLocation:
		s = "Location Identification"
	case LLDPMediaTypePower:
		s = "Extended Power-via-MDI"
	case LLDPMediaTypeHardware:
		s = "Hardware Revision"
	case LLDPMediaTypeFirmware:
		s = "Firmware Revision"
	case LLDPMediaTypeSoftware:
		s = "Software Revision"
	case LLDPMediaTypeSerial:
		s = "Serial Number"
	case LLDPMediaTypeManufacturer:
		s = "Manufacturer"
	case LLDPMediaTypeModel:
		s = "Model"
	case LLDPMediaTypeAssetID:
		s = "Asset ID"
	default:
		s = "Unknown"
	}
	return
}

func (t LLDPMediaClass) String() (s string) {
	switch t {
	case LLDPMediaClassUndefined:
		s = "Undefined"
	case LLDPMediaClassEndpointI:
		s = "Endpoint Class I"
	case LLDPMediaClassEndpointII:
		s = "Endpoint Class II"
	case LLDPMediaClassEndpointIII:
		s = "Endpoint Class III"
	case LLDPMediaClassNetwork:
		s = "Network connectivity "
	default:
		s = "Unknown"
	}
	return
}

func (t LLDPApplicationType) String() (s string) {
	switch t {
	case LLDPAppTypeReserved:
		s = "Reserved"
	case LLDPAppTypeVoice:
		s = "Voice"
	case LLDPappTypeVoiceSignaling:
		s = "Voice Signaling"
	case LLDPappTypeGuestVoice:
		s = "Guest Voice"
	case LLDPappTypeGuestVoiceSignaling:
		s = "Guest Voice Signaling"
	case LLDPappTypeSoftphoneVoice:
		s = "Softphone Voice"
	case LLDPappTypeVideoConferencing:
		s = "Video Conferencing"
	case LLDPappTypeStreamingVideo:
		s = "Streaming Video"
	case LLDPappTypeVideoSignaling:
		s = "Video Signaling"
	default:
		s = "Unknown"
	}
	return
}

func (t LLDPLocationFormat) String() (s string) {
	switch t {
	case LLDPLocationFormatInvalid:
		s = "Invalid"
	case LLDPLocationFormatCoordinate:
		s = "Coordinate-based LCI"
	case LLDPLocationFormatAddress:
		s = "Address-based LCO"
	case LLDPLocationFormatECS:
		s = "ECS ELIN"
	default:
		s = "Unknown"
	}
	return
}

func (t LLDPLocationAddressType) String() (s string) {
	switch t {
	case LLDPLocationAddressTypeLanguage:
		s = "Language"
	case LLDPLocationAddressTypeNational:
		s = "National subdivisions (province, state, etc)"
	case LLDPLocationAddressTypeCounty:
		s = "County, parish, district"
	case LLDPLocationAddressTypeCity:
		s = "City, township"
	case LLDPLocationAddressTypeCityDivision:
		s = "City division, borough, ward"
	case LLDPLocationAddressTypeNeighborhood:
		s = "Neighborhood, block"
	case LLDPLocationAddressTypeStreet:
		s = "Street"
	case LLDPLocationAddressTypeLeadingStreet:
		s = "Leading street direction"
	case LLDPLocationAddressTypeTrailingStreet:
		s = "Trailing street suffix"
	case LLDPLocationAddressTypeStreetSuffix:
		s = "Street suffix"
	case LLDPLocationAddressTypeHouseNum:
		s = "House number"
	case LLDPLocationAddressTypeHouseSuffix:
		s = "House number suffix"
	case LLDPLocationAddressTypeLandmark:
		s = "Landmark or vanity address"
	case LLDPLocationAddressTypeAdditional:
		s = "Additional location information"
	case LLDPLocationAddressTypeName:
		s = "Name"
	case LLDPLocationAddressTypePostal:
		s = "Postal/ZIP code"
	case LLDPLocationAddressTypeBuilding:
		s = "Building"
	case LLDPLocationAddressTypeUnit:
		s = "Unit"
	case LLDPLocationAddressTypeFloor:
		s = "Floor"
	case LLDPLocationAddressTypeRoom:
		s = "Room number"
	case LLDPLocationAddressTypePlace:
		s = "Place type"
	case LLDPLocationAddressTypeScript:
		s = "Script"
	default:
		s = "Unknown"
	}
	return
}

func checkLLDPTLVLen(v LinkLayerDiscoveryValue, l int) (err error) {
	if len(v.Value) < l {
		err = fmt.Errorf("Invalid TLV %v length %d (wanted mimimum %v", v.Type, len(v.Value), l)
	}
	return
}

func checkLLDPOrgSpecificLen(o LLDPOrgSpecificTLV, l int) (err error) {
	if len(o.Info) < l {
		err = fmt.Errorf("Invalid Org Specific TLV %v length %d (wanted minimum %v)", o.SubType, len(o.Info), l)
	}
	return
}
