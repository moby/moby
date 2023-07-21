// Copyright 2012 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

// Enum types courtesy of...
//   http://search.cpan.org/~mchapman/Net-CDP-0.09/lib/Net/CDP.pm
//   https://code.google.com/p/ladvd/
//   http://anonsvn.wireshark.org/viewvc/releases/wireshark-1.8.6/epan/dissectors/packet-cdp.c

package layers

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"

	"github.com/google/gopacket"
)

// CDPTLVType is the type of each TLV value in a CiscoDiscovery packet.
type CDPTLVType uint16

// CDPTLVType values.
const (
	CDPTLVDevID              CDPTLVType = 0x0001
	CDPTLVAddress            CDPTLVType = 0x0002
	CDPTLVPortID             CDPTLVType = 0x0003
	CDPTLVCapabilities       CDPTLVType = 0x0004
	CDPTLVVersion            CDPTLVType = 0x0005
	CDPTLVPlatform           CDPTLVType = 0x0006
	CDPTLVIPPrefix           CDPTLVType = 0x0007
	CDPTLVHello              CDPTLVType = 0x0008
	CDPTLVVTPDomain          CDPTLVType = 0x0009
	CDPTLVNativeVLAN         CDPTLVType = 0x000a
	CDPTLVFullDuplex         CDPTLVType = 0x000b
	CDPTLVVLANReply          CDPTLVType = 0x000e
	CDPTLVVLANQuery          CDPTLVType = 0x000f
	CDPTLVPower              CDPTLVType = 0x0010
	CDPTLVMTU                CDPTLVType = 0x0011
	CDPTLVExtendedTrust      CDPTLVType = 0x0012
	CDPTLVUntrustedCOS       CDPTLVType = 0x0013
	CDPTLVSysName            CDPTLVType = 0x0014
	CDPTLVSysOID             CDPTLVType = 0x0015
	CDPTLVMgmtAddresses      CDPTLVType = 0x0016
	CDPTLVLocation           CDPTLVType = 0x0017
	CDPTLVExternalPortID     CDPTLVType = 0x0018
	CDPTLVPowerRequested     CDPTLVType = 0x0019
	CDPTLVPowerAvailable     CDPTLVType = 0x001a
	CDPTLVPortUnidirectional CDPTLVType = 0x001b
	CDPTLVEnergyWise         CDPTLVType = 0x001d
	CDPTLVSparePairPOE       CDPTLVType = 0x001f
)

// CiscoDiscoveryValue is a TLV value inside a CiscoDiscovery packet layer.
type CiscoDiscoveryValue struct {
	Type   CDPTLVType
	Length uint16
	Value  []byte
}

// CiscoDiscovery is a packet layer containing the Cisco Discovery Protocol.
// See http://www.cisco.com/univercd/cc/td/doc/product/lan/trsrb/frames.htm#31885
type CiscoDiscovery struct {
	BaseLayer
	Version  byte
	TTL      byte
	Checksum uint16
	Values   []CiscoDiscoveryValue
}

// CDPCapability is the set of capabilities advertised by a CDP device.
type CDPCapability uint32

// CDPCapability values.
const (
	CDPCapMaskRouter     CDPCapability = 0x0001
	CDPCapMaskTBBridge   CDPCapability = 0x0002
	CDPCapMaskSPBridge   CDPCapability = 0x0004
	CDPCapMaskSwitch     CDPCapability = 0x0008
	CDPCapMaskHost       CDPCapability = 0x0010
	CDPCapMaskIGMPFilter CDPCapability = 0x0020
	CDPCapMaskRepeater   CDPCapability = 0x0040
	CDPCapMaskPhone      CDPCapability = 0x0080
	CDPCapMaskRemote     CDPCapability = 0x0100
)

// CDPCapabilities represents the capabilities of a device
type CDPCapabilities struct {
	L3Router        bool
	TBBridge        bool
	SPBridge        bool
	L2Switch        bool
	IsHost          bool
	IGMPFilter      bool
	L1Repeater      bool
	IsPhone         bool
	RemotelyManaged bool
}

// CDP Power-over-Ethernet values.
const (
	CDPPoEFourWire  byte = 0x01
	CDPPoEPDArch    byte = 0x02
	CDPPoEPDRequest byte = 0x04
	CDPPoEPSE       byte = 0x08
)

// CDPSparePairPoE provides information on PoE.
type CDPSparePairPoE struct {
	PSEFourWire  bool // Supported / Not supported
	PDArchShared bool // Shared / Independent
	PDRequestOn  bool // On / Off
	PSEOn        bool // On / Off
}

// CDPVLANDialogue encapsulates a VLAN Query/Reply
type CDPVLANDialogue struct {
	ID   uint8
	VLAN uint16
}

// CDPPowerDialogue encapsulates a Power Query/Reply
type CDPPowerDialogue struct {
	ID     uint16
	MgmtID uint16
	Values []uint32
}

// CDPLocation provides location information for a CDP device.
type CDPLocation struct {
	Type     uint8 // Undocumented
	Location string
}

// CDPHello is a Cisco Hello message (undocumented, hence the "Unknown" fields)
type CDPHello struct {
	OUI              []byte
	ProtocolID       uint16
	ClusterMaster    net.IP
	Unknown1         net.IP
	Version          byte
	SubVersion       byte
	Status           byte
	Unknown2         byte
	ClusterCommander net.HardwareAddr
	SwitchMAC        net.HardwareAddr
	Unknown3         byte
	ManagementVLAN   uint16
}

// CDPEnergyWiseSubtype is used within CDP to define TLV values.
type CDPEnergyWiseSubtype uint32

// CDPEnergyWiseSubtype values.
const (
	CDPEnergyWiseRole    CDPEnergyWiseSubtype = 0x00000007
	CDPEnergyWiseDomain  CDPEnergyWiseSubtype = 0x00000008
	CDPEnergyWiseName    CDPEnergyWiseSubtype = 0x00000009
	CDPEnergyWiseReplyTo CDPEnergyWiseSubtype = 0x00000017
)

// CDPEnergyWise is used by CDP to monitor and control power usage.
type CDPEnergyWise struct {
	EncryptedData  []byte
	Unknown1       uint32
	SequenceNumber uint32
	ModelNumber    string
	Unknown2       uint16
	HardwareID     string
	SerialNum      string
	Unknown3       []byte
	Role           string
	Domain         string
	Name           string
	ReplyUnknown1  []byte
	ReplyPort      []byte
	ReplyAddress   []byte
	ReplyUnknown2  []byte
	ReplyUnknown3  []byte
}

// CiscoDiscoveryInfo represents the decoded details for a set of CiscoDiscoveryValues
type CiscoDiscoveryInfo struct {
	BaseLayer
	CDPHello
	DeviceID         string
	Addresses        []net.IP
	PortID           string
	Capabilities     CDPCapabilities
	Version          string
	Platform         string
	IPPrefixes       []net.IPNet
	VTPDomain        string
	NativeVLAN       uint16
	FullDuplex       bool
	VLANReply        CDPVLANDialogue
	VLANQuery        CDPVLANDialogue
	PowerConsumption uint16
	MTU              uint32
	ExtendedTrust    uint8
	UntrustedCOS     uint8
	SysName          string
	SysOID           string
	MgmtAddresses    []net.IP
	Location         CDPLocation
	PowerRequest     CDPPowerDialogue
	PowerAvailable   CDPPowerDialogue
	SparePairPoe     CDPSparePairPoE
	EnergyWise       CDPEnergyWise
	Unknown          []CiscoDiscoveryValue
}

// LayerType returns gopacket.LayerTypeCiscoDiscovery.
func (c *CiscoDiscovery) LayerType() gopacket.LayerType {
	return LayerTypeCiscoDiscovery
}

func decodeCiscoDiscovery(data []byte, p gopacket.PacketBuilder) error {
	c := &CiscoDiscovery{
		Version:  data[0],
		TTL:      data[1],
		Checksum: binary.BigEndian.Uint16(data[2:4]),
	}
	if c.Version != 1 && c.Version != 2 {
		return fmt.Errorf("Invalid CiscoDiscovery version number %d", c.Version)
	}
	var err error
	c.Values, err = decodeCiscoDiscoveryTLVs(data[4:], p)
	if err != nil {
		return err
	}
	c.Contents = data[0:4]
	c.Payload = data[4:]
	p.AddLayer(c)
	return p.NextDecoder(gopacket.DecodeFunc(decodeCiscoDiscoveryInfo))
}

// LayerType returns gopacket.LayerTypeCiscoDiscoveryInfo.
func (c *CiscoDiscoveryInfo) LayerType() gopacket.LayerType {
	return LayerTypeCiscoDiscoveryInfo
}

func decodeCiscoDiscoveryTLVs(data []byte, p gopacket.PacketBuilder) (values []CiscoDiscoveryValue, err error) {
	for len(data) > 0 {
		if len(data) < 4 {
			p.SetTruncated()
			return nil, errors.New("CDP TLV < 4 bytes")
		}
		val := CiscoDiscoveryValue{
			Type:   CDPTLVType(binary.BigEndian.Uint16(data[:2])),
			Length: binary.BigEndian.Uint16(data[2:4]),
		}
		if val.Length < 4 {
			err = fmt.Errorf("Invalid CiscoDiscovery value length %d", val.Length)
			break
		} else if len(data) < int(val.Length) {
			p.SetTruncated()
			return nil, fmt.Errorf("CDP TLV < length %d", val.Length)
		}
		val.Value = data[4:val.Length]
		values = append(values, val)
		data = data[val.Length:]
	}
	return
}

func decodeCiscoDiscoveryInfo(data []byte, p gopacket.PacketBuilder) error {
	var err error
	info := &CiscoDiscoveryInfo{BaseLayer: BaseLayer{Contents: data}}
	p.AddLayer(info)
	values, err := decodeCiscoDiscoveryTLVs(data, p)
	if err != nil { // Unlikely, as parent decode will fail, but better safe...
		return err
	}
	for _, val := range values {
		switch val.Type {
		case CDPTLVDevID:
			info.DeviceID = string(val.Value)
		case CDPTLVAddress:
			if err = checkCDPTLVLen(val, 4); err != nil {
				return err
			}
			info.Addresses, err = decodeAddresses(val.Value)
			if err != nil {
				return err
			}
		case CDPTLVPortID:
			info.PortID = string(val.Value)
		case CDPTLVCapabilities:
			if err = checkCDPTLVLen(val, 4); err != nil {
				return err
			}
			val := CDPCapability(binary.BigEndian.Uint32(val.Value[0:4]))
			info.Capabilities.L3Router = (val&CDPCapMaskRouter > 0)
			info.Capabilities.TBBridge = (val&CDPCapMaskTBBridge > 0)
			info.Capabilities.SPBridge = (val&CDPCapMaskSPBridge > 0)
			info.Capabilities.L2Switch = (val&CDPCapMaskSwitch > 0)
			info.Capabilities.IsHost = (val&CDPCapMaskHost > 0)
			info.Capabilities.IGMPFilter = (val&CDPCapMaskIGMPFilter > 0)
			info.Capabilities.L1Repeater = (val&CDPCapMaskRepeater > 0)
			info.Capabilities.IsPhone = (val&CDPCapMaskPhone > 0)
			info.Capabilities.RemotelyManaged = (val&CDPCapMaskRemote > 0)
		case CDPTLVVersion:
			info.Version = string(val.Value)
		case CDPTLVPlatform:
			info.Platform = string(val.Value)
		case CDPTLVIPPrefix:
			v := val.Value
			l := len(v)
			if l%5 == 0 && l >= 5 {
				for len(v) > 0 {
					_, ipnet, _ := net.ParseCIDR(fmt.Sprintf("%d.%d.%d.%d/%d", v[0], v[1], v[2], v[3], v[4]))
					info.IPPrefixes = append(info.IPPrefixes, *ipnet)
					v = v[5:]
				}
			} else {
				return fmt.Errorf("Invalid TLV %v length %d", val.Type, len(val.Value))
			}
		case CDPTLVHello:
			if err = checkCDPTLVLen(val, 32); err != nil {
				return err
			}
			v := val.Value
			info.CDPHello.OUI = v[0:3]
			info.CDPHello.ProtocolID = binary.BigEndian.Uint16(v[3:5])
			info.CDPHello.ClusterMaster = v[5:9]
			info.CDPHello.Unknown1 = v[9:13]
			info.CDPHello.Version = v[13]
			info.CDPHello.SubVersion = v[14]
			info.CDPHello.Status = v[15]
			info.CDPHello.Unknown2 = v[16]
			info.CDPHello.ClusterCommander = v[17:23]
			info.CDPHello.SwitchMAC = v[23:29]
			info.CDPHello.Unknown3 = v[29]
			info.CDPHello.ManagementVLAN = binary.BigEndian.Uint16(v[30:32])
		case CDPTLVVTPDomain:
			info.VTPDomain = string(val.Value)
		case CDPTLVNativeVLAN:
			if err = checkCDPTLVLen(val, 2); err != nil {
				return err
			}
			info.NativeVLAN = binary.BigEndian.Uint16(val.Value[0:2])
		case CDPTLVFullDuplex:
			if err = checkCDPTLVLen(val, 1); err != nil {
				return err
			}
			info.FullDuplex = (val.Value[0] == 1)
		case CDPTLVVLANReply:
			if err = checkCDPTLVLen(val, 3); err != nil {
				return err
			}
			info.VLANReply.ID = uint8(val.Value[0])
			info.VLANReply.VLAN = binary.BigEndian.Uint16(val.Value[1:3])
		case CDPTLVVLANQuery:
			if err = checkCDPTLVLen(val, 3); err != nil {
				return err
			}
			info.VLANQuery.ID = uint8(val.Value[0])
			info.VLANQuery.VLAN = binary.BigEndian.Uint16(val.Value[1:3])
		case CDPTLVPower:
			if err = checkCDPTLVLen(val, 2); err != nil {
				return err
			}
			info.PowerConsumption = binary.BigEndian.Uint16(val.Value[0:2])
		case CDPTLVMTU:
			if err = checkCDPTLVLen(val, 4); err != nil {
				return err
			}
			info.MTU = binary.BigEndian.Uint32(val.Value[0:4])
		case CDPTLVExtendedTrust:
			if err = checkCDPTLVLen(val, 1); err != nil {
				return err
			}
			info.ExtendedTrust = uint8(val.Value[0])
		case CDPTLVUntrustedCOS:
			if err = checkCDPTLVLen(val, 1); err != nil {
				return err
			}
			info.UntrustedCOS = uint8(val.Value[0])
		case CDPTLVSysName:
			info.SysName = string(val.Value)
		case CDPTLVSysOID:
			info.SysOID = string(val.Value)
		case CDPTLVMgmtAddresses:
			if err = checkCDPTLVLen(val, 4); err != nil {
				return err
			}
			info.MgmtAddresses, err = decodeAddresses(val.Value)
			if err != nil {
				return err
			}
		case CDPTLVLocation:
			if err = checkCDPTLVLen(val, 2); err != nil {
				return err
			}
			info.Location.Type = uint8(val.Value[0])
			info.Location.Location = string(val.Value[1:])

			//		case CDPTLVLExternalPortID:
			//			Undocumented
		case CDPTLVPowerRequested:
			if err = checkCDPTLVLen(val, 4); err != nil {
				return err
			}
			info.PowerRequest.ID = binary.BigEndian.Uint16(val.Value[0:2])
			info.PowerRequest.MgmtID = binary.BigEndian.Uint16(val.Value[2:4])
			for n := 4; n < len(val.Value); n += 4 {
				info.PowerRequest.Values = append(info.PowerRequest.Values, binary.BigEndian.Uint32(val.Value[n:n+4]))
			}
		case CDPTLVPowerAvailable:
			if err = checkCDPTLVLen(val, 4); err != nil {
				return err
			}
			info.PowerAvailable.ID = binary.BigEndian.Uint16(val.Value[0:2])
			info.PowerAvailable.MgmtID = binary.BigEndian.Uint16(val.Value[2:4])
			for n := 4; n < len(val.Value); n += 4 {
				info.PowerAvailable.Values = append(info.PowerAvailable.Values, binary.BigEndian.Uint32(val.Value[n:n+4]))
			}
			//		case CDPTLVPortUnidirectional
			//			Undocumented
		case CDPTLVEnergyWise:
			if err = checkCDPTLVLen(val, 72); err != nil {
				return err
			}
			info.EnergyWise.EncryptedData = val.Value[0:20]
			info.EnergyWise.Unknown1 = binary.BigEndian.Uint32(val.Value[20:24])
			info.EnergyWise.SequenceNumber = binary.BigEndian.Uint32(val.Value[24:28])
			info.EnergyWise.ModelNumber = string(val.Value[28:44])
			info.EnergyWise.Unknown2 = binary.BigEndian.Uint16(val.Value[44:46])
			info.EnergyWise.HardwareID = string(val.Value[46:49])
			info.EnergyWise.SerialNum = string(val.Value[49:60])
			info.EnergyWise.Unknown3 = val.Value[60:68]
			tlvLen := binary.BigEndian.Uint16(val.Value[68:70])
			tlvNum := binary.BigEndian.Uint16(val.Value[70:72])
			data := val.Value[72:]
			if len(data) < int(tlvLen) {
				return fmt.Errorf("Invalid TLV length %d vs %d", tlvLen, len(data))
			}
			numSeen := 0
			for len(data) > 8 {
				numSeen++
				if numSeen > int(tlvNum) { // Too many TLV's ?
					return fmt.Errorf("Too many TLV's - wanted %d, saw %d", tlvNum, numSeen)
				}
				tType := CDPEnergyWiseSubtype(binary.BigEndian.Uint32(data[0:4]))
				tLen := int(binary.BigEndian.Uint32(data[4:8]))
				if tLen > len(data)-8 {
					return fmt.Errorf("Invalid TLV length %d vs %d", tLen, len(data)-8)
				}
				data = data[8:]
				switch tType {
				case CDPEnergyWiseRole:
					info.EnergyWise.Role = string(data[:])
				case CDPEnergyWiseDomain:
					info.EnergyWise.Domain = string(data[:])
				case CDPEnergyWiseName:
					info.EnergyWise.Name = string(data[:])
				case CDPEnergyWiseReplyTo:
					if len(data) >= 18 {
						info.EnergyWise.ReplyUnknown1 = data[0:2]
						info.EnergyWise.ReplyPort = data[2:4]
						info.EnergyWise.ReplyAddress = data[4:8]
						info.EnergyWise.ReplyUnknown2 = data[8:10]
						info.EnergyWise.ReplyUnknown3 = data[10:14]
					}
				}
				data = data[tLen:]
			}
		case CDPTLVSparePairPOE:
			if err = checkCDPTLVLen(val, 1); err != nil {
				return err
			}
			v := val.Value[0]
			info.SparePairPoe.PSEFourWire = (v&CDPPoEFourWire > 0)
			info.SparePairPoe.PDArchShared = (v&CDPPoEPDArch > 0)
			info.SparePairPoe.PDRequestOn = (v&CDPPoEPDRequest > 0)
			info.SparePairPoe.PSEOn = (v&CDPPoEPSE > 0)
		default:
			info.Unknown = append(info.Unknown, val)
		}
	}
	return nil
}

// CDP Protocol Types
const (
	CDPProtocolTypeNLPID byte = 1
	CDPProtocolType802_2 byte = 2
)

// CDPAddressType is used to define TLV values within CDP addresses.
type CDPAddressType uint64

// CDP Address types.
const (
	CDPAddressTypeCLNP      CDPAddressType = 0x81
	CDPAddressTypeIPV4      CDPAddressType = 0xcc
	CDPAddressTypeIPV6      CDPAddressType = 0xaaaa030000000800
	CDPAddressTypeDECNET    CDPAddressType = 0xaaaa030000006003
	CDPAddressTypeAPPLETALK CDPAddressType = 0xaaaa03000000809b
	CDPAddressTypeIPX       CDPAddressType = 0xaaaa030000008137
	CDPAddressTypeVINES     CDPAddressType = 0xaaaa0300000080c4
	CDPAddressTypeXNS       CDPAddressType = 0xaaaa030000000600
	CDPAddressTypeAPOLLO    CDPAddressType = 0xaaaa030000008019
)

func decodeAddresses(v []byte) (addresses []net.IP, err error) {
	numaddr := int(binary.BigEndian.Uint32(v[0:4]))
	if numaddr < 1 {
		return nil, fmt.Errorf("Invalid Address TLV number %d", numaddr)
	}
	v = v[4:]
	if len(v) < numaddr*8 {
		return nil, fmt.Errorf("Invalid Address TLV length %d", len(v))
	}
	for i := 0; i < numaddr; i++ {
		prottype := v[0]
		if prottype != CDPProtocolTypeNLPID && prottype != CDPProtocolType802_2 { // invalid protocol type
			return nil, fmt.Errorf("Invalid Address Protocol %d", prottype)
		}
		protlen := int(v[1])
		if (prottype == CDPProtocolTypeNLPID && protlen != 1) ||
			(prottype == CDPProtocolType802_2 && protlen != 3 && protlen != 8) { // invalid length
			return nil, fmt.Errorf("Invalid Address Protocol length %d", protlen)
		}
		plen := make([]byte, 8)
		copy(plen[8-protlen:], v[2:2+protlen])
		protocol := CDPAddressType(binary.BigEndian.Uint64(plen))
		v = v[2+protlen:]
		addrlen := binary.BigEndian.Uint16(v[0:2])
		ab := v[2 : 2+addrlen]
		if protocol == CDPAddressTypeIPV4 && addrlen == 4 {
			addresses = append(addresses, net.IPv4(ab[0], ab[1], ab[2], ab[3]))
		} else if protocol == CDPAddressTypeIPV6 && addrlen == 16 {
			addresses = append(addresses, net.IP(ab))
		} else {
			// only handle IPV4 & IPV6 for now
		}
		v = v[2+addrlen:]
		if len(v) < 8 {
			break
		}
	}
	return
}

func (t CDPTLVType) String() (s string) {
	switch t {
	case CDPTLVDevID:
		s = "Device ID"
	case CDPTLVAddress:
		s = "Addresses"
	case CDPTLVPortID:
		s = "Port ID"
	case CDPTLVCapabilities:
		s = "Capabilities"
	case CDPTLVVersion:
		s = "Software Version"
	case CDPTLVPlatform:
		s = "Platform"
	case CDPTLVIPPrefix:
		s = "IP Prefix"
	case CDPTLVHello:
		s = "Protocol Hello"
	case CDPTLVVTPDomain:
		s = "VTP Management Domain"
	case CDPTLVNativeVLAN:
		s = "Native VLAN"
	case CDPTLVFullDuplex:
		s = "Full Duplex"
	case CDPTLVVLANReply:
		s = "VoIP VLAN Reply"
	case CDPTLVVLANQuery:
		s = "VLANQuery"
	case CDPTLVPower:
		s = "Power consumption"
	case CDPTLVMTU:
		s = "MTU"
	case CDPTLVExtendedTrust:
		s = "Extended Trust Bitmap"
	case CDPTLVUntrustedCOS:
		s = "Untrusted Port CoS"
	case CDPTLVSysName:
		s = "System Name"
	case CDPTLVSysOID:
		s = "System OID"
	case CDPTLVMgmtAddresses:
		s = "Management Addresses"
	case CDPTLVLocation:
		s = "Location"
	case CDPTLVExternalPortID:
		s = "External Port ID"
	case CDPTLVPowerRequested:
		s = "Power Requested"
	case CDPTLVPowerAvailable:
		s = "Power Available"
	case CDPTLVPortUnidirectional:
		s = "Port Unidirectional"
	case CDPTLVEnergyWise:
		s = "Energy Wise"
	case CDPTLVSparePairPOE:
		s = "Spare Pair POE"
	default:
		s = "Unknown"
	}
	return
}

func (a CDPAddressType) String() (s string) {
	switch a {
	case CDPAddressTypeCLNP:
		s = "Connectionless Network Protocol"
	case CDPAddressTypeIPV4:
		s = "IPv4"
	case CDPAddressTypeIPV6:
		s = "IPv6"
	case CDPAddressTypeDECNET:
		s = "DECnet Phase IV"
	case CDPAddressTypeAPPLETALK:
		s = "Apple Talk"
	case CDPAddressTypeIPX:
		s = "Novell IPX"
	case CDPAddressTypeVINES:
		s = "Banyan VINES"
	case CDPAddressTypeXNS:
		s = "Xerox Network Systems"
	case CDPAddressTypeAPOLLO:
		s = "Apollo"
	default:
		s = "Unknown"
	}
	return
}

func (t CDPEnergyWiseSubtype) String() (s string) {
	switch t {
	case CDPEnergyWiseRole:
		s = "Role"
	case CDPEnergyWiseDomain:
		s = "Domain"
	case CDPEnergyWiseName:
		s = "Name"
	case CDPEnergyWiseReplyTo:
		s = "ReplyTo"
	default:
		s = "Unknown"
	}
	return
}

func checkCDPTLVLen(v CiscoDiscoveryValue, l int) (err error) {
	if len(v.Value) < l {
		err = fmt.Errorf("Invalid TLV %v length %d", v.Type, len(v.Value))
	}
	return
}
