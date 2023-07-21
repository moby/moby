// Copyright 2012 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

// Enum types courtesy of...
// http://anonsvn.wireshark.org/wireshark/trunk/epan/dissectors/packet-ndp.c

package layers

import (
	"fmt"
	"github.com/google/gopacket"
	"net"
)

type NDPChassisType uint8

// Nortel Chassis Types
const (
	NDPChassisother                                       NDPChassisType = 1
	NDPChassis3000                                        NDPChassisType = 2
	NDPChassis3030                                        NDPChassisType = 3
	NDPChassis2310                                        NDPChassisType = 4
	NDPChassis2810                                        NDPChassisType = 5
	NDPChassis2912                                        NDPChassisType = 6
	NDPChassis2914                                        NDPChassisType = 7
	NDPChassis271x                                        NDPChassisType = 8
	NDPChassis2813                                        NDPChassisType = 9
	NDPChassis2814                                        NDPChassisType = 10
	NDPChassis2915                                        NDPChassisType = 11
	NDPChassis5000                                        NDPChassisType = 12
	NDPChassis2813SA                                      NDPChassisType = 13
	NDPChassis2814SA                                      NDPChassisType = 14
	NDPChassis810M                                        NDPChassisType = 15
	NDPChassisEthercell                                   NDPChassisType = 16
	NDPChassis5005                                        NDPChassisType = 17
	NDPChassisAlcatelEWC                                  NDPChassisType = 18
	NDPChassis2715SA                                      NDPChassisType = 20
	NDPChassis2486                                        NDPChassisType = 21
	NDPChassis28000series                                 NDPChassisType = 22
	NDPChassis23000series                                 NDPChassisType = 23
	NDPChassis5DN00xseries                                NDPChassisType = 24
	NDPChassisBayStackEthernet                            NDPChassisType = 25
	NDPChassis23100series                                 NDPChassisType = 26
	NDPChassis100BaseTHub                                 NDPChassisType = 27
	NDPChassis3000FastEthernet                            NDPChassisType = 28
	NDPChassisOrionSwitch                                 NDPChassisType = 29
	NDPChassisDDS                                         NDPChassisType = 31
	NDPChassisCentillion6slot                             NDPChassisType = 32
	NDPChassisCentillion12slot                            NDPChassisType = 33
	NDPChassisCentillion1slot                             NDPChassisType = 34
	NDPChassisBayStack301                                 NDPChassisType = 35
	NDPChassisBayStackTokenRingHub                        NDPChassisType = 36
	NDPChassisFVCMultimediaSwitch                         NDPChassisType = 37
	NDPChassisSwitchNode                                  NDPChassisType = 38
	NDPChassisBayStack302Switch                           NDPChassisType = 39
	NDPChassisBayStack350Switch                           NDPChassisType = 40
	NDPChassisBayStack150EthernetHub                      NDPChassisType = 41
	NDPChassisCentillion50NSwitch                         NDPChassisType = 42
	NDPChassisCentillion50TSwitch                         NDPChassisType = 43
	NDPChassisBayStack303304Switches                      NDPChassisType = 44
	NDPChassisBayStack200EthernetHub                      NDPChassisType = 45
	NDPChassisBayStack25010100EthernetHub                 NDPChassisType = 46
	NDPChassisBayStack450101001000Switches                NDPChassisType = 48
	NDPChassisBayStack41010100Switches                    NDPChassisType = 49
	NDPChassisPassport1200L3Switch                        NDPChassisType = 50
	NDPChassisPassport1250L3Switch                        NDPChassisType = 51
	NDPChassisPassport1100L3Switch                        NDPChassisType = 52
	NDPChassisPassport1150L3Switch                        NDPChassisType = 53
	NDPChassisPassport1050L3Switch                        NDPChassisType = 54
	NDPChassisPassport1051L3Switch                        NDPChassisType = 55
	NDPChassisPassport8610L3Switch                        NDPChassisType = 56
	NDPChassisPassport8606L3Switch                        NDPChassisType = 57
	NDPChassisPassport8010                                NDPChassisType = 58
	NDPChassisPassport8006                                NDPChassisType = 59
	NDPChassisBayStack670wirelessaccesspoint              NDPChassisType = 60
	NDPChassisPassport740                                 NDPChassisType = 61
	NDPChassisPassport750                                 NDPChassisType = 62
	NDPChassisPassport790                                 NDPChassisType = 63
	NDPChassisBusinessPolicySwitch200010100Switches       NDPChassisType = 64
	NDPChassisPassport8110L2Switch                        NDPChassisType = 65
	NDPChassisPassport8106L2Switch                        NDPChassisType = 66
	NDPChassisBayStack3580GigSwitch                       NDPChassisType = 67
	NDPChassisBayStack10PowerSupplyUnit                   NDPChassisType = 68
	NDPChassisBayStack42010100Switch                      NDPChassisType = 69
	NDPChassisOPTeraMetro1200EthernetServiceModule        NDPChassisType = 70
	NDPChassisOPTera8010co                                NDPChassisType = 71
	NDPChassisOPTera8610coL3Switch                        NDPChassisType = 72
	NDPChassisOPTera8110coL2Switch                        NDPChassisType = 73
	NDPChassisOPTera8003                                  NDPChassisType = 74
	NDPChassisOPTera8603L3Switch                          NDPChassisType = 75
	NDPChassisOPTera8103L2Switch                          NDPChassisType = 76
	NDPChassisBayStack380101001000Switch                  NDPChassisType = 77
	NDPChassisEthernetSwitch47048T                        NDPChassisType = 78
	NDPChassisOPTeraMetro1450EthernetServiceModule        NDPChassisType = 79
	NDPChassisOPTeraMetro1400EthernetServiceModule        NDPChassisType = 80
	NDPChassisAlteonSwitchFamily                          NDPChassisType = 81
	NDPChassisEthernetSwitch46024TPWR                     NDPChassisType = 82
	NDPChassisOPTeraMetro8010OPML2Switch                  NDPChassisType = 83
	NDPChassisOPTeraMetro8010coOPML2Switch                NDPChassisType = 84
	NDPChassisOPTeraMetro8006OPML2Switch                  NDPChassisType = 85
	NDPChassisOPTeraMetro8003OPML2Switch                  NDPChassisType = 86
	NDPChassisAlteon180e                                  NDPChassisType = 87
	NDPChassisAlteonAD3                                   NDPChassisType = 88
	NDPChassisAlteon184                                   NDPChassisType = 89
	NDPChassisAlteonAD4                                   NDPChassisType = 90
	NDPChassisPassport1424L3Switch                        NDPChassisType = 91
	NDPChassisPassport1648L3Switch                        NDPChassisType = 92
	NDPChassisPassport1612L3Switch                        NDPChassisType = 93
	NDPChassisPassport1624L3Switch                        NDPChassisType = 94
	NDPChassisBayStack38024FFiber1000Switch               NDPChassisType = 95
	NDPChassisEthernetRoutingSwitch551024T                NDPChassisType = 96
	NDPChassisEthernetRoutingSwitch551048T                NDPChassisType = 97
	NDPChassisEthernetSwitch47024T                        NDPChassisType = 98
	NDPChassisNortelNetworksWirelessLANAccessPoint2220    NDPChassisType = 99
	NDPChassisPassportRBS2402L3Switch                     NDPChassisType = 100
	NDPChassisAlteonApplicationSwitch2424                 NDPChassisType = 101
	NDPChassisAlteonApplicationSwitch2224                 NDPChassisType = 102
	NDPChassisAlteonApplicationSwitch2208                 NDPChassisType = 103
	NDPChassisAlteonApplicationSwitch2216                 NDPChassisType = 104
	NDPChassisAlteonApplicationSwitch3408                 NDPChassisType = 105
	NDPChassisAlteonApplicationSwitch3416                 NDPChassisType = 106
	NDPChassisNortelNetworksWirelessLANSecuritySwitch2250 NDPChassisType = 107
	NDPChassisEthernetSwitch42548T                        NDPChassisType = 108
	NDPChassisEthernetSwitch42524T                        NDPChassisType = 109
	NDPChassisNortelNetworksWirelessLANAccessPoint2221    NDPChassisType = 110
	NDPChassisNortelMetroEthernetServiceUnit24TSPFswitch  NDPChassisType = 111
	NDPChassisNortelMetroEthernetServiceUnit24TLXDCswitch NDPChassisType = 112
	NDPChassisPassport830010slotchassis                   NDPChassisType = 113
	NDPChassisPassport83006slotchassis                    NDPChassisType = 114
	NDPChassisEthernetRoutingSwitch552024TPWR             NDPChassisType = 115
	NDPChassisEthernetRoutingSwitch552048TPWR             NDPChassisType = 116
	NDPChassisNortelNetworksVPNGateway3050                NDPChassisType = 117
	NDPChassisAlteonSSL31010100                           NDPChassisType = 118
	NDPChassisAlteonSSL31010100Fiber                      NDPChassisType = 119
	NDPChassisAlteonSSL31010100FIPS                       NDPChassisType = 120
	NDPChassisAlteonSSL410101001000                       NDPChassisType = 121
	NDPChassisAlteonSSL410101001000Fiber                  NDPChassisType = 122
	NDPChassisAlteonApplicationSwitch2424SSL              NDPChassisType = 123
	NDPChassisEthernetSwitch32524T                        NDPChassisType = 124
	NDPChassisEthernetSwitch32524G                        NDPChassisType = 125
	NDPChassisNortelNetworksWirelessLANAccessPoint2225    NDPChassisType = 126
	NDPChassisNortelNetworksWirelessLANSecuritySwitch2270 NDPChassisType = 127
	NDPChassis24portEthernetSwitch47024TPWR               NDPChassisType = 128
	NDPChassis48portEthernetSwitch47048TPWR               NDPChassisType = 129
	NDPChassisEthernetRoutingSwitch553024TFD              NDPChassisType = 130
	NDPChassisEthernetSwitch351024T                       NDPChassisType = 131
	NDPChassisNortelMetroEthernetServiceUnit12GACL3Switch NDPChassisType = 132
	NDPChassisNortelMetroEthernetServiceUnit12GDCL3Switch NDPChassisType = 133
	NDPChassisNortelSecureAccessSwitch                    NDPChassisType = 134
	NDPChassisNortelNetworksVPNGateway3070                NDPChassisType = 135
	NDPChassisOPTeraMetro3500                             NDPChassisType = 136
	NDPChassisSMBBES101024T                               NDPChassisType = 137
	NDPChassisSMBBES101048T                               NDPChassisType = 138
	NDPChassisSMBBES102024TPWR                            NDPChassisType = 139
	NDPChassisSMBBES102048TPWR                            NDPChassisType = 140
	NDPChassisSMBBES201024T                               NDPChassisType = 141
	NDPChassisSMBBES201048T                               NDPChassisType = 142
	NDPChassisSMBBES202024TPWR                            NDPChassisType = 143
	NDPChassisSMBBES202048TPWR                            NDPChassisType = 144
	NDPChassisSMBBES11024T                                NDPChassisType = 145
	NDPChassisSMBBES11048T                                NDPChassisType = 146
	NDPChassisSMBBES12024TPWR                             NDPChassisType = 147
	NDPChassisSMBBES12048TPWR                             NDPChassisType = 148
	NDPChassisSMBBES21024T                                NDPChassisType = 149
	NDPChassisSMBBES21048T                                NDPChassisType = 150
	NDPChassisSMBBES22024TPWR                             NDPChassisType = 151
	NDPChassisSMBBES22048TPWR                             NDPChassisType = 152
	NDPChassisOME6500                                     NDPChassisType = 153
	NDPChassisEthernetRoutingSwitch4548GT                 NDPChassisType = 154
	NDPChassisEthernetRoutingSwitch4548GTPWR              NDPChassisType = 155
	NDPChassisEthernetRoutingSwitch4550T                  NDPChassisType = 156
	NDPChassisEthernetRoutingSwitch4550TPWR               NDPChassisType = 157
	NDPChassisEthernetRoutingSwitch4526FX                 NDPChassisType = 158
	NDPChassisEthernetRoutingSwitch250026T                NDPChassisType = 159
	NDPChassisEthernetRoutingSwitch250026TPWR             NDPChassisType = 160
	NDPChassisEthernetRoutingSwitch250050T                NDPChassisType = 161
	NDPChassisEthernetRoutingSwitch250050TPWR             NDPChassisType = 162
)

type NDPBackplaneType uint8

// Nortel Backplane Types
const (
	NDPBackplaneOther                                       NDPBackplaneType = 1
	NDPBackplaneEthernet                                    NDPBackplaneType = 2
	NDPBackplaneEthernetTokenring                           NDPBackplaneType = 3
	NDPBackplaneEthernetFDDI                                NDPBackplaneType = 4
	NDPBackplaneEthernetTokenringFDDI                       NDPBackplaneType = 5
	NDPBackplaneEthernetTokenringRedundantPower             NDPBackplaneType = 6
	NDPBackplaneEthernetTokenringFDDIRedundantPower         NDPBackplaneType = 7
	NDPBackplaneTokenRing                                   NDPBackplaneType = 8
	NDPBackplaneEthernetTokenringFastEthernet               NDPBackplaneType = 9
	NDPBackplaneEthernetFastEthernet                        NDPBackplaneType = 10
	NDPBackplaneEthernetTokenringFastEthernetRedundantPower NDPBackplaneType = 11
	NDPBackplaneEthernetFastEthernetGigabitEthernet         NDPBackplaneType = 12
)

type NDPState uint8

// Device State
const (
	NDPStateTopology  NDPState = 1
	NDPStateHeartbeat NDPState = 2
	NDPStateNew       NDPState = 3
)

// NortelDiscovery is a packet layer containing the Nortel Discovery Protocol.
type NortelDiscovery struct {
	BaseLayer
	IPAddress net.IP
	SegmentID []byte
	Chassis   NDPChassisType
	Backplane NDPBackplaneType
	State     NDPState
	NumLinks  uint8
}

// LayerType returns gopacket.LayerTypeNortelDiscovery.
func (c *NortelDiscovery) LayerType() gopacket.LayerType {
	return LayerTypeNortelDiscovery
}

func decodeNortelDiscovery(data []byte, p gopacket.PacketBuilder) error {
	c := &NortelDiscovery{}
	if len(data) < 11 {
		return fmt.Errorf("Invalid NortelDiscovery packet length %d", len(data))
	}
	c.IPAddress = data[0:4]
	c.SegmentID = data[4:7]
	c.Chassis = NDPChassisType(data[7])
	c.Backplane = NDPBackplaneType(data[8])
	c.State = NDPState(data[9])
	c.NumLinks = uint8(data[10])
	p.AddLayer(c)
	return nil
}

func (t NDPChassisType) String() (s string) {
	switch t {
	case NDPChassisother:
		s = "other"
	case NDPChassis3000:
		s = "3000"
	case NDPChassis3030:
		s = "3030"
	case NDPChassis2310:
		s = "2310"
	case NDPChassis2810:
		s = "2810"
	case NDPChassis2912:
		s = "2912"
	case NDPChassis2914:
		s = "2914"
	case NDPChassis271x:
		s = "271x"
	case NDPChassis2813:
		s = "2813"
	case NDPChassis2814:
		s = "2814"
	case NDPChassis2915:
		s = "2915"
	case NDPChassis5000:
		s = "5000"
	case NDPChassis2813SA:
		s = "2813SA"
	case NDPChassis2814SA:
		s = "2814SA"
	case NDPChassis810M:
		s = "810M"
	case NDPChassisEthercell:
		s = "Ethercell"
	case NDPChassis5005:
		s = "5005"
	case NDPChassisAlcatelEWC:
		s = "Alcatel Ethernet workgroup conc."
	case NDPChassis2715SA:
		s = "2715SA"
	case NDPChassis2486:
		s = "2486"
	case NDPChassis28000series:
		s = "28000 series"
	case NDPChassis23000series:
		s = "23000 series"
	case NDPChassis5DN00xseries:
		s = "5DN00x series"
	case NDPChassisBayStackEthernet:
		s = "BayStack Ethernet"
	case NDPChassis23100series:
		s = "23100 series"
	case NDPChassis100BaseTHub:
		s = "100Base-T Hub"
	case NDPChassis3000FastEthernet:
		s = "3000 Fast Ethernet"
	case NDPChassisOrionSwitch:
		s = "Orion switch"
	case NDPChassisDDS:
		s = "DDS"
	case NDPChassisCentillion6slot:
		s = "Centillion (6 slot)"
	case NDPChassisCentillion12slot:
		s = "Centillion (12 slot)"
	case NDPChassisCentillion1slot:
		s = "Centillion (1 slot)"
	case NDPChassisBayStack301:
		s = "BayStack 301"
	case NDPChassisBayStackTokenRingHub:
		s = "BayStack TokenRing Hub"
	case NDPChassisFVCMultimediaSwitch:
		s = "FVC Multimedia Switch"
	case NDPChassisSwitchNode:
		s = "Switch Node"
	case NDPChassisBayStack302Switch:
		s = "BayStack 302 Switch"
	case NDPChassisBayStack350Switch:
		s = "BayStack 350 Switch"
	case NDPChassisBayStack150EthernetHub:
		s = "BayStack 150 Ethernet Hub"
	case NDPChassisCentillion50NSwitch:
		s = "Centillion 50N switch"
	case NDPChassisCentillion50TSwitch:
		s = "Centillion 50T switch"
	case NDPChassisBayStack303304Switches:
		s = "BayStack 303 and 304 Switches"
	case NDPChassisBayStack200EthernetHub:
		s = "BayStack 200 Ethernet Hub"
	case NDPChassisBayStack25010100EthernetHub:
		s = "BayStack 250 10/100 Ethernet Hub"
	case NDPChassisBayStack450101001000Switches:
		s = "BayStack 450 10/100/1000 Switches"
	case NDPChassisBayStack41010100Switches:
		s = "BayStack 410 10/100 Switches"
	case NDPChassisPassport1200L3Switch:
		s = "Passport 1200 L3 Switch"
	case NDPChassisPassport1250L3Switch:
		s = "Passport 1250 L3 Switch"
	case NDPChassisPassport1100L3Switch:
		s = "Passport 1100 L3 Switch"
	case NDPChassisPassport1150L3Switch:
		s = "Passport 1150 L3 Switch"
	case NDPChassisPassport1050L3Switch:
		s = "Passport 1050 L3 Switch"
	case NDPChassisPassport1051L3Switch:
		s = "Passport 1051 L3 Switch"
	case NDPChassisPassport8610L3Switch:
		s = "Passport 8610 L3 Switch"
	case NDPChassisPassport8606L3Switch:
		s = "Passport 8606 L3 Switch"
	case NDPChassisPassport8010:
		s = "Passport 8010"
	case NDPChassisPassport8006:
		s = "Passport 8006"
	case NDPChassisBayStack670wirelessaccesspoint:
		s = "BayStack 670 wireless access point"
	case NDPChassisPassport740:
		s = "Passport 740"
	case NDPChassisPassport750:
		s = "Passport 750"
	case NDPChassisPassport790:
		s = "Passport 790"
	case NDPChassisBusinessPolicySwitch200010100Switches:
		s = "Business Policy Switch 2000 10/100 Switches"
	case NDPChassisPassport8110L2Switch:
		s = "Passport 8110 L2 Switch"
	case NDPChassisPassport8106L2Switch:
		s = "Passport 8106 L2 Switch"
	case NDPChassisBayStack3580GigSwitch:
		s = "BayStack 3580 Gig Switch"
	case NDPChassisBayStack10PowerSupplyUnit:
		s = "BayStack 10 Power Supply Unit"
	case NDPChassisBayStack42010100Switch:
		s = "BayStack 420 10/100 Switch"
	case NDPChassisOPTeraMetro1200EthernetServiceModule:
		s = "OPTera Metro 1200 Ethernet Service Module"
	case NDPChassisOPTera8010co:
		s = "OPTera 8010co"
	case NDPChassisOPTera8610coL3Switch:
		s = "OPTera 8610co L3 switch"
	case NDPChassisOPTera8110coL2Switch:
		s = "OPTera 8110co L2 switch"
	case NDPChassisOPTera8003:
		s = "OPTera 8003"
	case NDPChassisOPTera8603L3Switch:
		s = "OPTera 8603 L3 switch"
	case NDPChassisOPTera8103L2Switch:
		s = "OPTera 8103 L2 switch"
	case NDPChassisBayStack380101001000Switch:
		s = "BayStack 380 10/100/1000 Switch"
	case NDPChassisEthernetSwitch47048T:
		s = "Ethernet Switch 470-48T"
	case NDPChassisOPTeraMetro1450EthernetServiceModule:
		s = "OPTera Metro 1450 Ethernet Service Module"
	case NDPChassisOPTeraMetro1400EthernetServiceModule:
		s = "OPTera Metro 1400 Ethernet Service Module"
	case NDPChassisAlteonSwitchFamily:
		s = "Alteon Switch Family"
	case NDPChassisEthernetSwitch46024TPWR:
		s = "Ethernet Switch 460-24T-PWR"
	case NDPChassisOPTeraMetro8010OPML2Switch:
		s = "OPTera Metro 8010 OPM L2 Switch"
	case NDPChassisOPTeraMetro8010coOPML2Switch:
		s = "OPTera Metro 8010co OPM L2 Switch"
	case NDPChassisOPTeraMetro8006OPML2Switch:
		s = "OPTera Metro 8006 OPM L2 Switch"
	case NDPChassisOPTeraMetro8003OPML2Switch:
		s = "OPTera Metro 8003 OPM L2 Switch"
	case NDPChassisAlteon180e:
		s = "Alteon 180e"
	case NDPChassisAlteonAD3:
		s = "Alteon AD3"
	case NDPChassisAlteon184:
		s = "Alteon 184"
	case NDPChassisAlteonAD4:
		s = "Alteon AD4"
	case NDPChassisPassport1424L3Switch:
		s = "Passport 1424 L3 switch"
	case NDPChassisPassport1648L3Switch:
		s = "Passport 1648 L3 switch"
	case NDPChassisPassport1612L3Switch:
		s = "Passport 1612 L3 switch"
	case NDPChassisPassport1624L3Switch:
		s = "Passport 1624 L3 switch"
	case NDPChassisBayStack38024FFiber1000Switch:
		s = "BayStack 380-24F Fiber 1000 Switch"
	case NDPChassisEthernetRoutingSwitch551024T:
		s = "Ethernet Routing Switch 5510-24T"
	case NDPChassisEthernetRoutingSwitch551048T:
		s = "Ethernet Routing Switch 5510-48T"
	case NDPChassisEthernetSwitch47024T:
		s = "Ethernet Switch 470-24T"
	case NDPChassisNortelNetworksWirelessLANAccessPoint2220:
		s = "Nortel Networks Wireless LAN Access Point 2220"
	case NDPChassisPassportRBS2402L3Switch:
		s = "Passport RBS 2402 L3 switch"
	case NDPChassisAlteonApplicationSwitch2424:
		s = "Alteon Application Switch 2424"
	case NDPChassisAlteonApplicationSwitch2224:
		s = "Alteon Application Switch 2224"
	case NDPChassisAlteonApplicationSwitch2208:
		s = "Alteon Application Switch 2208"
	case NDPChassisAlteonApplicationSwitch2216:
		s = "Alteon Application Switch 2216"
	case NDPChassisAlteonApplicationSwitch3408:
		s = "Alteon Application Switch 3408"
	case NDPChassisAlteonApplicationSwitch3416:
		s = "Alteon Application Switch 3416"
	case NDPChassisNortelNetworksWirelessLANSecuritySwitch2250:
		s = "Nortel Networks Wireless LAN SecuritySwitch 2250"
	case NDPChassisEthernetSwitch42548T:
		s = "Ethernet Switch 425-48T"
	case NDPChassisEthernetSwitch42524T:
		s = "Ethernet Switch 425-24T"
	case NDPChassisNortelNetworksWirelessLANAccessPoint2221:
		s = "Nortel Networks Wireless LAN Access Point 2221"
	case NDPChassisNortelMetroEthernetServiceUnit24TSPFswitch:
		s = "Nortel Metro Ethernet Service Unit 24-T SPF switch"
	case NDPChassisNortelMetroEthernetServiceUnit24TLXDCswitch:
		s = " Nortel Metro Ethernet Service Unit 24-T LX DC switch"
	case NDPChassisPassport830010slotchassis:
		s = "Passport 8300 10-slot chassis"
	case NDPChassisPassport83006slotchassis:
		s = "Passport 8300 6-slot chassis"
	case NDPChassisEthernetRoutingSwitch552024TPWR:
		s = "Ethernet Routing Switch 5520-24T-PWR"
	case NDPChassisEthernetRoutingSwitch552048TPWR:
		s = "Ethernet Routing Switch 5520-48T-PWR"
	case NDPChassisNortelNetworksVPNGateway3050:
		s = "Nortel Networks VPN Gateway 3050"
	case NDPChassisAlteonSSL31010100:
		s = "Alteon SSL 310 10/100"
	case NDPChassisAlteonSSL31010100Fiber:
		s = "Alteon SSL 310 10/100 Fiber"
	case NDPChassisAlteonSSL31010100FIPS:
		s = "Alteon SSL 310 10/100 FIPS"
	case NDPChassisAlteonSSL410101001000:
		s = "Alteon SSL 410 10/100/1000"
	case NDPChassisAlteonSSL410101001000Fiber:
		s = "Alteon SSL 410 10/100/1000 Fiber"
	case NDPChassisAlteonApplicationSwitch2424SSL:
		s = "Alteon Application Switch 2424-SSL"
	case NDPChassisEthernetSwitch32524T:
		s = "Ethernet Switch 325-24T"
	case NDPChassisEthernetSwitch32524G:
		s = "Ethernet Switch 325-24G"
	case NDPChassisNortelNetworksWirelessLANAccessPoint2225:
		s = "Nortel Networks Wireless LAN Access Point 2225"
	case NDPChassisNortelNetworksWirelessLANSecuritySwitch2270:
		s = "Nortel Networks Wireless LAN SecuritySwitch 2270"
	case NDPChassis24portEthernetSwitch47024TPWR:
		s = "24-port Ethernet Switch 470-24T-PWR"
	case NDPChassis48portEthernetSwitch47048TPWR:
		s = "48-port Ethernet Switch 470-48T-PWR"
	case NDPChassisEthernetRoutingSwitch553024TFD:
		s = "Ethernet Routing Switch 5530-24TFD"
	case NDPChassisEthernetSwitch351024T:
		s = "Ethernet Switch 3510-24T"
	case NDPChassisNortelMetroEthernetServiceUnit12GACL3Switch:
		s = "Nortel Metro Ethernet Service Unit 12G AC L3 switch"
	case NDPChassisNortelMetroEthernetServiceUnit12GDCL3Switch:
		s = "Nortel Metro Ethernet Service Unit 12G DC L3 switch"
	case NDPChassisNortelSecureAccessSwitch:
		s = "Nortel Secure Access Switch"
	case NDPChassisNortelNetworksVPNGateway3070:
		s = "Nortel Networks VPN Gateway 3070"
	case NDPChassisOPTeraMetro3500:
		s = "OPTera Metro 3500"
	case NDPChassisSMBBES101024T:
		s = "SMB BES 1010 24T"
	case NDPChassisSMBBES101048T:
		s = "SMB BES 1010 48T"
	case NDPChassisSMBBES102024TPWR:
		s = "SMB BES 1020 24T PWR"
	case NDPChassisSMBBES102048TPWR:
		s = "SMB BES 1020 48T PWR"
	case NDPChassisSMBBES201024T:
		s = "SMB BES 2010 24T"
	case NDPChassisSMBBES201048T:
		s = "SMB BES 2010 48T"
	case NDPChassisSMBBES202024TPWR:
		s = "SMB BES 2020 24T PWR"
	case NDPChassisSMBBES202048TPWR:
		s = "SMB BES 2020 48T PWR"
	case NDPChassisSMBBES11024T:
		s = "SMB BES 110 24T"
	case NDPChassisSMBBES11048T:
		s = "SMB BES 110 48T"
	case NDPChassisSMBBES12024TPWR:
		s = "SMB BES 120 24T PWR"
	case NDPChassisSMBBES12048TPWR:
		s = "SMB BES 120 48T PWR"
	case NDPChassisSMBBES21024T:
		s = "SMB BES 210 24T"
	case NDPChassisSMBBES21048T:
		s = "SMB BES 210 48T"
	case NDPChassisSMBBES22024TPWR:
		s = "SMB BES 220 24T PWR"
	case NDPChassisSMBBES22048TPWR:
		s = "SMB BES 220 48T PWR"
	case NDPChassisOME6500:
		s = "OME 6500"
	case NDPChassisEthernetRoutingSwitch4548GT:
		s = "Ethernet Routing Switch 4548GT"
	case NDPChassisEthernetRoutingSwitch4548GTPWR:
		s = "Ethernet Routing Switch 4548GT-PWR"
	case NDPChassisEthernetRoutingSwitch4550T:
		s = "Ethernet Routing Switch 4550T"
	case NDPChassisEthernetRoutingSwitch4550TPWR:
		s = "Ethernet Routing Switch 4550T-PWR"
	case NDPChassisEthernetRoutingSwitch4526FX:
		s = "Ethernet Routing Switch 4526FX"
	case NDPChassisEthernetRoutingSwitch250026T:
		s = "Ethernet Routing Switch 2500-26T"
	case NDPChassisEthernetRoutingSwitch250026TPWR:
		s = "Ethernet Routing Switch 2500-26T-PWR"
	case NDPChassisEthernetRoutingSwitch250050T:
		s = "Ethernet Routing Switch 2500-50T"
	case NDPChassisEthernetRoutingSwitch250050TPWR:
		s = "Ethernet Routing Switch 2500-50T-PWR"
	default:
		s = "Unknown"
	}
	return
}

func (t NDPBackplaneType) String() (s string) {
	switch t {
	case NDPBackplaneOther:
		s = "Other"
	case NDPBackplaneEthernet:
		s = "Ethernet"
	case NDPBackplaneEthernetTokenring:
		s = "Ethernet and Tokenring"
	case NDPBackplaneEthernetFDDI:
		s = "Ethernet and FDDI"
	case NDPBackplaneEthernetTokenringFDDI:
		s = "Ethernet, Tokenring and FDDI"
	case NDPBackplaneEthernetTokenringRedundantPower:
		s = "Ethernet and Tokenring with redundant power"
	case NDPBackplaneEthernetTokenringFDDIRedundantPower:
		s = "Ethernet, Tokenring, FDDI with redundant power"
	case NDPBackplaneTokenRing:
		s = "Token Ring"
	case NDPBackplaneEthernetTokenringFastEthernet:
		s = "Ethernet, Tokenring and Fast Ethernet"
	case NDPBackplaneEthernetFastEthernet:
		s = "Ethernet and Fast Ethernet"
	case NDPBackplaneEthernetTokenringFastEthernetRedundantPower:
		s = "Ethernet, Tokenring, Fast Ethernet with redundant power"
	case NDPBackplaneEthernetFastEthernetGigabitEthernet:
		s = "Ethernet, Fast Ethernet and Gigabit Ethernet"
	default:
		s = "Unknown"
	}
	return
}

func (t NDPState) String() (s string) {
	switch t {
	case NDPStateTopology:
		s = "Topology Change"
	case NDPStateHeartbeat:
		s = "Heartbeat"
	case NDPStateNew:
		s = "New"
	default:
		s = "Unknown"
	}
	return
}
