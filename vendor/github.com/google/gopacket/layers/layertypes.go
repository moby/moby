// Copyright 2012 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"github.com/google/gopacket"
)

var (
	LayerTypeARP                          = gopacket.RegisterLayerType(10, gopacket.LayerTypeMetadata{Name: "ARP", Decoder: gopacket.DecodeFunc(decodeARP)})
	LayerTypeCiscoDiscovery               = gopacket.RegisterLayerType(11, gopacket.LayerTypeMetadata{Name: "CiscoDiscovery", Decoder: gopacket.DecodeFunc(decodeCiscoDiscovery)})
	LayerTypeEthernetCTP                  = gopacket.RegisterLayerType(12, gopacket.LayerTypeMetadata{Name: "EthernetCTP", Decoder: gopacket.DecodeFunc(decodeEthernetCTP)})
	LayerTypeEthernetCTPForwardData       = gopacket.RegisterLayerType(13, gopacket.LayerTypeMetadata{Name: "EthernetCTPForwardData", Decoder: nil})
	LayerTypeEthernetCTPReply             = gopacket.RegisterLayerType(14, gopacket.LayerTypeMetadata{Name: "EthernetCTPReply", Decoder: nil})
	LayerTypeDot1Q                        = gopacket.RegisterLayerType(15, gopacket.LayerTypeMetadata{Name: "Dot1Q", Decoder: gopacket.DecodeFunc(decodeDot1Q)})
	LayerTypeEtherIP                      = gopacket.RegisterLayerType(16, gopacket.LayerTypeMetadata{Name: "EtherIP", Decoder: gopacket.DecodeFunc(decodeEtherIP)})
	LayerTypeEthernet                     = gopacket.RegisterLayerType(17, gopacket.LayerTypeMetadata{Name: "Ethernet", Decoder: gopacket.DecodeFunc(decodeEthernet)})
	LayerTypeGRE                          = gopacket.RegisterLayerType(18, gopacket.LayerTypeMetadata{Name: "GRE", Decoder: gopacket.DecodeFunc(decodeGRE)})
	LayerTypeICMPv4                       = gopacket.RegisterLayerType(19, gopacket.LayerTypeMetadata{Name: "ICMPv4", Decoder: gopacket.DecodeFunc(decodeICMPv4)})
	LayerTypeIPv4                         = gopacket.RegisterLayerType(20, gopacket.LayerTypeMetadata{Name: "IPv4", Decoder: gopacket.DecodeFunc(decodeIPv4)})
	LayerTypeIPv6                         = gopacket.RegisterLayerType(21, gopacket.LayerTypeMetadata{Name: "IPv6", Decoder: gopacket.DecodeFunc(decodeIPv6)})
	LayerTypeLLC                          = gopacket.RegisterLayerType(22, gopacket.LayerTypeMetadata{Name: "LLC", Decoder: gopacket.DecodeFunc(decodeLLC)})
	LayerTypeSNAP                         = gopacket.RegisterLayerType(23, gopacket.LayerTypeMetadata{Name: "SNAP", Decoder: gopacket.DecodeFunc(decodeSNAP)})
	LayerTypeMPLS                         = gopacket.RegisterLayerType(24, gopacket.LayerTypeMetadata{Name: "MPLS", Decoder: gopacket.DecodeFunc(decodeMPLS)})
	LayerTypePPP                          = gopacket.RegisterLayerType(25, gopacket.LayerTypeMetadata{Name: "PPP", Decoder: gopacket.DecodeFunc(decodePPP)})
	LayerTypePPPoE                        = gopacket.RegisterLayerType(26, gopacket.LayerTypeMetadata{Name: "PPPoE", Decoder: gopacket.DecodeFunc(decodePPPoE)})
	LayerTypeRUDP                         = gopacket.RegisterLayerType(27, gopacket.LayerTypeMetadata{Name: "RUDP", Decoder: gopacket.DecodeFunc(decodeRUDP)})
	LayerTypeSCTP                         = gopacket.RegisterLayerType(28, gopacket.LayerTypeMetadata{Name: "SCTP", Decoder: gopacket.DecodeFunc(decodeSCTP)})
	LayerTypeSCTPUnknownChunkType         = gopacket.RegisterLayerType(29, gopacket.LayerTypeMetadata{Name: "SCTPUnknownChunkType", Decoder: nil})
	LayerTypeSCTPData                     = gopacket.RegisterLayerType(30, gopacket.LayerTypeMetadata{Name: "SCTPData", Decoder: nil})
	LayerTypeSCTPInit                     = gopacket.RegisterLayerType(31, gopacket.LayerTypeMetadata{Name: "SCTPInit", Decoder: nil})
	LayerTypeSCTPSack                     = gopacket.RegisterLayerType(32, gopacket.LayerTypeMetadata{Name: "SCTPSack", Decoder: nil})
	LayerTypeSCTPHeartbeat                = gopacket.RegisterLayerType(33, gopacket.LayerTypeMetadata{Name: "SCTPHeartbeat", Decoder: nil})
	LayerTypeSCTPError                    = gopacket.RegisterLayerType(34, gopacket.LayerTypeMetadata{Name: "SCTPError", Decoder: nil})
	LayerTypeSCTPShutdown                 = gopacket.RegisterLayerType(35, gopacket.LayerTypeMetadata{Name: "SCTPShutdown", Decoder: nil})
	LayerTypeSCTPShutdownAck              = gopacket.RegisterLayerType(36, gopacket.LayerTypeMetadata{Name: "SCTPShutdownAck", Decoder: nil})
	LayerTypeSCTPCookieEcho               = gopacket.RegisterLayerType(37, gopacket.LayerTypeMetadata{Name: "SCTPCookieEcho", Decoder: nil})
	LayerTypeSCTPEmptyLayer               = gopacket.RegisterLayerType(38, gopacket.LayerTypeMetadata{Name: "SCTPEmptyLayer", Decoder: nil})
	LayerTypeSCTPInitAck                  = gopacket.RegisterLayerType(39, gopacket.LayerTypeMetadata{Name: "SCTPInitAck", Decoder: nil})
	LayerTypeSCTPHeartbeatAck             = gopacket.RegisterLayerType(40, gopacket.LayerTypeMetadata{Name: "SCTPHeartbeatAck", Decoder: nil})
	LayerTypeSCTPAbort                    = gopacket.RegisterLayerType(41, gopacket.LayerTypeMetadata{Name: "SCTPAbort", Decoder: nil})
	LayerTypeSCTPShutdownComplete         = gopacket.RegisterLayerType(42, gopacket.LayerTypeMetadata{Name: "SCTPShutdownComplete", Decoder: nil})
	LayerTypeSCTPCookieAck                = gopacket.RegisterLayerType(43, gopacket.LayerTypeMetadata{Name: "SCTPCookieAck", Decoder: nil})
	LayerTypeTCP                          = gopacket.RegisterLayerType(44, gopacket.LayerTypeMetadata{Name: "TCP", Decoder: gopacket.DecodeFunc(decodeTCP)})
	LayerTypeUDP                          = gopacket.RegisterLayerType(45, gopacket.LayerTypeMetadata{Name: "UDP", Decoder: gopacket.DecodeFunc(decodeUDP)})
	LayerTypeIPv6HopByHop                 = gopacket.RegisterLayerType(46, gopacket.LayerTypeMetadata{Name: "IPv6HopByHop", Decoder: gopacket.DecodeFunc(decodeIPv6HopByHop)})
	LayerTypeIPv6Routing                  = gopacket.RegisterLayerType(47, gopacket.LayerTypeMetadata{Name: "IPv6Routing", Decoder: gopacket.DecodeFunc(decodeIPv6Routing)})
	LayerTypeIPv6Fragment                 = gopacket.RegisterLayerType(48, gopacket.LayerTypeMetadata{Name: "IPv6Fragment", Decoder: gopacket.DecodeFunc(decodeIPv6Fragment)})
	LayerTypeIPv6Destination              = gopacket.RegisterLayerType(49, gopacket.LayerTypeMetadata{Name: "IPv6Destination", Decoder: gopacket.DecodeFunc(decodeIPv6Destination)})
	LayerTypeIPSecAH                      = gopacket.RegisterLayerType(50, gopacket.LayerTypeMetadata{Name: "IPSecAH", Decoder: gopacket.DecodeFunc(decodeIPSecAH)})
	LayerTypeIPSecESP                     = gopacket.RegisterLayerType(51, gopacket.LayerTypeMetadata{Name: "IPSecESP", Decoder: gopacket.DecodeFunc(decodeIPSecESP)})
	LayerTypeUDPLite                      = gopacket.RegisterLayerType(52, gopacket.LayerTypeMetadata{Name: "UDPLite", Decoder: gopacket.DecodeFunc(decodeUDPLite)})
	LayerTypeFDDI                         = gopacket.RegisterLayerType(53, gopacket.LayerTypeMetadata{Name: "FDDI", Decoder: gopacket.DecodeFunc(decodeFDDI)})
	LayerTypeLoopback                     = gopacket.RegisterLayerType(54, gopacket.LayerTypeMetadata{Name: "Loopback", Decoder: gopacket.DecodeFunc(decodeLoopback)})
	LayerTypeEAP                          = gopacket.RegisterLayerType(55, gopacket.LayerTypeMetadata{Name: "EAP", Decoder: gopacket.DecodeFunc(decodeEAP)})
	LayerTypeEAPOL                        = gopacket.RegisterLayerType(56, gopacket.LayerTypeMetadata{Name: "EAPOL", Decoder: gopacket.DecodeFunc(decodeEAPOL)})
	LayerTypeICMPv6                       = gopacket.RegisterLayerType(57, gopacket.LayerTypeMetadata{Name: "ICMPv6", Decoder: gopacket.DecodeFunc(decodeICMPv6)})
	LayerTypeLinkLayerDiscovery           = gopacket.RegisterLayerType(58, gopacket.LayerTypeMetadata{Name: "LinkLayerDiscovery", Decoder: gopacket.DecodeFunc(decodeLinkLayerDiscovery)})
	LayerTypeCiscoDiscoveryInfo           = gopacket.RegisterLayerType(59, gopacket.LayerTypeMetadata{Name: "CiscoDiscoveryInfo", Decoder: gopacket.DecodeFunc(decodeCiscoDiscoveryInfo)})
	LayerTypeLinkLayerDiscoveryInfo       = gopacket.RegisterLayerType(60, gopacket.LayerTypeMetadata{Name: "LinkLayerDiscoveryInfo", Decoder: nil})
	LayerTypeNortelDiscovery              = gopacket.RegisterLayerType(61, gopacket.LayerTypeMetadata{Name: "NortelDiscovery", Decoder: gopacket.DecodeFunc(decodeNortelDiscovery)})
	LayerTypeIGMP                         = gopacket.RegisterLayerType(62, gopacket.LayerTypeMetadata{Name: "IGMP", Decoder: gopacket.DecodeFunc(decodeIGMP)})
	LayerTypePFLog                        = gopacket.RegisterLayerType(63, gopacket.LayerTypeMetadata{Name: "PFLog", Decoder: gopacket.DecodeFunc(decodePFLog)})
	LayerTypeRadioTap                     = gopacket.RegisterLayerType(64, gopacket.LayerTypeMetadata{Name: "RadioTap", Decoder: gopacket.DecodeFunc(decodeRadioTap)})
	LayerTypeDot11                        = gopacket.RegisterLayerType(65, gopacket.LayerTypeMetadata{Name: "Dot11", Decoder: gopacket.DecodeFunc(decodeDot11)})
	LayerTypeDot11Ctrl                    = gopacket.RegisterLayerType(66, gopacket.LayerTypeMetadata{Name: "Dot11Ctrl", Decoder: gopacket.DecodeFunc(decodeDot11Ctrl)})
	LayerTypeDot11Data                    = gopacket.RegisterLayerType(67, gopacket.LayerTypeMetadata{Name: "Dot11Data", Decoder: gopacket.DecodeFunc(decodeDot11Data)})
	LayerTypeDot11DataCFAck               = gopacket.RegisterLayerType(68, gopacket.LayerTypeMetadata{Name: "Dot11DataCFAck", Decoder: gopacket.DecodeFunc(decodeDot11DataCFAck)})
	LayerTypeDot11DataCFPoll              = gopacket.RegisterLayerType(69, gopacket.LayerTypeMetadata{Name: "Dot11DataCFPoll", Decoder: gopacket.DecodeFunc(decodeDot11DataCFPoll)})
	LayerTypeDot11DataCFAckPoll           = gopacket.RegisterLayerType(70, gopacket.LayerTypeMetadata{Name: "Dot11DataCFAckPoll", Decoder: gopacket.DecodeFunc(decodeDot11DataCFAckPoll)})
	LayerTypeDot11DataNull                = gopacket.RegisterLayerType(71, gopacket.LayerTypeMetadata{Name: "Dot11DataNull", Decoder: gopacket.DecodeFunc(decodeDot11DataNull)})
	LayerTypeDot11DataCFAckNoData         = gopacket.RegisterLayerType(72, gopacket.LayerTypeMetadata{Name: "Dot11DataCFAck", Decoder: gopacket.DecodeFunc(decodeDot11DataCFAck)})
	LayerTypeDot11DataCFPollNoData        = gopacket.RegisterLayerType(73, gopacket.LayerTypeMetadata{Name: "Dot11DataCFPoll", Decoder: gopacket.DecodeFunc(decodeDot11DataCFPoll)})
	LayerTypeDot11DataCFAckPollNoData     = gopacket.RegisterLayerType(74, gopacket.LayerTypeMetadata{Name: "Dot11DataCFAckPoll", Decoder: gopacket.DecodeFunc(decodeDot11DataCFAckPoll)})
	LayerTypeDot11DataQOSData             = gopacket.RegisterLayerType(75, gopacket.LayerTypeMetadata{Name: "Dot11DataQOSData", Decoder: gopacket.DecodeFunc(decodeDot11DataQOSData)})
	LayerTypeDot11DataQOSDataCFAck        = gopacket.RegisterLayerType(76, gopacket.LayerTypeMetadata{Name: "Dot11DataQOSDataCFAck", Decoder: gopacket.DecodeFunc(decodeDot11DataQOSDataCFAck)})
	LayerTypeDot11DataQOSDataCFPoll       = gopacket.RegisterLayerType(77, gopacket.LayerTypeMetadata{Name: "Dot11DataQOSDataCFPoll", Decoder: gopacket.DecodeFunc(decodeDot11DataQOSDataCFPoll)})
	LayerTypeDot11DataQOSDataCFAckPoll    = gopacket.RegisterLayerType(78, gopacket.LayerTypeMetadata{Name: "Dot11DataQOSDataCFAckPoll", Decoder: gopacket.DecodeFunc(decodeDot11DataQOSDataCFAckPoll)})
	LayerTypeDot11DataQOSNull             = gopacket.RegisterLayerType(79, gopacket.LayerTypeMetadata{Name: "Dot11DataQOSNull", Decoder: gopacket.DecodeFunc(decodeDot11DataQOSNull)})
	LayerTypeDot11DataQOSCFPollNoData     = gopacket.RegisterLayerType(80, gopacket.LayerTypeMetadata{Name: "Dot11DataQOSCFPoll", Decoder: gopacket.DecodeFunc(decodeDot11DataQOSCFPollNoData)})
	LayerTypeDot11DataQOSCFAckPollNoData  = gopacket.RegisterLayerType(81, gopacket.LayerTypeMetadata{Name: "Dot11DataQOSCFAckPoll", Decoder: gopacket.DecodeFunc(decodeDot11DataQOSCFAckPollNoData)})
	LayerTypeDot11InformationElement      = gopacket.RegisterLayerType(82, gopacket.LayerTypeMetadata{Name: "Dot11InformationElement", Decoder: gopacket.DecodeFunc(decodeDot11InformationElement)})
	LayerTypeDot11CtrlCTS                 = gopacket.RegisterLayerType(83, gopacket.LayerTypeMetadata{Name: "Dot11CtrlCTS", Decoder: gopacket.DecodeFunc(decodeDot11CtrlCTS)})
	LayerTypeDot11CtrlRTS                 = gopacket.RegisterLayerType(84, gopacket.LayerTypeMetadata{Name: "Dot11CtrlRTS", Decoder: gopacket.DecodeFunc(decodeDot11CtrlRTS)})
	LayerTypeDot11CtrlBlockAckReq         = gopacket.RegisterLayerType(85, gopacket.LayerTypeMetadata{Name: "Dot11CtrlBlockAckReq", Decoder: gopacket.DecodeFunc(decodeDot11CtrlBlockAckReq)})
	LayerTypeDot11CtrlBlockAck            = gopacket.RegisterLayerType(86, gopacket.LayerTypeMetadata{Name: "Dot11CtrlBlockAck", Decoder: gopacket.DecodeFunc(decodeDot11CtrlBlockAck)})
	LayerTypeDot11CtrlPowersavePoll       = gopacket.RegisterLayerType(87, gopacket.LayerTypeMetadata{Name: "Dot11CtrlPowersavePoll", Decoder: gopacket.DecodeFunc(decodeDot11CtrlPowersavePoll)})
	LayerTypeDot11CtrlAck                 = gopacket.RegisterLayerType(88, gopacket.LayerTypeMetadata{Name: "Dot11CtrlAck", Decoder: gopacket.DecodeFunc(decodeDot11CtrlAck)})
	LayerTypeDot11CtrlCFEnd               = gopacket.RegisterLayerType(89, gopacket.LayerTypeMetadata{Name: "Dot11CtrlCFEnd", Decoder: gopacket.DecodeFunc(decodeDot11CtrlCFEnd)})
	LayerTypeDot11CtrlCFEndAck            = gopacket.RegisterLayerType(90, gopacket.LayerTypeMetadata{Name: "Dot11CtrlCFEndAck", Decoder: gopacket.DecodeFunc(decodeDot11CtrlCFEndAck)})
	LayerTypeDot11MgmtAssociationReq      = gopacket.RegisterLayerType(91, gopacket.LayerTypeMetadata{Name: "Dot11MgmtAssociationReq", Decoder: gopacket.DecodeFunc(decodeDot11MgmtAssociationReq)})
	LayerTypeDot11MgmtAssociationResp     = gopacket.RegisterLayerType(92, gopacket.LayerTypeMetadata{Name: "Dot11MgmtAssociationResp", Decoder: gopacket.DecodeFunc(decodeDot11MgmtAssociationResp)})
	LayerTypeDot11MgmtReassociationReq    = gopacket.RegisterLayerType(93, gopacket.LayerTypeMetadata{Name: "Dot11MgmtReassociationReq", Decoder: gopacket.DecodeFunc(decodeDot11MgmtReassociationReq)})
	LayerTypeDot11MgmtReassociationResp   = gopacket.RegisterLayerType(94, gopacket.LayerTypeMetadata{Name: "Dot11MgmtReassociationResp", Decoder: gopacket.DecodeFunc(decodeDot11MgmtReassociationResp)})
	LayerTypeDot11MgmtProbeReq            = gopacket.RegisterLayerType(95, gopacket.LayerTypeMetadata{Name: "Dot11MgmtProbeReq", Decoder: gopacket.DecodeFunc(decodeDot11MgmtProbeReq)})
	LayerTypeDot11MgmtProbeResp           = gopacket.RegisterLayerType(96, gopacket.LayerTypeMetadata{Name: "Dot11MgmtProbeResp", Decoder: gopacket.DecodeFunc(decodeDot11MgmtProbeResp)})
	LayerTypeDot11MgmtMeasurementPilot    = gopacket.RegisterLayerType(97, gopacket.LayerTypeMetadata{Name: "Dot11MgmtMeasurementPilot", Decoder: gopacket.DecodeFunc(decodeDot11MgmtMeasurementPilot)})
	LayerTypeDot11MgmtBeacon              = gopacket.RegisterLayerType(98, gopacket.LayerTypeMetadata{Name: "Dot11MgmtBeacon", Decoder: gopacket.DecodeFunc(decodeDot11MgmtBeacon)})
	LayerTypeDot11MgmtATIM                = gopacket.RegisterLayerType(99, gopacket.LayerTypeMetadata{Name: "Dot11MgmtATIM", Decoder: gopacket.DecodeFunc(decodeDot11MgmtATIM)})
	LayerTypeDot11MgmtDisassociation      = gopacket.RegisterLayerType(100, gopacket.LayerTypeMetadata{Name: "Dot11MgmtDisassociation", Decoder: gopacket.DecodeFunc(decodeDot11MgmtDisassociation)})
	LayerTypeDot11MgmtAuthentication      = gopacket.RegisterLayerType(101, gopacket.LayerTypeMetadata{Name: "Dot11MgmtAuthentication", Decoder: gopacket.DecodeFunc(decodeDot11MgmtAuthentication)})
	LayerTypeDot11MgmtDeauthentication    = gopacket.RegisterLayerType(102, gopacket.LayerTypeMetadata{Name: "Dot11MgmtDeauthentication", Decoder: gopacket.DecodeFunc(decodeDot11MgmtDeauthentication)})
	LayerTypeDot11MgmtAction              = gopacket.RegisterLayerType(103, gopacket.LayerTypeMetadata{Name: "Dot11MgmtAction", Decoder: gopacket.DecodeFunc(decodeDot11MgmtAction)})
	LayerTypeDot11MgmtActionNoAck         = gopacket.RegisterLayerType(104, gopacket.LayerTypeMetadata{Name: "Dot11MgmtActionNoAck", Decoder: gopacket.DecodeFunc(decodeDot11MgmtActionNoAck)})
	LayerTypeDot11MgmtArubaWLAN           = gopacket.RegisterLayerType(105, gopacket.LayerTypeMetadata{Name: "Dot11MgmtArubaWLAN", Decoder: gopacket.DecodeFunc(decodeDot11MgmtArubaWLAN)})
	LayerTypeDot11WEP                     = gopacket.RegisterLayerType(106, gopacket.LayerTypeMetadata{Name: "Dot11WEP", Decoder: gopacket.DecodeFunc(decodeDot11WEP)})
	LayerTypeDNS                          = gopacket.RegisterLayerType(107, gopacket.LayerTypeMetadata{Name: "DNS", Decoder: gopacket.DecodeFunc(decodeDNS)})
	LayerTypeUSB                          = gopacket.RegisterLayerType(108, gopacket.LayerTypeMetadata{Name: "USB", Decoder: gopacket.DecodeFunc(decodeUSB)})
	LayerTypeUSBRequestBlockSetup         = gopacket.RegisterLayerType(109, gopacket.LayerTypeMetadata{Name: "USBRequestBlockSetup", Decoder: gopacket.DecodeFunc(decodeUSBRequestBlockSetup)})
	LayerTypeUSBControl                   = gopacket.RegisterLayerType(110, gopacket.LayerTypeMetadata{Name: "USBControl", Decoder: gopacket.DecodeFunc(decodeUSBControl)})
	LayerTypeUSBInterrupt                 = gopacket.RegisterLayerType(111, gopacket.LayerTypeMetadata{Name: "USBInterrupt", Decoder: gopacket.DecodeFunc(decodeUSBInterrupt)})
	LayerTypeUSBBulk                      = gopacket.RegisterLayerType(112, gopacket.LayerTypeMetadata{Name: "USBBulk", Decoder: gopacket.DecodeFunc(decodeUSBBulk)})
	LayerTypeLinuxSLL                     = gopacket.RegisterLayerType(113, gopacket.LayerTypeMetadata{Name: "Linux SLL", Decoder: gopacket.DecodeFunc(decodeLinuxSLL)})
	LayerTypeSFlow                        = gopacket.RegisterLayerType(114, gopacket.LayerTypeMetadata{Name: "SFlow", Decoder: gopacket.DecodeFunc(decodeSFlow)})
	LayerTypePrismHeader                  = gopacket.RegisterLayerType(115, gopacket.LayerTypeMetadata{Name: "Prism monitor mode header", Decoder: gopacket.DecodeFunc(decodePrismHeader)})
	LayerTypeVXLAN                        = gopacket.RegisterLayerType(116, gopacket.LayerTypeMetadata{Name: "VXLAN", Decoder: gopacket.DecodeFunc(decodeVXLAN)})
	LayerTypeNTP                          = gopacket.RegisterLayerType(117, gopacket.LayerTypeMetadata{Name: "NTP", Decoder: gopacket.DecodeFunc(decodeNTP)})
	LayerTypeDHCPv4                       = gopacket.RegisterLayerType(118, gopacket.LayerTypeMetadata{Name: "DHCPv4", Decoder: gopacket.DecodeFunc(decodeDHCPv4)})
	LayerTypeVRRP                         = gopacket.RegisterLayerType(119, gopacket.LayerTypeMetadata{Name: "VRRP", Decoder: gopacket.DecodeFunc(decodeVRRP)})
	LayerTypeGeneve                       = gopacket.RegisterLayerType(120, gopacket.LayerTypeMetadata{Name: "Geneve", Decoder: gopacket.DecodeFunc(decodeGeneve)})
	LayerTypeSTP                          = gopacket.RegisterLayerType(121, gopacket.LayerTypeMetadata{Name: "STP", Decoder: gopacket.DecodeFunc(decodeSTP)})
	LayerTypeBFD                          = gopacket.RegisterLayerType(122, gopacket.LayerTypeMetadata{Name: "BFD", Decoder: gopacket.DecodeFunc(decodeBFD)})
	LayerTypeOSPF                         = gopacket.RegisterLayerType(123, gopacket.LayerTypeMetadata{Name: "OSPF", Decoder: gopacket.DecodeFunc(decodeOSPF)})
	LayerTypeICMPv6RouterSolicitation     = gopacket.RegisterLayerType(124, gopacket.LayerTypeMetadata{Name: "ICMPv6RouterSolicitation", Decoder: gopacket.DecodeFunc(decodeICMPv6RouterSolicitation)})
	LayerTypeICMPv6RouterAdvertisement    = gopacket.RegisterLayerType(125, gopacket.LayerTypeMetadata{Name: "ICMPv6RouterAdvertisement", Decoder: gopacket.DecodeFunc(decodeICMPv6RouterAdvertisement)})
	LayerTypeICMPv6NeighborSolicitation   = gopacket.RegisterLayerType(126, gopacket.LayerTypeMetadata{Name: "ICMPv6NeighborSolicitation", Decoder: gopacket.DecodeFunc(decodeICMPv6NeighborSolicitation)})
	LayerTypeICMPv6NeighborAdvertisement  = gopacket.RegisterLayerType(127, gopacket.LayerTypeMetadata{Name: "ICMPv6NeighborAdvertisement", Decoder: gopacket.DecodeFunc(decodeICMPv6NeighborAdvertisement)})
	LayerTypeICMPv6Redirect               = gopacket.RegisterLayerType(128, gopacket.LayerTypeMetadata{Name: "ICMPv6Redirect", Decoder: gopacket.DecodeFunc(decodeICMPv6Redirect)})
	LayerTypeGTPv1U                       = gopacket.RegisterLayerType(129, gopacket.LayerTypeMetadata{Name: "GTPv1U", Decoder: gopacket.DecodeFunc(decodeGTPv1u)})
	LayerTypeEAPOLKey                     = gopacket.RegisterLayerType(130, gopacket.LayerTypeMetadata{Name: "EAPOLKey", Decoder: gopacket.DecodeFunc(decodeEAPOLKey)})
	LayerTypeLCM                          = gopacket.RegisterLayerType(131, gopacket.LayerTypeMetadata{Name: "LCM", Decoder: gopacket.DecodeFunc(decodeLCM)})
	LayerTypeICMPv6Echo                   = gopacket.RegisterLayerType(132, gopacket.LayerTypeMetadata{Name: "ICMPv6Echo", Decoder: gopacket.DecodeFunc(decodeICMPv6Echo)})
	LayerTypeSIP                          = gopacket.RegisterLayerType(133, gopacket.LayerTypeMetadata{Name: "SIP", Decoder: gopacket.DecodeFunc(decodeSIP)})
	LayerTypeDHCPv6                       = gopacket.RegisterLayerType(134, gopacket.LayerTypeMetadata{Name: "DHCPv6", Decoder: gopacket.DecodeFunc(decodeDHCPv6)})
	LayerTypeMLDv1MulticastListenerReport = gopacket.RegisterLayerType(135, gopacket.LayerTypeMetadata{Name: "MLDv1MulticastListenerReport", Decoder: gopacket.DecodeFunc(decodeMLDv1MulticastListenerReport)})
	LayerTypeMLDv1MulticastListenerDone   = gopacket.RegisterLayerType(136, gopacket.LayerTypeMetadata{Name: "MLDv1MulticastListenerDone", Decoder: gopacket.DecodeFunc(decodeMLDv1MulticastListenerDone)})
	LayerTypeMLDv1MulticastListenerQuery  = gopacket.RegisterLayerType(137, gopacket.LayerTypeMetadata{Name: "MLDv1MulticastListenerQuery", Decoder: gopacket.DecodeFunc(decodeMLDv1MulticastListenerQuery)})
	LayerTypeMLDv2MulticastListenerReport = gopacket.RegisterLayerType(138, gopacket.LayerTypeMetadata{Name: "MLDv2MulticastListenerReport", Decoder: gopacket.DecodeFunc(decodeMLDv2MulticastListenerReport)})
	LayerTypeMLDv2MulticastListenerQuery  = gopacket.RegisterLayerType(139, gopacket.LayerTypeMetadata{Name: "MLDv2MulticastListenerQuery", Decoder: gopacket.DecodeFunc(decodeMLDv2MulticastListenerQuery)})
	LayerTypeTLS                          = gopacket.RegisterLayerType(140, gopacket.LayerTypeMetadata{Name: "TLS", Decoder: gopacket.DecodeFunc(decodeTLS)})
	LayerTypeModbusTCP                    = gopacket.RegisterLayerType(141, gopacket.LayerTypeMetadata{Name: "ModbusTCP", Decoder: gopacket.DecodeFunc(decodeModbusTCP)})
	LayerTypeRMCP                         = gopacket.RegisterLayerType(142, gopacket.LayerTypeMetadata{Name: "RMCP", Decoder: gopacket.DecodeFunc(decodeRMCP)})
	LayerTypeASF                          = gopacket.RegisterLayerType(143, gopacket.LayerTypeMetadata{Name: "ASF", Decoder: gopacket.DecodeFunc(decodeASF)})
	LayerTypeASFPresencePong              = gopacket.RegisterLayerType(144, gopacket.LayerTypeMetadata{Name: "ASFPresencePong", Decoder: gopacket.DecodeFunc(decodeASFPresencePong)})
	LayerTypeERSPANII                     = gopacket.RegisterLayerType(145, gopacket.LayerTypeMetadata{Name: "ERSPAN Type II", Decoder: gopacket.DecodeFunc(decodeERSPANII)})
	LayerTypeRADIUS                       = gopacket.RegisterLayerType(146, gopacket.LayerTypeMetadata{Name: "RADIUS", Decoder: gopacket.DecodeFunc(decodeRADIUS)})
)

var (
	// LayerClassIPNetwork contains TCP/IP network layer types.
	LayerClassIPNetwork = gopacket.NewLayerClass([]gopacket.LayerType{
		LayerTypeIPv4,
		LayerTypeIPv6,
	})
	// LayerClassIPTransport contains TCP/IP transport layer types.
	LayerClassIPTransport = gopacket.NewLayerClass([]gopacket.LayerType{
		LayerTypeTCP,
		LayerTypeUDP,
		LayerTypeSCTP,
	})
	// LayerClassIPControl contains TCP/IP control protocols.
	LayerClassIPControl = gopacket.NewLayerClass([]gopacket.LayerType{
		LayerTypeICMPv4,
		LayerTypeICMPv6,
	})
	// LayerClassSCTPChunk contains SCTP chunk types (not the top-level SCTP
	// layer).
	LayerClassSCTPChunk = gopacket.NewLayerClass([]gopacket.LayerType{
		LayerTypeSCTPUnknownChunkType,
		LayerTypeSCTPData,
		LayerTypeSCTPInit,
		LayerTypeSCTPSack,
		LayerTypeSCTPHeartbeat,
		LayerTypeSCTPError,
		LayerTypeSCTPShutdown,
		LayerTypeSCTPShutdownAck,
		LayerTypeSCTPCookieEcho,
		LayerTypeSCTPEmptyLayer,
		LayerTypeSCTPInitAck,
		LayerTypeSCTPHeartbeatAck,
		LayerTypeSCTPAbort,
		LayerTypeSCTPShutdownComplete,
		LayerTypeSCTPCookieAck,
	})
	// LayerClassIPv6Extension contains IPv6 extension headers.
	LayerClassIPv6Extension = gopacket.NewLayerClass([]gopacket.LayerType{
		LayerTypeIPv6HopByHop,
		LayerTypeIPv6Routing,
		LayerTypeIPv6Fragment,
		LayerTypeIPv6Destination,
	})
	LayerClassIPSec = gopacket.NewLayerClass([]gopacket.LayerType{
		LayerTypeIPSecAH,
		LayerTypeIPSecESP,
	})
	// LayerClassICMPv6NDP contains ICMPv6 neighbor discovery protocol
	// messages.
	LayerClassICMPv6NDP = gopacket.NewLayerClass([]gopacket.LayerType{
		LayerTypeICMPv6RouterSolicitation,
		LayerTypeICMPv6RouterAdvertisement,
		LayerTypeICMPv6NeighborSolicitation,
		LayerTypeICMPv6NeighborAdvertisement,
		LayerTypeICMPv6Redirect,
	})
	// LayerClassMLDv1 contains multicast listener discovery protocol
	LayerClassMLDv1 = gopacket.NewLayerClass([]gopacket.LayerType{
		LayerTypeMLDv1MulticastListenerQuery,
		LayerTypeMLDv1MulticastListenerReport,
		LayerTypeMLDv1MulticastListenerDone,
	})
	// LayerClassMLDv2 contains multicast listener discovery protocol v2
	LayerClassMLDv2 = gopacket.NewLayerClass([]gopacket.LayerType{
		LayerTypeMLDv1MulticastListenerReport,
		LayerTypeMLDv1MulticastListenerDone,
		LayerTypeMLDv2MulticastListenerReport,
		LayerTypeMLDv1MulticastListenerQuery,
		LayerTypeMLDv2MulticastListenerQuery,
	})
)
