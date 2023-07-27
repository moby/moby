// Copyright 2018 The GoPacket Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/google/gopacket"
)

// DHCPv6Opt represents a DHCP option or parameter from RFC-3315
type DHCPv6Opt uint16

// Constants for the DHCPv6Opt options.
const (
	DHCPv6OptClientID           DHCPv6Opt = 1
	DHCPv6OptServerID           DHCPv6Opt = 2
	DHCPv6OptIANA               DHCPv6Opt = 3
	DHCPv6OptIATA               DHCPv6Opt = 4
	DHCPv6OptIAAddr             DHCPv6Opt = 5
	DHCPv6OptOro                DHCPv6Opt = 6
	DHCPv6OptPreference         DHCPv6Opt = 7
	DHCPv6OptElapsedTime        DHCPv6Opt = 8
	DHCPv6OptRelayMessage       DHCPv6Opt = 9
	DHCPv6OptAuth               DHCPv6Opt = 11
	DHCPv6OptUnicast            DHCPv6Opt = 12
	DHCPv6OptStatusCode         DHCPv6Opt = 13
	DHCPv6OptRapidCommit        DHCPv6Opt = 14
	DHCPv6OptUserClass          DHCPv6Opt = 15
	DHCPv6OptVendorClass        DHCPv6Opt = 16
	DHCPv6OptVendorOpts         DHCPv6Opt = 17
	DHCPv6OptInterfaceID        DHCPv6Opt = 18
	DHCPv6OptReconfigureMessage DHCPv6Opt = 19
	DHCPv6OptReconfigureAccept  DHCPv6Opt = 20

	// RFC 3319 Session Initiation Protocol (SIP)
	DHCPv6OptSIPServersDomainList  DHCPv6Opt = 21
	DHCPv6OptSIPServersAddressList DHCPv6Opt = 22

	// RFC 3646 DNS Configuration
	DHCPv6OptDNSServers DHCPv6Opt = 23
	DHCPv6OptDomainList DHCPv6Opt = 24

	// RFC 3633 Prefix Delegation
	DHCPv6OptIAPD     DHCPv6Opt = 25
	DHCPv6OptIAPrefix DHCPv6Opt = 26

	// RFC 3898 Network Information Service (NIS)
	DHCPv6OptNISServers     DHCPv6Opt = 27
	DHCPv6OptNISPServers    DHCPv6Opt = 28
	DHCPv6OptNISDomainName  DHCPv6Opt = 29
	DHCPv6OptNISPDomainName DHCPv6Opt = 30

	// RFC 4075 Simple Network Time Protocol (SNTP)
	DHCPv6OptSNTPServers DHCPv6Opt = 31

	// RFC 4242 Information Refresh Time Option
	DHCPv6OptInformationRefreshTime DHCPv6Opt = 32

	// RFC 4280 Broadcast and Multicast Control Servers
	DHCPv6OptBCMCSServerDomainNameList DHCPv6Opt = 33
	DHCPv6OptBCMCSServerAddressList    DHCPv6Opt = 34

	// RFC 4776 Civic Address ConfigurationOption
	DHCPv6OptGeoconfCivic DHCPv6Opt = 36

	// RFC 4649 Relay Agent Remote-ID
	DHCPv6OptRemoteID DHCPv6Opt = 37

	// RFC 4580 Relay Agent Subscriber-ID
	DHCPv6OptSubscriberID DHCPv6Opt = 38

	// RFC 4704 Client Full Qualified Domain Name (FQDN)
	DHCPv6OptClientFQDN DHCPv6Opt = 39

	// RFC 5192 Protocol for Carrying Authentication for Network Access (PANA)
	DHCPv6OptPanaAgent DHCPv6Opt = 40

	// RFC 4833 Timezone Options
	DHCPv6OptNewPOSIXTimezone DHCPv6Opt = 41
	DHCPv6OptNewTZDBTimezone  DHCPv6Opt = 42

	// RFC 4994 Relay Agent Echo Request
	DHCPv6OptEchoRequestOption DHCPv6Opt = 43

	// RFC 5007 Leasequery
	DHCPv6OptLQQuery      DHCPv6Opt = 44
	DHCPv6OptCLTTime      DHCPv6Opt = 45
	DHCPv6OptClientData   DHCPv6Opt = 46
	DHCPv6OptLQRelayData  DHCPv6Opt = 47
	DHCPv6OptLQClientLink DHCPv6Opt = 48

	// RFC 6610 Home Information Discovery in Mobile IPv6 (MIPv6)
	DHCPv6OptMIP6HNIDF DHCPv6Opt = 49
	DHCPv6OptMIP6VDINF DHCPv6Opt = 50
	DHCPv6OptMIP6IDINF DHCPv6Opt = 69
	DHCPv6OptMIP6UDINF DHCPv6Opt = 70
	DHCPv6OptMIP6HNP   DHCPv6Opt = 71
	DHCPv6OptMIP6HAA   DHCPv6Opt = 72
	DHCPv6OptMIP6HAF   DHCPv6Opt = 73

	// RFC 5223 Discovering Location-to-Service Translation (LoST) Servers
	DHCPv6OptV6LOST DHCPv6Opt = 51

	// RFC 5417 Control And Provisioning of Wireless Access Points (CAPWAP)
	DHCPv6OptCAPWAPACV6 DHCPv6Opt = 52

	// RFC 5460 Bulk Leasequery
	DHCPv6OptRelayID DHCPv6Opt = 53

	// RFC 5678 IEEE 802.21 Mobility Services (MoS) Discovery
	DHCPv6OptIPv6AddressMoS DHCPv6Opt = 54
	DHCPv6OptIPv6FQDNMoS    DHCPv6Opt = 55

	// RFC 5908 NTP Server Option
	DHCPv6OptNTPServer DHCPv6Opt = 56

	// RFC 5986 Discovering the Local Location Information Server (LIS)
	DHCPv6OptV6AccessDomain DHCPv6Opt = 57

	// RFC 5986 SIP User Agent
	DHCPv6OptSIPUACSList DHCPv6Opt = 58

	// RFC 5970 Options for Network Boot
	DHCPv6OptBootFileURL    DHCPv6Opt = 59
	DHCPv6OptBootFileParam  DHCPv6Opt = 60
	DHCPv6OptClientArchType DHCPv6Opt = 61
	DHCPv6OptNII            DHCPv6Opt = 62

	// RFC 6225 Coordinate-Based Location Configuration Information
	DHCPv6OptGeolocation DHCPv6Opt = 63

	// RFC 6334 Dual-Stack Lite
	DHCPv6OptAFTRName DHCPv6Opt = 64

	// RFC 6440 EAP Re-authentication Protocol (ERP)
	DHCPv6OptERPLocalDomainName DHCPv6Opt = 65

	// RFC 6422 Relay-Supplied DHCP Options
	DHCPv6OptRSOO DHCPv6Opt = 66

	// RFC 6603 Prefix Exclude Option for DHCPv6-based Prefix Delegation
	DHCPv6OptPDExclude DHCPv6Opt = 67

	// RFC 6607 Virtual Subnet Selection
	DHCPv6OptVSS DHCPv6Opt = 68

	// RFC 6731 Improved Recursive DNS Server Selection for Multi-Interfaced Nodes
	DHCPv6OptRDNSSSelection DHCPv6Opt = 74

	// RFC 6784 Kerberos Options for DHCPv6
	DHCPv6OptKRBPrincipalName DHCPv6Opt = 75
	DHCPv6OptKRBRealmName     DHCPv6Opt = 76
	DHCPv6OptKRBKDC           DHCPv6Opt = 77

	// RFC 6939 Client Link-Layer Address Option
	DHCPv6OptClientLinkLayerAddress DHCPv6Opt = 79

	// RFC 6977 Triggering DHCPv6 Reconfiguration from Relay Agents
	DHCPv6OptLinkAddress DHCPv6Opt = 80

	// RFC 7037 RADIUS Option for the DHCPv6 Relay Agent
	DHCPv6OptRADIUS DHCPv6Opt = 81

	// RFC 7083 Modification to Default Values of SOL_MAX_RT and INF_MAX_RT
	DHCPv6OptSolMaxRt DHCPv6Opt = 82
	DHCPv6OptInfMaxRt DHCPv6Opt = 83

	// RFC 7078 Distributing Address Selection Policy
	DHCPv6OptAddrSel      DHCPv6Opt = 84
	DHCPv6OptAddrSelTable DHCPv6Opt = 85

	// RFC 7291 DHCP Options for the Port Control Protocol (PCP)
	DHCPv6OptV6PCPServer DHCPv6Opt = 86

	// RFC 7341 DHCPv4-over-DHCPv6 (DHCP 4o6) Transport
	DHCPv6OptDHCPv4Message          DHCPv6Opt = 87
	DHCPv6OptDHCPv4OverDHCPv6Server DHCPv6Opt = 88

	// RFC 7598 Configuration of Softwire Address and Port-Mapped Clients
	DHCPv6OptS46Rule           DHCPv6Opt = 89
	DHCPv6OptS46BR             DHCPv6Opt = 90
	DHCPv6OptS46DMR            DHCPv6Opt = 91
	DHCPv6OptS46V4V4Bind       DHCPv6Opt = 92
	DHCPv6OptS46PortParameters DHCPv6Opt = 93
	DHCPv6OptS46ContMAPE       DHCPv6Opt = 94
	DHCPv6OptS46ContMAPT       DHCPv6Opt = 95
	DHCPv6OptS46ContLW         DHCPv6Opt = 96

	// RFC 7600 IPv4 Residual Deployment via IPv6
	DHCPv6Opt4RD           DHCPv6Opt = 97
	DHCPv6Opt4RDMapRule    DHCPv6Opt = 98
	DHCPv6Opt4RDNonMapRule DHCPv6Opt = 99

	// RFC 7653 Active Leasequery
	DHCPv6OptLQBaseTime  DHCPv6Opt = 100
	DHCPv6OptLQStartTime DHCPv6Opt = 101
	DHCPv6OptLQEndTime   DHCPv6Opt = 102

	// RFC 7710 Captive-Portal Identification
	DHCPv6OptCaptivePortal DHCPv6Opt = 103

	// RFC 7774 Multicast Protocol for Low-Power and Lossy Networks (MPL) Parameter Configuration
	DHCPv6OptMPLParameters DHCPv6Opt = 104

	// RFC 7839 Access-Network-Identifier (ANI)
	DHCPv6OptANIATT           DHCPv6Opt = 105
	DHCPv6OptANINetworkName   DHCPv6Opt = 106
	DHCPv6OptANIAPName        DHCPv6Opt = 107
	DHCPv6OptANIAPBSSID       DHCPv6Opt = 108
	DHCPv6OptANIOperatorID    DHCPv6Opt = 109
	DHCPv6OptANIOperatorRealm DHCPv6Opt = 110

	// RFC 8026 Unified IPv4-in-IPv6 Softwire Customer Premises Equipment (CPE)
	DHCPv6OptS46Priority DHCPv6Opt = 111

	// draft-ietf-opsawg-mud-25 Manufacturer Usage Description (MUD)
	DHCPv6OptMUDURLV6 DHCPv6Opt = 112

	// RFC 8115 IPv4-Embedded Multicast and Unicast IPv6 Prefixes
	DHCPv6OptV6Prefix64 DHCPv6Opt = 113

	// RFC 8156 DHCPv6 Failover Protocol
	DHCPv6OptFBindingStatus           DHCPv6Opt = 114
	DHCPv6OptFConnectFlags            DHCPv6Opt = 115
	DHCPv6OptFDNSRemovalInfo          DHCPv6Opt = 116
	DHCPv6OptFDNSHostName             DHCPv6Opt = 117
	DHCPv6OptFDNSZoneName             DHCPv6Opt = 118
	DHCPv6OptFDNSFlags                DHCPv6Opt = 119
	DHCPv6OptFExpirationTime          DHCPv6Opt = 120
	DHCPv6OptFMaxUnacknowledgedBNDUPD DHCPv6Opt = 121
	DHCPv6OptFMCLT                    DHCPv6Opt = 122
	DHCPv6OptFPartnerLifetime         DHCPv6Opt = 123
	DHCPv6OptFPartnerLifetimeSent     DHCPv6Opt = 124
	DHCPv6OptFPartnerDownTime         DHCPv6Opt = 125
	DHCPv6OptFPartnerRawCltTime       DHCPv6Opt = 126
	DHCPv6OptFProtocolVersion         DHCPv6Opt = 127
	DHCPv6OptFKeepaliveTime           DHCPv6Opt = 128
	DHCPv6OptFReconfigureData         DHCPv6Opt = 129
	DHCPv6OptFRelationshipName        DHCPv6Opt = 130
	DHCPv6OptFServerFlags             DHCPv6Opt = 131
	DHCPv6OptFServerState             DHCPv6Opt = 132
	DHCPv6OptFStartTimeOfState        DHCPv6Opt = 133
	DHCPv6OptFStateExpirationTime     DHCPv6Opt = 134

	// RFC 8357 Generalized UDP Source Port for DHCP Relay
	DHCPv6OptRelayPort DHCPv6Opt = 135

	// draft-ietf-netconf-zerotouch-25 Zero Touch Provisioning for Networking Devices
	DHCPv6OptV6ZeroTouchRedirect DHCPv6Opt = 136

	// RFC 6153 Access Network Discovery and Selection Function (ANDSF) Discovery
	DHCPv6OptIPV6AddressANDSF DHCPv6Opt = 143
)

// String returns a string version of a DHCPv6Opt.
func (o DHCPv6Opt) String() string {
	switch o {
	case DHCPv6OptClientID:
		return "ClientID"
	case DHCPv6OptServerID:
		return "ServerID"
	case DHCPv6OptIANA:
		return "IA_NA"
	case DHCPv6OptIATA:
		return "IA_TA"
	case DHCPv6OptIAAddr:
		return "IAAddr"
	case DHCPv6OptOro:
		return "Oro"
	case DHCPv6OptPreference:
		return "Preference"
	case DHCPv6OptElapsedTime:
		return "ElapsedTime"
	case DHCPv6OptRelayMessage:
		return "RelayMessage"
	case DHCPv6OptAuth:
		return "Auth"
	case DHCPv6OptUnicast:
		return "Unicast"
	case DHCPv6OptStatusCode:
		return "StatusCode"
	case DHCPv6OptRapidCommit:
		return "RapidCommit"
	case DHCPv6OptUserClass:
		return "UserClass"
	case DHCPv6OptVendorClass:
		return "VendorClass"
	case DHCPv6OptVendorOpts:
		return "VendorOpts"
	case DHCPv6OptInterfaceID:
		return "InterfaceID"
	case DHCPv6OptReconfigureMessage:
		return "ReconfigureMessage"
	case DHCPv6OptReconfigureAccept:
		return "ReconfigureAccept"
	case DHCPv6OptSIPServersDomainList:
		return "SIPServersDomainList"
	case DHCPv6OptSIPServersAddressList:
		return "SIPServersAddressList"
	case DHCPv6OptDNSServers:
		return "DNSRecursiveNameServer"
	case DHCPv6OptDomainList:
		return "DomainSearchList"
	case DHCPv6OptIAPD:
		return "IdentityAssociationPrefixDelegation"
	case DHCPv6OptIAPrefix:
		return "IAPDPrefix"
	case DHCPv6OptNISServers:
		return "NISServers"
	case DHCPv6OptNISPServers:
		return "NISv2Servers"
	case DHCPv6OptNISDomainName:
		return "NISDomainName"
	case DHCPv6OptNISPDomainName:
		return "NISv2DomainName"
	case DHCPv6OptSNTPServers:
		return "SNTPServers"
	case DHCPv6OptInformationRefreshTime:
		return "InformationRefreshTime"
	case DHCPv6OptBCMCSServerDomainNameList:
		return "BCMCSControlServersDomainNameList"
	case DHCPv6OptBCMCSServerAddressList:
		return "BCMCSControlServersAddressList"
	case DHCPv6OptGeoconfCivic:
		return "CivicAddress"
	case DHCPv6OptRemoteID:
		return "RelayAgentRemoteID"
	case DHCPv6OptSubscriberID:
		return "RelayAgentSubscriberID"
	case DHCPv6OptClientFQDN:
		return "ClientFQDN"
	case DHCPv6OptPanaAgent:
		return "PANAAuthenticationAgent"
	case DHCPv6OptNewPOSIXTimezone:
		return "NewPOSIXTimezone"
	case DHCPv6OptNewTZDBTimezone:
		return "NewTZDBTimezone"
	case DHCPv6OptEchoRequestOption:
		return "EchoRequest"
	case DHCPv6OptLQQuery:
		return "LeasequeryQuery"
	case DHCPv6OptClientData:
		return "LeasequeryClientData"
	case DHCPv6OptCLTTime:
		return "LeasequeryClientLastTransactionTime"
	case DHCPv6OptLQRelayData:
		return "LeasequeryRelayData"
	case DHCPv6OptLQClientLink:
		return "LeasequeryClientLink"
	case DHCPv6OptMIP6HNIDF:
		return "MIPv6HomeNetworkIDFQDN"
	case DHCPv6OptMIP6VDINF:
		return "MIPv6VisitedHomeNetworkInformation"
	case DHCPv6OptMIP6IDINF:
		return "MIPv6IdentifiedHomeNetworkInformation"
	case DHCPv6OptMIP6UDINF:
		return "MIPv6UnrestrictedHomeNetworkInformation"
	case DHCPv6OptMIP6HNP:
		return "MIPv6HomeNetworkPrefix"
	case DHCPv6OptMIP6HAA:
		return "MIPv6HomeAgentAddress"
	case DHCPv6OptMIP6HAF:
		return "MIPv6HomeAgentFQDN"
	case DHCPv6OptV6LOST:
		return "LoST Server"
	case DHCPv6OptCAPWAPACV6:
		return "CAPWAPAccessControllerV6"
	case DHCPv6OptRelayID:
		return "LeasequeryRelayID"
	case DHCPv6OptIPv6AddressMoS:
		return "MoSIPv6Address"
	case DHCPv6OptIPv6FQDNMoS:
		return "MoSDomainNameList"
	case DHCPv6OptNTPServer:
		return "NTPServer"
	case DHCPv6OptV6AccessDomain:
		return "AccessNetworkDomainName"
	case DHCPv6OptSIPUACSList:
		return "SIPUserAgentConfigurationServiceDomains"
	case DHCPv6OptBootFileURL:
		return "BootFileURL"
	case DHCPv6OptBootFileParam:
		return "BootFileParameters"
	case DHCPv6OptClientArchType:
		return "ClientSystemArchitectureType"
	case DHCPv6OptNII:
		return "ClientNetworkInterfaceIdentifier"
	case DHCPv6OptGeolocation:
		return "Geolocation"
	case DHCPv6OptAFTRName:
		return "AFTRName"
	case DHCPv6OptERPLocalDomainName:
		return "AFTRName"
	case DHCPv6OptRSOO:
		return "RSOOption"
	case DHCPv6OptPDExclude:
		return "PrefixExclude"
	case DHCPv6OptVSS:
		return "VirtualSubnetSelection"
	case DHCPv6OptRDNSSSelection:
		return "RDNSSSelection"
	case DHCPv6OptKRBPrincipalName:
		return "KerberosPrincipalName"
	case DHCPv6OptKRBRealmName:
		return "KerberosRealmName"
	case DHCPv6OptKRBKDC:
		return "KerberosKDC"
	case DHCPv6OptClientLinkLayerAddress:
		return "ClientLinkLayerAddress"
	case DHCPv6OptLinkAddress:
		return "LinkAddress"
	case DHCPv6OptRADIUS:
		return "RADIUS"
	case DHCPv6OptSolMaxRt:
		return "SolMaxRt"
	case DHCPv6OptInfMaxRt:
		return "InfMaxRt"
	case DHCPv6OptAddrSel:
		return "AddressSelection"
	case DHCPv6OptAddrSelTable:
		return "AddressSelectionTable"
	case DHCPv6OptV6PCPServer:
		return "PCPServer"
	case DHCPv6OptDHCPv4Message:
		return "DHCPv4Message"
	case DHCPv6OptDHCPv4OverDHCPv6Server:
		return "DHCP4o6ServerAddress"
	case DHCPv6OptS46Rule:
		return "S46Rule"
	case DHCPv6OptS46BR:
		return "S46BR"
	case DHCPv6OptS46DMR:
		return "S46DMR"
	case DHCPv6OptS46V4V4Bind:
		return "S46IPv4IPv6AddressBinding"
	case DHCPv6OptS46PortParameters:
		return "S46PortParameters"
	case DHCPv6OptS46ContMAPE:
		return "S46MAPEContainer"
	case DHCPv6OptS46ContMAPT:
		return "S46MAPTContainer"
	case DHCPv6OptS46ContLW:
		return "S46Lightweight4Over6Container"
	case DHCPv6Opt4RD:
		return "4RD"
	case DHCPv6Opt4RDMapRule:
		return "4RDMapRule"
	case DHCPv6Opt4RDNonMapRule:
		return "4RDNonMapRule"
	case DHCPv6OptLQBaseTime:
		return "LQBaseTime"
	case DHCPv6OptLQStartTime:
		return "LQStartTime"
	case DHCPv6OptLQEndTime:
		return "LQEndTime"
	case DHCPv6OptCaptivePortal:
		return "CaptivePortal"
	case DHCPv6OptMPLParameters:
		return "MPLParameterConfiguration"
	case DHCPv6OptANIATT:
		return "ANIAccessTechnologyType"
	case DHCPv6OptANINetworkName:
		return "ANINetworkName"
	case DHCPv6OptANIAPName:
		return "ANIAccessPointName"
	case DHCPv6OptANIAPBSSID:
		return "ANIAccessPointBSSID"
	case DHCPv6OptANIOperatorID:
		return "ANIOperatorIdentifier"
	case DHCPv6OptANIOperatorRealm:
		return "ANIOperatorRealm"
	case DHCPv6OptS46Priority:
		return "S64Priority"
	case DHCPv6OptMUDURLV6:
		return "ManufacturerUsageDescriptionURL"
	case DHCPv6OptV6Prefix64:
		return "V6Prefix64"
	case DHCPv6OptFBindingStatus:
		return "FailoverBindingStatus"
	case DHCPv6OptFConnectFlags:
		return "FailoverConnectFlags"
	case DHCPv6OptFDNSRemovalInfo:
		return "FailoverDNSRemovalInfo"
	case DHCPv6OptFDNSHostName:
		return "FailoverDNSHostName"
	case DHCPv6OptFDNSZoneName:
		return "FailoverDNSZoneName"
	case DHCPv6OptFDNSFlags:
		return "FailoverDNSFlags"
	case DHCPv6OptFExpirationTime:
		return "FailoverExpirationTime"
	case DHCPv6OptFMaxUnacknowledgedBNDUPD:
		return "FailoverMaxUnacknowledgedBNDUPDMessages"
	case DHCPv6OptFMCLT:
		return "FailoverMaximumClientLeadTime"
	case DHCPv6OptFPartnerLifetime:
		return "FailoverPartnerLifetime"
	case DHCPv6OptFPartnerLifetimeSent:
		return "FailoverPartnerLifetimeSent"
	case DHCPv6OptFPartnerDownTime:
		return "FailoverPartnerDownTime"
	case DHCPv6OptFPartnerRawCltTime:
		return "FailoverPartnerRawClientLeadTime"
	case DHCPv6OptFProtocolVersion:
		return "FailoverProtocolVersion"
	case DHCPv6OptFKeepaliveTime:
		return "FailoverKeepaliveTime"
	case DHCPv6OptFReconfigureData:
		return "FailoverReconfigureData"
	case DHCPv6OptFRelationshipName:
		return "FailoverRelationshipName"
	case DHCPv6OptFServerFlags:
		return "FailoverServerFlags"
	case DHCPv6OptFServerState:
		return "FailoverServerState"
	case DHCPv6OptFStartTimeOfState:
		return "FailoverStartTimeOfState"
	case DHCPv6OptFStateExpirationTime:
		return "FailoverStateExpirationTime"
	case DHCPv6OptRelayPort:
		return "RelayPort"
	case DHCPv6OptV6ZeroTouchRedirect:
		return "ZeroTouch"
	case DHCPv6OptIPV6AddressANDSF:
		return "ANDSFIPv6Address"
	default:
		return fmt.Sprintf("Unknown(%d)", uint16(o))
	}
}

// DHCPv6Options is used to get nicely printed option lists which would normally
// be cut off after 5 options.
type DHCPv6Options []DHCPv6Option

// String returns a string version of the options list.
func (o DHCPv6Options) String() string {
	buf := &bytes.Buffer{}
	buf.WriteByte('[')
	for i, opt := range o {
		buf.WriteString(opt.String())
		if i+1 != len(o) {
			buf.WriteString(", ")
		}
	}
	buf.WriteByte(']')
	return buf.String()
}

// DHCPv6Option rerpresents a DHCP option.
type DHCPv6Option struct {
	Code   DHCPv6Opt
	Length uint16
	Data   []byte
}

// String returns a string version of a DHCP Option.
func (o DHCPv6Option) String() string {
	switch o.Code {
	case DHCPv6OptClientID, DHCPv6OptServerID:
		duid, err := decodeDHCPv6DUID(o.Data)
		if err != nil {
			return fmt.Sprintf("Option(%s:INVALID)", o.Code)
		}
		return fmt.Sprintf("Option(%s:[%s])", o.Code, duid.String())
	case DHCPv6OptOro:
		options := ""
		for i := 0; i < int(o.Length); i += 2 {
			if options != "" {
				options += ","
			}
			option := DHCPv6Opt(binary.BigEndian.Uint16(o.Data[i : i+2]))
			options += option.String()
		}
		return fmt.Sprintf("Option(%s:[%s])", o.Code, options)
	default:
		return fmt.Sprintf("Option(%s:%v)", o.Code, o.Data)
	}
}

// NewDHCPv6Option constructs a new DHCPv6Option with a given type and data.
func NewDHCPv6Option(code DHCPv6Opt, data []byte) DHCPv6Option {
	o := DHCPv6Option{Code: code}
	if data != nil {
		o.Data = data
		o.Length = uint16(len(data))
	}

	return o
}

func (o *DHCPv6Option) encode(b []byte, opts gopacket.SerializeOptions) error {
	binary.BigEndian.PutUint16(b[0:2], uint16(o.Code))
	if opts.FixLengths {
		binary.BigEndian.PutUint16(b[2:4], uint16(len(o.Data)))
	} else {
		binary.BigEndian.PutUint16(b[2:4], o.Length)
	}
	copy(b[4:], o.Data)

	return nil
}

func (o *DHCPv6Option) decode(data []byte) error {
	if len(data) < 4 {
		return errors.New("not enough data to decode")
	}
	o.Code = DHCPv6Opt(binary.BigEndian.Uint16(data[0:2]))
	o.Length = binary.BigEndian.Uint16(data[2:4])
	if len(data) < 4+int(o.Length) {
		return fmt.Errorf("dhcpv6 option size < length %d", 4+o.Length)
	}
	o.Data = data[4 : 4+o.Length]
	return nil
}
