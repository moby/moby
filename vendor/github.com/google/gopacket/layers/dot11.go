// Copyright 2014 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

// See http://standards.ieee.org/findstds/standard/802.11-2012.html for info on
// all of the layers in this file.

package layers

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"net"

	"github.com/google/gopacket"
)

// Dot11Flags contains the set of 8 flags in the IEEE 802.11 frame control
// header, all in one place.
type Dot11Flags uint8

const (
	Dot11FlagsToDS Dot11Flags = 1 << iota
	Dot11FlagsFromDS
	Dot11FlagsMF
	Dot11FlagsRetry
	Dot11FlagsPowerManagement
	Dot11FlagsMD
	Dot11FlagsWEP
	Dot11FlagsOrder
)

func (d Dot11Flags) ToDS() bool {
	return d&Dot11FlagsToDS != 0
}
func (d Dot11Flags) FromDS() bool {
	return d&Dot11FlagsFromDS != 0
}
func (d Dot11Flags) MF() bool {
	return d&Dot11FlagsMF != 0
}
func (d Dot11Flags) Retry() bool {
	return d&Dot11FlagsRetry != 0
}
func (d Dot11Flags) PowerManagement() bool {
	return d&Dot11FlagsPowerManagement != 0
}
func (d Dot11Flags) MD() bool {
	return d&Dot11FlagsMD != 0
}
func (d Dot11Flags) WEP() bool {
	return d&Dot11FlagsWEP != 0
}
func (d Dot11Flags) Order() bool {
	return d&Dot11FlagsOrder != 0
}

// String provides a human readable string for Dot11Flags.
// This string is possibly subject to change over time; if you're storing this
// persistently, you should probably store the Dot11Flags value, not its string.
func (a Dot11Flags) String() string {
	var out bytes.Buffer
	if a.ToDS() {
		out.WriteString("TO-DS,")
	}
	if a.FromDS() {
		out.WriteString("FROM-DS,")
	}
	if a.MF() {
		out.WriteString("MF,")
	}
	if a.Retry() {
		out.WriteString("Retry,")
	}
	if a.PowerManagement() {
		out.WriteString("PowerManagement,")
	}
	if a.MD() {
		out.WriteString("MD,")
	}
	if a.WEP() {
		out.WriteString("WEP,")
	}
	if a.Order() {
		out.WriteString("Order,")
	}

	if length := out.Len(); length > 0 {
		return string(out.Bytes()[:length-1]) // strip final comma
	}
	return ""
}

type Dot11Reason uint16

// TODO: Verify these reasons, and append more reasons if necessary.

const (
	Dot11ReasonReserved          Dot11Reason = 1
	Dot11ReasonUnspecified       Dot11Reason = 2
	Dot11ReasonAuthExpired       Dot11Reason = 3
	Dot11ReasonDeauthStLeaving   Dot11Reason = 4
	Dot11ReasonInactivity        Dot11Reason = 5
	Dot11ReasonApFull            Dot11Reason = 6
	Dot11ReasonClass2FromNonAuth Dot11Reason = 7
	Dot11ReasonClass3FromNonAss  Dot11Reason = 8
	Dot11ReasonDisasStLeaving    Dot11Reason = 9
	Dot11ReasonStNotAuth         Dot11Reason = 10
)

// String provides a human readable string for Dot11Reason.
// This string is possibly subject to change over time; if you're storing this
// persistently, you should probably store the Dot11Reason value, not its string.
func (a Dot11Reason) String() string {
	switch a {
	case Dot11ReasonReserved:
		return "Reserved"
	case Dot11ReasonUnspecified:
		return "Unspecified"
	case Dot11ReasonAuthExpired:
		return "Auth. expired"
	case Dot11ReasonDeauthStLeaving:
		return "Deauth. st. leaving"
	case Dot11ReasonInactivity:
		return "Inactivity"
	case Dot11ReasonApFull:
		return "Ap. full"
	case Dot11ReasonClass2FromNonAuth:
		return "Class2 from non auth."
	case Dot11ReasonClass3FromNonAss:
		return "Class3 from non ass."
	case Dot11ReasonDisasStLeaving:
		return "Disass st. leaving"
	case Dot11ReasonStNotAuth:
		return "St. not auth."
	default:
		return "Unknown reason"
	}
}

type Dot11Status uint16

const (
	Dot11StatusSuccess                      Dot11Status = 0
	Dot11StatusFailure                      Dot11Status = 1  // Unspecified failure
	Dot11StatusCannotSupportAllCapabilities Dot11Status = 10 // Cannot support all requested capabilities in the Capability Information field
	Dot11StatusInabilityExistsAssociation   Dot11Status = 11 // Reassociation denied due to inability to confirm that association exists
	Dot11StatusAssociationDenied            Dot11Status = 12 // Association denied due to reason outside the scope of this standard
	Dot11StatusAlgorithmUnsupported         Dot11Status = 13 // Responding station does not support the specified authentication algorithm
	Dot11StatusOufOfExpectedSequence        Dot11Status = 14 // Received an Authentication frame with authentication transaction sequence number out of expected sequence
	Dot11StatusChallengeFailure             Dot11Status = 15 // Authentication rejected because of challenge failure
	Dot11StatusTimeout                      Dot11Status = 16 // Authentication rejected due to timeout waiting for next frame in sequence
	Dot11StatusAPUnableToHandle             Dot11Status = 17 // Association denied because AP is unable to handle additional associated stations
	Dot11StatusRateUnsupported              Dot11Status = 18 // Association denied due to requesting station not supporting all of the data rates in the BSSBasicRateSet parameter
)

// String provides a human readable string for Dot11Status.
// This string is possibly subject to change over time; if you're storing this
// persistently, you should probably store the Dot11Status value, not its string.
func (a Dot11Status) String() string {
	switch a {
	case Dot11StatusSuccess:
		return "success"
	case Dot11StatusFailure:
		return "failure"
	case Dot11StatusCannotSupportAllCapabilities:
		return "cannot-support-all-capabilities"
	case Dot11StatusInabilityExistsAssociation:
		return "inability-exists-association"
	case Dot11StatusAssociationDenied:
		return "association-denied"
	case Dot11StatusAlgorithmUnsupported:
		return "algorithm-unsupported"
	case Dot11StatusOufOfExpectedSequence:
		return "out-of-expected-sequence"
	case Dot11StatusChallengeFailure:
		return "challenge-failure"
	case Dot11StatusTimeout:
		return "timeout"
	case Dot11StatusAPUnableToHandle:
		return "ap-unable-to-handle"
	case Dot11StatusRateUnsupported:
		return "rate-unsupported"
	default:
		return "unknown status"
	}
}

type Dot11AckPolicy uint8

const (
	Dot11AckPolicyNormal     Dot11AckPolicy = 0
	Dot11AckPolicyNone       Dot11AckPolicy = 1
	Dot11AckPolicyNoExplicit Dot11AckPolicy = 2
	Dot11AckPolicyBlock      Dot11AckPolicy = 3
)

// String provides a human readable string for Dot11AckPolicy.
// This string is possibly subject to change over time; if you're storing this
// persistently, you should probably store the Dot11AckPolicy value, not its string.
func (a Dot11AckPolicy) String() string {
	switch a {
	case Dot11AckPolicyNormal:
		return "normal-ack"
	case Dot11AckPolicyNone:
		return "no-ack"
	case Dot11AckPolicyNoExplicit:
		return "no-explicit-ack"
	case Dot11AckPolicyBlock:
		return "block-ack"
	default:
		return "unknown-ack-policy"
	}
}

type Dot11Algorithm uint16

const (
	Dot11AlgorithmOpen      Dot11Algorithm = 0
	Dot11AlgorithmSharedKey Dot11Algorithm = 1
)

// String provides a human readable string for Dot11Algorithm.
// This string is possibly subject to change over time; if you're storing this
// persistently, you should probably store the Dot11Algorithm value, not its string.
func (a Dot11Algorithm) String() string {
	switch a {
	case Dot11AlgorithmOpen:
		return "open"
	case Dot11AlgorithmSharedKey:
		return "shared-key"
	default:
		return "unknown-algorithm"
	}
}

type Dot11InformationElementID uint8

const (
	Dot11InformationElementIDSSID                      Dot11InformationElementID = 0
	Dot11InformationElementIDRates                     Dot11InformationElementID = 1
	Dot11InformationElementIDFHSet                     Dot11InformationElementID = 2
	Dot11InformationElementIDDSSet                     Dot11InformationElementID = 3
	Dot11InformationElementIDCFSet                     Dot11InformationElementID = 4
	Dot11InformationElementIDTIM                       Dot11InformationElementID = 5
	Dot11InformationElementIDIBSSSet                   Dot11InformationElementID = 6
	Dot11InformationElementIDCountryInfo               Dot11InformationElementID = 7
	Dot11InformationElementIDHoppingPatternParam       Dot11InformationElementID = 8
	Dot11InformationElementIDHoppingPatternTable       Dot11InformationElementID = 9
	Dot11InformationElementIDRequest                   Dot11InformationElementID = 10
	Dot11InformationElementIDQBSSLoadElem              Dot11InformationElementID = 11
	Dot11InformationElementIDEDCAParamSet              Dot11InformationElementID = 12
	Dot11InformationElementIDTrafficSpec               Dot11InformationElementID = 13
	Dot11InformationElementIDTrafficClass              Dot11InformationElementID = 14
	Dot11InformationElementIDSchedule                  Dot11InformationElementID = 15
	Dot11InformationElementIDChallenge                 Dot11InformationElementID = 16
	Dot11InformationElementIDPowerConst                Dot11InformationElementID = 32
	Dot11InformationElementIDPowerCapability           Dot11InformationElementID = 33
	Dot11InformationElementIDTPCRequest                Dot11InformationElementID = 34
	Dot11InformationElementIDTPCReport                 Dot11InformationElementID = 35
	Dot11InformationElementIDSupportedChannels         Dot11InformationElementID = 36
	Dot11InformationElementIDSwitchChannelAnnounce     Dot11InformationElementID = 37
	Dot11InformationElementIDMeasureRequest            Dot11InformationElementID = 38
	Dot11InformationElementIDMeasureReport             Dot11InformationElementID = 39
	Dot11InformationElementIDQuiet                     Dot11InformationElementID = 40
	Dot11InformationElementIDIBSSDFS                   Dot11InformationElementID = 41
	Dot11InformationElementIDERPInfo                   Dot11InformationElementID = 42
	Dot11InformationElementIDTSDelay                   Dot11InformationElementID = 43
	Dot11InformationElementIDTCLASProcessing           Dot11InformationElementID = 44
	Dot11InformationElementIDHTCapabilities            Dot11InformationElementID = 45
	Dot11InformationElementIDQOSCapability             Dot11InformationElementID = 46
	Dot11InformationElementIDERPInfo2                  Dot11InformationElementID = 47
	Dot11InformationElementIDRSNInfo                   Dot11InformationElementID = 48
	Dot11InformationElementIDESRates                   Dot11InformationElementID = 50
	Dot11InformationElementIDAPChannelReport           Dot11InformationElementID = 51
	Dot11InformationElementIDNeighborReport            Dot11InformationElementID = 52
	Dot11InformationElementIDRCPI                      Dot11InformationElementID = 53
	Dot11InformationElementIDMobilityDomain            Dot11InformationElementID = 54
	Dot11InformationElementIDFastBSSTrans              Dot11InformationElementID = 55
	Dot11InformationElementIDTimeoutInt                Dot11InformationElementID = 56
	Dot11InformationElementIDRICData                   Dot11InformationElementID = 57
	Dot11InformationElementIDDSERegisteredLoc          Dot11InformationElementID = 58
	Dot11InformationElementIDSuppOperatingClass        Dot11InformationElementID = 59
	Dot11InformationElementIDExtChanSwitchAnnounce     Dot11InformationElementID = 60
	Dot11InformationElementIDHTInfo                    Dot11InformationElementID = 61
	Dot11InformationElementIDSecChanOffset             Dot11InformationElementID = 62
	Dot11InformationElementIDBSSAverageAccessDelay     Dot11InformationElementID = 63
	Dot11InformationElementIDAntenna                   Dot11InformationElementID = 64
	Dot11InformationElementIDRSNI                      Dot11InformationElementID = 65
	Dot11InformationElementIDMeasurePilotTrans         Dot11InformationElementID = 66
	Dot11InformationElementIDBSSAvailAdmCapacity       Dot11InformationElementID = 67
	Dot11InformationElementIDBSSACAccDelayWAPIParam    Dot11InformationElementID = 68
	Dot11InformationElementIDTimeAdvertisement         Dot11InformationElementID = 69
	Dot11InformationElementIDRMEnabledCapabilities     Dot11InformationElementID = 70
	Dot11InformationElementIDMultipleBSSID             Dot11InformationElementID = 71
	Dot11InformationElementID2040BSSCoExist            Dot11InformationElementID = 72
	Dot11InformationElementID2040BSSIntChanReport      Dot11InformationElementID = 73
	Dot11InformationElementIDOverlapBSSScanParam       Dot11InformationElementID = 74
	Dot11InformationElementIDRICDescriptor             Dot11InformationElementID = 75
	Dot11InformationElementIDManagementMIC             Dot11InformationElementID = 76
	Dot11InformationElementIDEventRequest              Dot11InformationElementID = 78
	Dot11InformationElementIDEventReport               Dot11InformationElementID = 79
	Dot11InformationElementIDDiagnosticRequest         Dot11InformationElementID = 80
	Dot11InformationElementIDDiagnosticReport          Dot11InformationElementID = 81
	Dot11InformationElementIDLocationParam             Dot11InformationElementID = 82
	Dot11InformationElementIDNonTransBSSIDCapability   Dot11InformationElementID = 83
	Dot11InformationElementIDSSIDList                  Dot11InformationElementID = 84
	Dot11InformationElementIDMultipleBSSIDIndex        Dot11InformationElementID = 85
	Dot11InformationElementIDFMSDescriptor             Dot11InformationElementID = 86
	Dot11InformationElementIDFMSRequest                Dot11InformationElementID = 87
	Dot11InformationElementIDFMSResponse               Dot11InformationElementID = 88
	Dot11InformationElementIDQOSTrafficCapability      Dot11InformationElementID = 89
	Dot11InformationElementIDBSSMaxIdlePeriod          Dot11InformationElementID = 90
	Dot11InformationElementIDTFSRequest                Dot11InformationElementID = 91
	Dot11InformationElementIDTFSResponse               Dot11InformationElementID = 92
	Dot11InformationElementIDWNMSleepMode              Dot11InformationElementID = 93
	Dot11InformationElementIDTIMBroadcastRequest       Dot11InformationElementID = 94
	Dot11InformationElementIDTIMBroadcastResponse      Dot11InformationElementID = 95
	Dot11InformationElementIDCollInterferenceReport    Dot11InformationElementID = 96
	Dot11InformationElementIDChannelUsage              Dot11InformationElementID = 97
	Dot11InformationElementIDTimeZone                  Dot11InformationElementID = 98
	Dot11InformationElementIDDMSRequest                Dot11InformationElementID = 99
	Dot11InformationElementIDDMSResponse               Dot11InformationElementID = 100
	Dot11InformationElementIDLinkIdentifier            Dot11InformationElementID = 101
	Dot11InformationElementIDWakeupSchedule            Dot11InformationElementID = 102
	Dot11InformationElementIDChannelSwitchTiming       Dot11InformationElementID = 104
	Dot11InformationElementIDPTIControl                Dot11InformationElementID = 105
	Dot11InformationElementIDPUBufferStatus            Dot11InformationElementID = 106
	Dot11InformationElementIDInterworking              Dot11InformationElementID = 107
	Dot11InformationElementIDAdvertisementProtocol     Dot11InformationElementID = 108
	Dot11InformationElementIDExpBWRequest              Dot11InformationElementID = 109
	Dot11InformationElementIDQOSMapSet                 Dot11InformationElementID = 110
	Dot11InformationElementIDRoamingConsortium         Dot11InformationElementID = 111
	Dot11InformationElementIDEmergencyAlertIdentifier  Dot11InformationElementID = 112
	Dot11InformationElementIDMeshConfiguration         Dot11InformationElementID = 113
	Dot11InformationElementIDMeshID                    Dot11InformationElementID = 114
	Dot11InformationElementIDMeshLinkMetricReport      Dot11InformationElementID = 115
	Dot11InformationElementIDCongestionNotification    Dot11InformationElementID = 116
	Dot11InformationElementIDMeshPeeringManagement     Dot11InformationElementID = 117
	Dot11InformationElementIDMeshChannelSwitchParam    Dot11InformationElementID = 118
	Dot11InformationElementIDMeshAwakeWindows          Dot11InformationElementID = 119
	Dot11InformationElementIDBeaconTiming              Dot11InformationElementID = 120
	Dot11InformationElementIDMCCAOPSetupRequest        Dot11InformationElementID = 121
	Dot11InformationElementIDMCCAOPSetupReply          Dot11InformationElementID = 122
	Dot11InformationElementIDMCCAOPAdvertisement       Dot11InformationElementID = 123
	Dot11InformationElementIDMCCAOPTeardown            Dot11InformationElementID = 124
	Dot11InformationElementIDGateAnnouncement          Dot11InformationElementID = 125
	Dot11InformationElementIDRootAnnouncement          Dot11InformationElementID = 126
	Dot11InformationElementIDExtCapability             Dot11InformationElementID = 127
	Dot11InformationElementIDAgereProprietary          Dot11InformationElementID = 128
	Dot11InformationElementIDPathRequest               Dot11InformationElementID = 130
	Dot11InformationElementIDPathReply                 Dot11InformationElementID = 131
	Dot11InformationElementIDPathError                 Dot11InformationElementID = 132
	Dot11InformationElementIDCiscoCCX1CKIPDeviceName   Dot11InformationElementID = 133
	Dot11InformationElementIDCiscoCCX2                 Dot11InformationElementID = 136
	Dot11InformationElementIDProxyUpdate               Dot11InformationElementID = 137
	Dot11InformationElementIDProxyUpdateConfirmation   Dot11InformationElementID = 138
	Dot11InformationElementIDAuthMeshPerringExch       Dot11InformationElementID = 139
	Dot11InformationElementIDMIC                       Dot11InformationElementID = 140
	Dot11InformationElementIDDestinationURI            Dot11InformationElementID = 141
	Dot11InformationElementIDUAPSDCoexistence          Dot11InformationElementID = 142
	Dot11InformationElementIDWakeupSchedule80211ad     Dot11InformationElementID = 143
	Dot11InformationElementIDExtendedSchedule          Dot11InformationElementID = 144
	Dot11InformationElementIDSTAAvailability           Dot11InformationElementID = 145
	Dot11InformationElementIDDMGTSPEC                  Dot11InformationElementID = 146
	Dot11InformationElementIDNextDMGATI                Dot11InformationElementID = 147
	Dot11InformationElementIDDMSCapabilities           Dot11InformationElementID = 148
	Dot11InformationElementIDCiscoUnknown95            Dot11InformationElementID = 149
	Dot11InformationElementIDVendor2                   Dot11InformationElementID = 150
	Dot11InformationElementIDDMGOperating              Dot11InformationElementID = 151
	Dot11InformationElementIDDMGBSSParamChange         Dot11InformationElementID = 152
	Dot11InformationElementIDDMGBeamRefinement         Dot11InformationElementID = 153
	Dot11InformationElementIDChannelMeasFeedback       Dot11InformationElementID = 154
	Dot11InformationElementIDAwakeWindow               Dot11InformationElementID = 157
	Dot11InformationElementIDMultiBand                 Dot11InformationElementID = 158
	Dot11InformationElementIDADDBAExtension            Dot11InformationElementID = 159
	Dot11InformationElementIDNEXTPCPList               Dot11InformationElementID = 160
	Dot11InformationElementIDPCPHandover               Dot11InformationElementID = 161
	Dot11InformationElementIDDMGLinkMargin             Dot11InformationElementID = 162
	Dot11InformationElementIDSwitchingStream           Dot11InformationElementID = 163
	Dot11InformationElementIDSessionTransmission       Dot11InformationElementID = 164
	Dot11InformationElementIDDynamicTonePairReport     Dot11InformationElementID = 165
	Dot11InformationElementIDClusterReport             Dot11InformationElementID = 166
	Dot11InformationElementIDRelayCapabilities         Dot11InformationElementID = 167
	Dot11InformationElementIDRelayTransferParameter    Dot11InformationElementID = 168
	Dot11InformationElementIDBeamlinkMaintenance       Dot11InformationElementID = 169
	Dot11InformationElementIDMultipleMacSublayers      Dot11InformationElementID = 170
	Dot11InformationElementIDUPID                      Dot11InformationElementID = 171
	Dot11InformationElementIDDMGLinkAdaptionAck        Dot11InformationElementID = 172
	Dot11InformationElementIDSymbolProprietary         Dot11InformationElementID = 173
	Dot11InformationElementIDMCCAOPAdvertOverview      Dot11InformationElementID = 174
	Dot11InformationElementIDQuietPeriodRequest        Dot11InformationElementID = 175
	Dot11InformationElementIDQuietPeriodResponse       Dot11InformationElementID = 177
	Dot11InformationElementIDECPACPolicy               Dot11InformationElementID = 182
	Dot11InformationElementIDClusterTimeOffset         Dot11InformationElementID = 183
	Dot11InformationElementIDAntennaSectorID           Dot11InformationElementID = 190
	Dot11InformationElementIDVHTCapabilities           Dot11InformationElementID = 191
	Dot11InformationElementIDVHTOperation              Dot11InformationElementID = 192
	Dot11InformationElementIDExtendedBSSLoad           Dot11InformationElementID = 193
	Dot11InformationElementIDWideBWChannelSwitch       Dot11InformationElementID = 194
	Dot11InformationElementIDVHTTxPowerEnvelope        Dot11InformationElementID = 195
	Dot11InformationElementIDChannelSwitchWrapper      Dot11InformationElementID = 196
	Dot11InformationElementIDOperatingModeNotification Dot11InformationElementID = 199
	Dot11InformationElementIDUPSIM                     Dot11InformationElementID = 200
	Dot11InformationElementIDReducedNeighborReport     Dot11InformationElementID = 201
	Dot11InformationElementIDTVHTOperation             Dot11InformationElementID = 202
	Dot11InformationElementIDDeviceLocation            Dot11InformationElementID = 204
	Dot11InformationElementIDWhiteSpaceMap             Dot11InformationElementID = 205
	Dot11InformationElementIDFineTuningMeasureParams   Dot11InformationElementID = 206
	Dot11InformationElementIDVendor                    Dot11InformationElementID = 221
)

// String provides a human readable string for Dot11InformationElementID.
// This string is possibly subject to change over time; if you're storing this
// persistently, you should probably store the Dot11InformationElementID value,
// not its string.
func (a Dot11InformationElementID) String() string {
	switch a {
	case Dot11InformationElementIDSSID:
		return "SSID parameter set"
	case Dot11InformationElementIDRates:
		return "Supported Rates"
	case Dot11InformationElementIDFHSet:
		return "FH Parameter set"
	case Dot11InformationElementIDDSSet:
		return "DS Parameter set"
	case Dot11InformationElementIDCFSet:
		return "CF Parameter set"
	case Dot11InformationElementIDTIM:
		return "Traffic Indication Map (TIM)"
	case Dot11InformationElementIDIBSSSet:
		return "IBSS Parameter set"
	case Dot11InformationElementIDCountryInfo:
		return "Country Information"
	case Dot11InformationElementIDHoppingPatternParam:
		return "Hopping Pattern Parameters"
	case Dot11InformationElementIDHoppingPatternTable:
		return "Hopping Pattern Table"
	case Dot11InformationElementIDRequest:
		return "Request"
	case Dot11InformationElementIDQBSSLoadElem:
		return "QBSS Load Element"
	case Dot11InformationElementIDEDCAParamSet:
		return "EDCA Parameter Set"
	case Dot11InformationElementIDTrafficSpec:
		return "Traffic Specification"
	case Dot11InformationElementIDTrafficClass:
		return "Traffic Classification"
	case Dot11InformationElementIDSchedule:
		return "Schedule"
	case Dot11InformationElementIDChallenge:
		return "Challenge text"
	case Dot11InformationElementIDPowerConst:
		return "Power Constraint"
	case Dot11InformationElementIDPowerCapability:
		return "Power Capability"
	case Dot11InformationElementIDTPCRequest:
		return "TPC Request"
	case Dot11InformationElementIDTPCReport:
		return "TPC Report"
	case Dot11InformationElementIDSupportedChannels:
		return "Supported Channels"
	case Dot11InformationElementIDSwitchChannelAnnounce:
		return "Channel Switch Announcement"
	case Dot11InformationElementIDMeasureRequest:
		return "Measurement Request"
	case Dot11InformationElementIDMeasureReport:
		return "Measurement Report"
	case Dot11InformationElementIDQuiet:
		return "Quiet"
	case Dot11InformationElementIDIBSSDFS:
		return "IBSS DFS"
	case Dot11InformationElementIDERPInfo:
		return "ERP Information"
	case Dot11InformationElementIDTSDelay:
		return "TS Delay"
	case Dot11InformationElementIDTCLASProcessing:
		return "TCLAS Processing"
	case Dot11InformationElementIDHTCapabilities:
		return "HT Capabilities (802.11n D1.10)"
	case Dot11InformationElementIDQOSCapability:
		return "QOS Capability"
	case Dot11InformationElementIDERPInfo2:
		return "ERP Information-2"
	case Dot11InformationElementIDRSNInfo:
		return "RSN Information"
	case Dot11InformationElementIDESRates:
		return "Extended Supported Rates"
	case Dot11InformationElementIDAPChannelReport:
		return "AP Channel Report"
	case Dot11InformationElementIDNeighborReport:
		return "Neighbor Report"
	case Dot11InformationElementIDRCPI:
		return "RCPI"
	case Dot11InformationElementIDMobilityDomain:
		return "Mobility Domain"
	case Dot11InformationElementIDFastBSSTrans:
		return "Fast BSS Transition"
	case Dot11InformationElementIDTimeoutInt:
		return "Timeout Interval"
	case Dot11InformationElementIDRICData:
		return "RIC Data"
	case Dot11InformationElementIDDSERegisteredLoc:
		return "DSE Registered Location"
	case Dot11InformationElementIDSuppOperatingClass:
		return "Supported Operating Classes"
	case Dot11InformationElementIDExtChanSwitchAnnounce:
		return "Extended Channel Switch Announcement"
	case Dot11InformationElementIDHTInfo:
		return "HT Information (802.11n D1.10)"
	case Dot11InformationElementIDSecChanOffset:
		return "Secondary Channel Offset (802.11n D1.10)"
	case Dot11InformationElementIDBSSAverageAccessDelay:
		return "BSS Average Access Delay"
	case Dot11InformationElementIDAntenna:
		return "Antenna"
	case Dot11InformationElementIDRSNI:
		return "RSNI"
	case Dot11InformationElementIDMeasurePilotTrans:
		return "Measurement Pilot Transmission"
	case Dot11InformationElementIDBSSAvailAdmCapacity:
		return "BSS Available Admission Capacity"
	case Dot11InformationElementIDBSSACAccDelayWAPIParam:
		return "BSS AC Access Delay/WAPI Parameter Set"
	case Dot11InformationElementIDTimeAdvertisement:
		return "Time Advertisement"
	case Dot11InformationElementIDRMEnabledCapabilities:
		return "RM Enabled Capabilities"
	case Dot11InformationElementIDMultipleBSSID:
		return "Multiple BSSID"
	case Dot11InformationElementID2040BSSCoExist:
		return "20/40 BSS Coexistence"
	case Dot11InformationElementID2040BSSIntChanReport:
		return "20/40 BSS Intolerant Channel Report"
	case Dot11InformationElementIDOverlapBSSScanParam:
		return "Overlapping BSS Scan Parameters"
	case Dot11InformationElementIDRICDescriptor:
		return "RIC Descriptor"
	case Dot11InformationElementIDManagementMIC:
		return "Management MIC"
	case Dot11InformationElementIDEventRequest:
		return "Event Request"
	case Dot11InformationElementIDEventReport:
		return "Event Report"
	case Dot11InformationElementIDDiagnosticRequest:
		return "Diagnostic Request"
	case Dot11InformationElementIDDiagnosticReport:
		return "Diagnostic Report"
	case Dot11InformationElementIDLocationParam:
		return "Location Parameters"
	case Dot11InformationElementIDNonTransBSSIDCapability:
		return "Non Transmitted BSSID Capability"
	case Dot11InformationElementIDSSIDList:
		return "SSID List"
	case Dot11InformationElementIDMultipleBSSIDIndex:
		return "Multiple BSSID Index"
	case Dot11InformationElementIDFMSDescriptor:
		return "FMS Descriptor"
	case Dot11InformationElementIDFMSRequest:
		return "FMS Request"
	case Dot11InformationElementIDFMSResponse:
		return "FMS Response"
	case Dot11InformationElementIDQOSTrafficCapability:
		return "QoS Traffic Capability"
	case Dot11InformationElementIDBSSMaxIdlePeriod:
		return "BSS Max Idle Period"
	case Dot11InformationElementIDTFSRequest:
		return "TFS Request"
	case Dot11InformationElementIDTFSResponse:
		return "TFS Response"
	case Dot11InformationElementIDWNMSleepMode:
		return "WNM-Sleep Mode"
	case Dot11InformationElementIDTIMBroadcastRequest:
		return "TIM Broadcast Request"
	case Dot11InformationElementIDTIMBroadcastResponse:
		return "TIM Broadcast Response"
	case Dot11InformationElementIDCollInterferenceReport:
		return "Collocated Interference Report"
	case Dot11InformationElementIDChannelUsage:
		return "Channel Usage"
	case Dot11InformationElementIDTimeZone:
		return "Time Zone"
	case Dot11InformationElementIDDMSRequest:
		return "DMS Request"
	case Dot11InformationElementIDDMSResponse:
		return "DMS Response"
	case Dot11InformationElementIDLinkIdentifier:
		return "Link Identifier"
	case Dot11InformationElementIDWakeupSchedule:
		return "Wakeup Schedule"
	case Dot11InformationElementIDChannelSwitchTiming:
		return "Channel Switch Timing"
	case Dot11InformationElementIDPTIControl:
		return "PTI Control"
	case Dot11InformationElementIDPUBufferStatus:
		return "PU Buffer Status"
	case Dot11InformationElementIDInterworking:
		return "Interworking"
	case Dot11InformationElementIDAdvertisementProtocol:
		return "Advertisement Protocol"
	case Dot11InformationElementIDExpBWRequest:
		return "Expedited Bandwidth Request"
	case Dot11InformationElementIDQOSMapSet:
		return "QoS Map Set"
	case Dot11InformationElementIDRoamingConsortium:
		return "Roaming Consortium"
	case Dot11InformationElementIDEmergencyAlertIdentifier:
		return "Emergency Alert Identifier"
	case Dot11InformationElementIDMeshConfiguration:
		return "Mesh Configuration"
	case Dot11InformationElementIDMeshID:
		return "Mesh ID"
	case Dot11InformationElementIDMeshLinkMetricReport:
		return "Mesh Link Metric Report"
	case Dot11InformationElementIDCongestionNotification:
		return "Congestion Notification"
	case Dot11InformationElementIDMeshPeeringManagement:
		return "Mesh Peering Management"
	case Dot11InformationElementIDMeshChannelSwitchParam:
		return "Mesh Channel Switch Parameters"
	case Dot11InformationElementIDMeshAwakeWindows:
		return "Mesh Awake Windows"
	case Dot11InformationElementIDBeaconTiming:
		return "Beacon Timing"
	case Dot11InformationElementIDMCCAOPSetupRequest:
		return "MCCAOP Setup Request"
	case Dot11InformationElementIDMCCAOPSetupReply:
		return "MCCAOP SETUP Reply"
	case Dot11InformationElementIDMCCAOPAdvertisement:
		return "MCCAOP Advertisement"
	case Dot11InformationElementIDMCCAOPTeardown:
		return "MCCAOP Teardown"
	case Dot11InformationElementIDGateAnnouncement:
		return "Gate Announcement"
	case Dot11InformationElementIDRootAnnouncement:
		return "Root Announcement"
	case Dot11InformationElementIDExtCapability:
		return "Extended Capabilities"
	case Dot11InformationElementIDAgereProprietary:
		return "Agere Proprietary"
	case Dot11InformationElementIDPathRequest:
		return "Path Request"
	case Dot11InformationElementIDPathReply:
		return "Path Reply"
	case Dot11InformationElementIDPathError:
		return "Path Error"
	case Dot11InformationElementIDCiscoCCX1CKIPDeviceName:
		return "Cisco CCX1 CKIP + Device Name"
	case Dot11InformationElementIDCiscoCCX2:
		return "Cisco CCX2"
	case Dot11InformationElementIDProxyUpdate:
		return "Proxy Update"
	case Dot11InformationElementIDProxyUpdateConfirmation:
		return "Proxy Update Confirmation"
	case Dot11InformationElementIDAuthMeshPerringExch:
		return "Auhenticated Mesh Perring Exchange"
	case Dot11InformationElementIDMIC:
		return "MIC (Message Integrity Code)"
	case Dot11InformationElementIDDestinationURI:
		return "Destination URI"
	case Dot11InformationElementIDUAPSDCoexistence:
		return "U-APSD Coexistence"
	case Dot11InformationElementIDWakeupSchedule80211ad:
		return "Wakeup Schedule 802.11ad"
	case Dot11InformationElementIDExtendedSchedule:
		return "Extended Schedule"
	case Dot11InformationElementIDSTAAvailability:
		return "STA Availability"
	case Dot11InformationElementIDDMGTSPEC:
		return "DMG TSPEC"
	case Dot11InformationElementIDNextDMGATI:
		return "Next DMG ATI"
	case Dot11InformationElementIDDMSCapabilities:
		return "DMG Capabilities"
	case Dot11InformationElementIDCiscoUnknown95:
		return "Cisco Unknown 95"
	case Dot11InformationElementIDVendor2:
		return "Vendor Specific"
	case Dot11InformationElementIDDMGOperating:
		return "DMG Operating"
	case Dot11InformationElementIDDMGBSSParamChange:
		return "DMG BSS Parameter Change"
	case Dot11InformationElementIDDMGBeamRefinement:
		return "DMG Beam Refinement"
	case Dot11InformationElementIDChannelMeasFeedback:
		return "Channel Measurement Feedback"
	case Dot11InformationElementIDAwakeWindow:
		return "Awake Window"
	case Dot11InformationElementIDMultiBand:
		return "Multi Band"
	case Dot11InformationElementIDADDBAExtension:
		return "ADDBA Extension"
	case Dot11InformationElementIDNEXTPCPList:
		return "NEXTPCP List"
	case Dot11InformationElementIDPCPHandover:
		return "PCP Handover"
	case Dot11InformationElementIDDMGLinkMargin:
		return "DMG Link Margin"
	case Dot11InformationElementIDSwitchingStream:
		return "Switching Stream"
	case Dot11InformationElementIDSessionTransmission:
		return "Session Transmission"
	case Dot11InformationElementIDDynamicTonePairReport:
		return "Dynamic Tone Pairing Report"
	case Dot11InformationElementIDClusterReport:
		return "Cluster Report"
	case Dot11InformationElementIDRelayCapabilities:
		return "Relay Capabilities"
	case Dot11InformationElementIDRelayTransferParameter:
		return "Relay Transfer Parameter"
	case Dot11InformationElementIDBeamlinkMaintenance:
		return "Beamlink Maintenance"
	case Dot11InformationElementIDMultipleMacSublayers:
		return "Multiple MAC Sublayers"
	case Dot11InformationElementIDUPID:
		return "U-PID"
	case Dot11InformationElementIDDMGLinkAdaptionAck:
		return "DMG Link Adaption Acknowledgment"
	case Dot11InformationElementIDSymbolProprietary:
		return "Symbol Proprietary"
	case Dot11InformationElementIDMCCAOPAdvertOverview:
		return "MCCAOP Advertisement Overview"
	case Dot11InformationElementIDQuietPeriodRequest:
		return "Quiet Period Request"
	case Dot11InformationElementIDQuietPeriodResponse:
		return "Quiet Period Response"
	case Dot11InformationElementIDECPACPolicy:
		return "ECPAC Policy"
	case Dot11InformationElementIDClusterTimeOffset:
		return "Cluster Time Offset"
	case Dot11InformationElementIDAntennaSectorID:
		return "Antenna Sector ID"
	case Dot11InformationElementIDVHTCapabilities:
		return "VHT Capabilities (IEEE Std 802.11ac/D3.1)"
	case Dot11InformationElementIDVHTOperation:
		return "VHT Operation (IEEE Std 802.11ac/D3.1)"
	case Dot11InformationElementIDExtendedBSSLoad:
		return "Extended BSS Load"
	case Dot11InformationElementIDWideBWChannelSwitch:
		return "Wide Bandwidth Channel Switch"
	case Dot11InformationElementIDVHTTxPowerEnvelope:
		return "VHT Tx Power Envelope (IEEE Std 802.11ac/D5.0)"
	case Dot11InformationElementIDChannelSwitchWrapper:
		return "Channel Switch Wrapper"
	case Dot11InformationElementIDOperatingModeNotification:
		return "Operating Mode Notification"
	case Dot11InformationElementIDUPSIM:
		return "UP SIM"
	case Dot11InformationElementIDReducedNeighborReport:
		return "Reduced Neighbor Report"
	case Dot11InformationElementIDTVHTOperation:
		return "TVHT Op"
	case Dot11InformationElementIDDeviceLocation:
		return "Device Location"
	case Dot11InformationElementIDWhiteSpaceMap:
		return "White Space Map"
	case Dot11InformationElementIDFineTuningMeasureParams:
		return "Fine Tuning Measure Parameters"
	case Dot11InformationElementIDVendor:
		return "Vendor"
	default:
		return "Unknown information element id"
	}
}

// Dot11 provides an IEEE 802.11 base packet header.
// See http://standards.ieee.org/findstds/standard/802.11-2012.html
// for excruciating detail.
type Dot11 struct {
	BaseLayer
	Type           Dot11Type
	Proto          uint8
	Flags          Dot11Flags
	DurationID     uint16
	Address1       net.HardwareAddr
	Address2       net.HardwareAddr
	Address3       net.HardwareAddr
	Address4       net.HardwareAddr
	SequenceNumber uint16
	FragmentNumber uint16
	Checksum       uint32
	QOS            *Dot11QOS
	HTControl      *Dot11HTControl
	DataLayer      gopacket.Layer
}

type Dot11QOS struct {
	TID       uint8 /* Traffic IDentifier */
	EOSP      bool  /* End of service period */
	AckPolicy Dot11AckPolicy
	TXOP      uint8
}

type Dot11HTControl struct {
	ACConstraint bool
	RDGMorePPDU  bool

	VHT *Dot11HTControlVHT
	HT  *Dot11HTControlHT
}

type Dot11HTControlHT struct {
	LinkAdapationControl *Dot11LinkAdapationControl
	CalibrationPosition  uint8
	CalibrationSequence  uint8
	CSISteering          uint8
	NDPAnnouncement      bool
	DEI                  bool
}

type Dot11HTControlVHT struct {
	MRQ            bool
	UnsolicitedMFB bool
	MSI            *uint8
	MFB            Dot11HTControlMFB
	CompressedMSI  *uint8
	STBCIndication bool
	MFSI           *uint8
	GID            *uint8
	CodingType     *Dot11CodingType
	FbTXBeamformed bool
}

type Dot11HTControlMFB struct {
	NumSTS uint8
	VHTMCS uint8
	BW     uint8
	SNR    int8
}

type Dot11LinkAdapationControl struct {
	TRQ  bool
	MRQ  bool
	MSI  uint8
	MFSI uint8
	ASEL *Dot11ASEL
	MFB  *uint8
}

type Dot11ASEL struct {
	Command uint8
	Data    uint8
}

type Dot11CodingType uint8

const (
	Dot11CodingTypeBCC  = 0
	Dot11CodingTypeLDPC = 1
)

func (a Dot11CodingType) String() string {
	switch a {
	case Dot11CodingTypeBCC:
		return "BCC"
	case Dot11CodingTypeLDPC:
		return "LDPC"
	default:
		return "Unknown coding type"
	}
}

func (m *Dot11HTControlMFB) NoFeedBackPresent() bool {
	return m.VHTMCS == 15 && m.NumSTS == 7
}

func decodeDot11(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11{}
	err := d.DecodeFromBytes(data, p)
	if err != nil {
		return err
	}
	p.AddLayer(d)
	if d.DataLayer != nil {
		p.AddLayer(d.DataLayer)
	}
	return p.NextDecoder(d.NextLayerType())
}

func (m *Dot11) LayerType() gopacket.LayerType  { return LayerTypeDot11 }
func (m *Dot11) CanDecode() gopacket.LayerClass { return LayerTypeDot11 }
func (m *Dot11) NextLayerType() gopacket.LayerType {
	if m.DataLayer != nil {
		if m.Flags.WEP() {
			return LayerTypeDot11WEP
		}
		return m.DataLayer.(gopacket.DecodingLayer).NextLayerType()
	}
	return m.Type.LayerType()
}

func createU8(x uint8) *uint8 {
	return &x
}

var dataDecodeMap = map[Dot11Type]func() gopacket.DecodingLayer{
	Dot11TypeData:                   func() gopacket.DecodingLayer { return &Dot11Data{} },
	Dot11TypeDataCFAck:              func() gopacket.DecodingLayer { return &Dot11DataCFAck{} },
	Dot11TypeDataCFPoll:             func() gopacket.DecodingLayer { return &Dot11DataCFPoll{} },
	Dot11TypeDataCFAckPoll:          func() gopacket.DecodingLayer { return &Dot11DataCFAckPoll{} },
	Dot11TypeDataNull:               func() gopacket.DecodingLayer { return &Dot11DataNull{} },
	Dot11TypeDataCFAckNoData:        func() gopacket.DecodingLayer { return &Dot11DataCFAckNoData{} },
	Dot11TypeDataCFPollNoData:       func() gopacket.DecodingLayer { return &Dot11DataCFPollNoData{} },
	Dot11TypeDataCFAckPollNoData:    func() gopacket.DecodingLayer { return &Dot11DataCFAckPollNoData{} },
	Dot11TypeDataQOSData:            func() gopacket.DecodingLayer { return &Dot11DataQOSData{} },
	Dot11TypeDataQOSDataCFAck:       func() gopacket.DecodingLayer { return &Dot11DataQOSDataCFAck{} },
	Dot11TypeDataQOSDataCFPoll:      func() gopacket.DecodingLayer { return &Dot11DataQOSDataCFPoll{} },
	Dot11TypeDataQOSDataCFAckPoll:   func() gopacket.DecodingLayer { return &Dot11DataQOSDataCFAckPoll{} },
	Dot11TypeDataQOSNull:            func() gopacket.DecodingLayer { return &Dot11DataQOSNull{} },
	Dot11TypeDataQOSCFPollNoData:    func() gopacket.DecodingLayer { return &Dot11DataQOSCFPollNoData{} },
	Dot11TypeDataQOSCFAckPollNoData: func() gopacket.DecodingLayer { return &Dot11DataQOSCFAckPollNoData{} },
}

func (m *Dot11) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 10 {
		df.SetTruncated()
		return fmt.Errorf("Dot11 length %v too short, %v required", len(data), 10)
	}
	m.Type = Dot11Type((data[0])&0xFC) >> 2

	m.DataLayer = nil
	m.Proto = uint8(data[0]) & 0x0003
	m.Flags = Dot11Flags(data[1])
	m.DurationID = binary.LittleEndian.Uint16(data[2:4])
	m.Address1 = net.HardwareAddr(data[4:10])

	offset := 10

	mainType := m.Type.MainType()

	switch mainType {
	case Dot11TypeCtrl:
		switch m.Type {
		case Dot11TypeCtrlRTS, Dot11TypeCtrlPowersavePoll, Dot11TypeCtrlCFEnd, Dot11TypeCtrlCFEndAck:
			if len(data) < offset+6 {
				df.SetTruncated()
				return fmt.Errorf("Dot11 length %v too short, %v required", len(data), offset+6)
			}
			m.Address2 = net.HardwareAddr(data[offset : offset+6])
			offset += 6
		}
	case Dot11TypeMgmt, Dot11TypeData:
		if len(data) < offset+14 {
			df.SetTruncated()
			return fmt.Errorf("Dot11 length %v too short, %v required", len(data), offset+14)
		}
		m.Address2 = net.HardwareAddr(data[offset : offset+6])
		offset += 6
		m.Address3 = net.HardwareAddr(data[offset : offset+6])
		offset += 6

		m.SequenceNumber = (binary.LittleEndian.Uint16(data[offset:offset+2]) & 0xFFF0) >> 4
		m.FragmentNumber = (binary.LittleEndian.Uint16(data[offset:offset+2]) & 0x000F)
		offset += 2
	}

	if mainType == Dot11TypeData && m.Flags.FromDS() && m.Flags.ToDS() {
		if len(data) < offset+6 {
			df.SetTruncated()
			return fmt.Errorf("Dot11 length %v too short, %v required", len(data), offset+6)
		}
		m.Address4 = net.HardwareAddr(data[offset : offset+6])
		offset += 6
	}

	if m.Type.QOS() {
		if len(data) < offset+2 {
			df.SetTruncated()
			return fmt.Errorf("Dot11 length %v too short, %v required", len(data), offset+6)
		}
		m.QOS = &Dot11QOS{
			TID:       (uint8(data[offset]) & 0x0F),
			EOSP:      (uint8(data[offset]) & 0x10) == 0x10,
			AckPolicy: Dot11AckPolicy((uint8(data[offset]) & 0x60) >> 5),
			TXOP:      uint8(data[offset+1]),
		}
		offset += 2
	}
	if m.Flags.Order() && (m.Type.QOS() || mainType == Dot11TypeMgmt) {
		if len(data) < offset+4 {
			df.SetTruncated()
			return fmt.Errorf("Dot11 length %v too short, %v required", len(data), offset+6)
		}

		htc := &Dot11HTControl{
			ACConstraint: data[offset+3]&0x40 != 0,
			RDGMorePPDU:  data[offset+3]&0x80 != 0,
		}
		m.HTControl = htc

		if data[offset]&0x1 != 0 { // VHT Variant
			vht := &Dot11HTControlVHT{}
			htc.VHT = vht
			vht.MRQ = data[offset]&0x4 != 0
			vht.UnsolicitedMFB = data[offset+3]&0x20 != 0
			vht.MFB = Dot11HTControlMFB{
				NumSTS: uint8(data[offset+1] >> 1 & 0x7),
				VHTMCS: uint8(data[offset+1] >> 4 & 0xF),
				BW:     uint8(data[offset+2] & 0x3),
				SNR:    int8((-(data[offset+2] >> 2 & 0x20))+data[offset+2]>>2&0x1F) + 22,
			}

			if vht.UnsolicitedMFB {
				if !vht.MFB.NoFeedBackPresent() {
					vht.CompressedMSI = createU8(data[offset] >> 3 & 0x3)
					vht.STBCIndication = data[offset]&0x20 != 0
					vht.CodingType = (*Dot11CodingType)(createU8(data[offset+3] >> 3 & 0x1))
					vht.FbTXBeamformed = data[offset+3]&0x10 != 0
					vht.GID = createU8(
						data[offset]>>6 +
							(data[offset+1] & 0x1 << 2) +
							data[offset+3]&0x7<<3)
				}
			} else {
				if vht.MRQ {
					vht.MSI = createU8((data[offset] >> 3) & 0x07)
				}
				vht.MFSI = createU8(data[offset]>>6 + (data[offset+1] & 0x1 << 2))
			}

		} else { // HT Variant
			ht := &Dot11HTControlHT{}
			htc.HT = ht

			lac := &Dot11LinkAdapationControl{}
			ht.LinkAdapationControl = lac
			lac.TRQ = data[offset]&0x2 != 0
			lac.MFSI = data[offset]>>6&0x3 + data[offset+1]&0x1<<3
			if data[offset]&0x3C == 0x38 { // ASEL
				lac.ASEL = &Dot11ASEL{
					Command: data[offset+1] >> 1 & 0x7,
					Data:    data[offset+1] >> 4 & 0xF,
				}
			} else {
				lac.MRQ = data[offset]&0x4 != 0
				if lac.MRQ {
					lac.MSI = data[offset] >> 3 & 0x7
				}
				lac.MFB = createU8(data[offset+1] >> 1)
			}
			ht.CalibrationPosition = data[offset+2] & 0x3
			ht.CalibrationSequence = data[offset+2] >> 2 & 0x3
			ht.CSISteering = data[offset+2] >> 6 & 0x3
			ht.NDPAnnouncement = data[offset+3]&0x1 != 0
			if mainType != Dot11TypeMgmt {
				ht.DEI = data[offset+3]&0x20 != 0
			}
		}

		offset += 4
	}

	if len(data) < offset+4 {
		df.SetTruncated()
		return fmt.Errorf("Dot11 length %v too short, %v required", len(data), offset+4)
	}

	m.BaseLayer = BaseLayer{
		Contents: data[0:offset],
		Payload:  data[offset : len(data)-4],
	}

	if mainType == Dot11TypeData {
		d := dataDecodeMap[m.Type]
		if d == nil {
			return fmt.Errorf("unsupported type: %v", m.Type)
		}
		l := d()
		err := l.DecodeFromBytes(m.BaseLayer.Payload, df)
		if err != nil {
			return err
		}
		m.DataLayer = l.(gopacket.Layer)
	}

	m.Checksum = binary.LittleEndian.Uint32(data[len(data)-4 : len(data)])
	return nil
}

func (m *Dot11) ChecksumValid() bool {
	// only for CTRL and MGMT frames
	h := crc32.NewIEEE()
	h.Write(m.Contents)
	h.Write(m.Payload)
	return m.Checksum == h.Sum32()
}

func (m Dot11) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	buf, err := b.PrependBytes(24)

	if err != nil {
		return err
	}

	buf[0] = (uint8(m.Type) << 2) | m.Proto
	buf[1] = uint8(m.Flags)

	binary.LittleEndian.PutUint16(buf[2:4], m.DurationID)

	copy(buf[4:10], m.Address1)

	offset := 10

	switch m.Type.MainType() {
	case Dot11TypeCtrl:
		switch m.Type {
		case Dot11TypeCtrlRTS, Dot11TypeCtrlPowersavePoll, Dot11TypeCtrlCFEnd, Dot11TypeCtrlCFEndAck:
			copy(buf[offset:offset+6], m.Address2)
			offset += 6
		}
	case Dot11TypeMgmt, Dot11TypeData:
		copy(buf[offset:offset+6], m.Address2)
		offset += 6
		copy(buf[offset:offset+6], m.Address3)
		offset += 6

		binary.LittleEndian.PutUint16(buf[offset:offset+2], (m.SequenceNumber<<4)|m.FragmentNumber)
		offset += 2
	}

	if m.Type.MainType() == Dot11TypeData && m.Flags.FromDS() && m.Flags.ToDS() {
		copy(buf[offset:offset+6], m.Address4)
		offset += 6
	}

	return nil
}

// Dot11Mgmt is a base for all IEEE 802.11 management layers.
type Dot11Mgmt struct {
	BaseLayer
}

func (m *Dot11Mgmt) NextLayerType() gopacket.LayerType { return gopacket.LayerTypePayload }
func (m *Dot11Mgmt) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	m.Contents = data
	return nil
}

// Dot11Ctrl is a base for all IEEE 802.11 control layers.
type Dot11Ctrl struct {
	BaseLayer
}

func (m *Dot11Ctrl) NextLayerType() gopacket.LayerType { return gopacket.LayerTypePayload }

func (m *Dot11Ctrl) LayerType() gopacket.LayerType  { return LayerTypeDot11Ctrl }
func (m *Dot11Ctrl) CanDecode() gopacket.LayerClass { return LayerTypeDot11Ctrl }
func (m *Dot11Ctrl) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	m.Contents = data
	return nil
}

func decodeDot11Ctrl(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11Ctrl{}
	return decodingLayerDecoder(d, data, p)
}

// Dot11WEP contains WEP encrpted IEEE 802.11 data.
type Dot11WEP struct {
	BaseLayer
}

func (m *Dot11WEP) NextLayerType() gopacket.LayerType { return gopacket.LayerTypePayload }

func (m *Dot11WEP) LayerType() gopacket.LayerType  { return LayerTypeDot11WEP }
func (m *Dot11WEP) CanDecode() gopacket.LayerClass { return LayerTypeDot11WEP }
func (m *Dot11WEP) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	m.Contents = data
	return nil
}

func decodeDot11WEP(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11WEP{}
	return decodingLayerDecoder(d, data, p)
}

// Dot11Data is a base for all IEEE 802.11 data layers.
type Dot11Data struct {
	BaseLayer
}

func (m *Dot11Data) NextLayerType() gopacket.LayerType {
	return LayerTypeLLC
}

func (m *Dot11Data) LayerType() gopacket.LayerType  { return LayerTypeDot11Data }
func (m *Dot11Data) CanDecode() gopacket.LayerClass { return LayerTypeDot11Data }
func (m *Dot11Data) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	m.Payload = data
	return nil
}

func decodeDot11Data(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11Data{}
	return decodingLayerDecoder(d, data, p)
}

type Dot11DataCFAck struct {
	Dot11Data
}

func decodeDot11DataCFAck(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11DataCFAck{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11DataCFAck) LayerType() gopacket.LayerType  { return LayerTypeDot11DataCFAck }
func (m *Dot11DataCFAck) CanDecode() gopacket.LayerClass { return LayerTypeDot11DataCFAck }
func (m *Dot11DataCFAck) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	return m.Dot11Data.DecodeFromBytes(data, df)
}

type Dot11DataCFPoll struct {
	Dot11Data
}

func decodeDot11DataCFPoll(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11DataCFPoll{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11DataCFPoll) LayerType() gopacket.LayerType  { return LayerTypeDot11DataCFPoll }
func (m *Dot11DataCFPoll) CanDecode() gopacket.LayerClass { return LayerTypeDot11DataCFPoll }
func (m *Dot11DataCFPoll) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	return m.Dot11Data.DecodeFromBytes(data, df)
}

type Dot11DataCFAckPoll struct {
	Dot11Data
}

func decodeDot11DataCFAckPoll(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11DataCFAckPoll{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11DataCFAckPoll) LayerType() gopacket.LayerType  { return LayerTypeDot11DataCFAckPoll }
func (m *Dot11DataCFAckPoll) CanDecode() gopacket.LayerClass { return LayerTypeDot11DataCFAckPoll }
func (m *Dot11DataCFAckPoll) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	return m.Dot11Data.DecodeFromBytes(data, df)
}

type Dot11DataNull struct {
	Dot11Data
}

func decodeDot11DataNull(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11DataNull{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11DataNull) LayerType() gopacket.LayerType  { return LayerTypeDot11DataNull }
func (m *Dot11DataNull) CanDecode() gopacket.LayerClass { return LayerTypeDot11DataNull }
func (m *Dot11DataNull) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	return m.Dot11Data.DecodeFromBytes(data, df)
}

type Dot11DataCFAckNoData struct {
	Dot11Data
}

func decodeDot11DataCFAckNoData(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11DataCFAckNoData{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11DataCFAckNoData) LayerType() gopacket.LayerType  { return LayerTypeDot11DataCFAckNoData }
func (m *Dot11DataCFAckNoData) CanDecode() gopacket.LayerClass { return LayerTypeDot11DataCFAckNoData }
func (m *Dot11DataCFAckNoData) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	return m.Dot11Data.DecodeFromBytes(data, df)
}

type Dot11DataCFPollNoData struct {
	Dot11Data
}

func decodeDot11DataCFPollNoData(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11DataCFPollNoData{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11DataCFPollNoData) LayerType() gopacket.LayerType { return LayerTypeDot11DataCFPollNoData }
func (m *Dot11DataCFPollNoData) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11DataCFPollNoData
}
func (m *Dot11DataCFPollNoData) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	return m.Dot11Data.DecodeFromBytes(data, df)
}

type Dot11DataCFAckPollNoData struct {
	Dot11Data
}

func decodeDot11DataCFAckPollNoData(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11DataCFAckPollNoData{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11DataCFAckPollNoData) LayerType() gopacket.LayerType {
	return LayerTypeDot11DataCFAckPollNoData
}
func (m *Dot11DataCFAckPollNoData) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11DataCFAckPollNoData
}
func (m *Dot11DataCFAckPollNoData) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	return m.Dot11Data.DecodeFromBytes(data, df)
}

type Dot11DataQOS struct {
	Dot11Ctrl
}

func (m *Dot11DataQOS) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	m.BaseLayer = BaseLayer{Payload: data}
	return nil
}

type Dot11DataQOSData struct {
	Dot11DataQOS
}

func decodeDot11DataQOSData(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11DataQOSData{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11DataQOSData) LayerType() gopacket.LayerType  { return LayerTypeDot11DataQOSData }
func (m *Dot11DataQOSData) CanDecode() gopacket.LayerClass { return LayerTypeDot11DataQOSData }

func (m *Dot11DataQOSData) NextLayerType() gopacket.LayerType {
	return LayerTypeDot11Data
}

type Dot11DataQOSDataCFAck struct {
	Dot11DataQOS
}

func decodeDot11DataQOSDataCFAck(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11DataQOSDataCFAck{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11DataQOSDataCFAck) LayerType() gopacket.LayerType { return LayerTypeDot11DataQOSDataCFAck }
func (m *Dot11DataQOSDataCFAck) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11DataQOSDataCFAck
}
func (m *Dot11DataQOSDataCFAck) NextLayerType() gopacket.LayerType { return LayerTypeDot11DataCFAck }

type Dot11DataQOSDataCFPoll struct {
	Dot11DataQOS
}

func decodeDot11DataQOSDataCFPoll(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11DataQOSDataCFPoll{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11DataQOSDataCFPoll) LayerType() gopacket.LayerType {
	return LayerTypeDot11DataQOSDataCFPoll
}
func (m *Dot11DataQOSDataCFPoll) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11DataQOSDataCFPoll
}
func (m *Dot11DataQOSDataCFPoll) NextLayerType() gopacket.LayerType { return LayerTypeDot11DataCFPoll }

type Dot11DataQOSDataCFAckPoll struct {
	Dot11DataQOS
}

func decodeDot11DataQOSDataCFAckPoll(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11DataQOSDataCFAckPoll{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11DataQOSDataCFAckPoll) LayerType() gopacket.LayerType {
	return LayerTypeDot11DataQOSDataCFAckPoll
}
func (m *Dot11DataQOSDataCFAckPoll) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11DataQOSDataCFAckPoll
}
func (m *Dot11DataQOSDataCFAckPoll) NextLayerType() gopacket.LayerType {
	return LayerTypeDot11DataCFAckPoll
}

type Dot11DataQOSNull struct {
	Dot11DataQOS
}

func decodeDot11DataQOSNull(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11DataQOSNull{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11DataQOSNull) LayerType() gopacket.LayerType     { return LayerTypeDot11DataQOSNull }
func (m *Dot11DataQOSNull) CanDecode() gopacket.LayerClass    { return LayerTypeDot11DataQOSNull }
func (m *Dot11DataQOSNull) NextLayerType() gopacket.LayerType { return LayerTypeDot11DataNull }

type Dot11DataQOSCFPollNoData struct {
	Dot11DataQOS
}

func decodeDot11DataQOSCFPollNoData(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11DataQOSCFPollNoData{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11DataQOSCFPollNoData) LayerType() gopacket.LayerType {
	return LayerTypeDot11DataQOSCFPollNoData
}
func (m *Dot11DataQOSCFPollNoData) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11DataQOSCFPollNoData
}
func (m *Dot11DataQOSCFPollNoData) NextLayerType() gopacket.LayerType {
	return LayerTypeDot11DataCFPollNoData
}

type Dot11DataQOSCFAckPollNoData struct {
	Dot11DataQOS
}

func decodeDot11DataQOSCFAckPollNoData(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11DataQOSCFAckPollNoData{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11DataQOSCFAckPollNoData) LayerType() gopacket.LayerType {
	return LayerTypeDot11DataQOSCFAckPollNoData
}
func (m *Dot11DataQOSCFAckPollNoData) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11DataQOSCFAckPollNoData
}
func (m *Dot11DataQOSCFAckPollNoData) NextLayerType() gopacket.LayerType {
	return LayerTypeDot11DataCFAckPollNoData
}

type Dot11InformationElement struct {
	BaseLayer
	ID     Dot11InformationElementID
	Length uint8
	OUI    []byte
	Info   []byte
}

func (m *Dot11InformationElement) LayerType() gopacket.LayerType {
	return LayerTypeDot11InformationElement
}
func (m *Dot11InformationElement) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11InformationElement
}

func (m *Dot11InformationElement) NextLayerType() gopacket.LayerType {
	return LayerTypeDot11InformationElement
}

func (m *Dot11InformationElement) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 2 {
		df.SetTruncated()
		return fmt.Errorf("Dot11InformationElement length %v too short, %v required", len(data), 2)
	}
	m.ID = Dot11InformationElementID(data[0])
	m.Length = data[1]
	offset := int(2)

	if len(data) < offset+int(m.Length) {
		df.SetTruncated()
		return fmt.Errorf("Dot11InformationElement length %v too short, %v required", len(data), offset+int(m.Length))
	}
	if len(data) < offset+4 {
		df.SetTruncated()
		return fmt.Errorf("vendor extension size < %d", offset+int(m.Length))
	}
	if m.ID == 221 {
		// Vendor extension
		m.OUI = data[offset : offset+4]
		m.Info = data[offset+4 : offset+int(m.Length)]
	} else {
		m.Info = data[offset : offset+int(m.Length)]
	}

	offset += int(m.Length)

	m.BaseLayer = BaseLayer{Contents: data[:offset], Payload: data[offset:]}
	return nil
}

func (d *Dot11InformationElement) String() string {
	if d.ID == 0 {
		return fmt.Sprintf("802.11 Information Element (ID: %v, Length: %v, SSID: %v)", d.ID, d.Length, string(d.Info))
	} else if d.ID == 1 {
		rates := ""
		for i := 0; i < len(d.Info); i++ {
			if d.Info[i]&0x80 == 0 {
				rates += fmt.Sprintf("%.1f ", float32(d.Info[i])*0.5)
			} else {
				rates += fmt.Sprintf("%.1f* ", float32(d.Info[i]&0x7F)*0.5)
			}
		}
		return fmt.Sprintf("802.11 Information Element (ID: %v, Length: %v, Rates: %s Mbit)", d.ID, d.Length, rates)
	} else if d.ID == 221 {
		return fmt.Sprintf("802.11 Information Element (ID: %v, Length: %v, OUI: %X, Info: %X)", d.ID, d.Length, d.OUI, d.Info)
	} else {
		return fmt.Sprintf("802.11 Information Element (ID: %v, Length: %v, Info: %X)", d.ID, d.Length, d.Info)
	}
}

func (m Dot11InformationElement) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	length := len(m.Info) + len(m.OUI)
	if buf, err := b.PrependBytes(2 + length); err != nil {
		return err
	} else {
		buf[0] = uint8(m.ID)
		buf[1] = uint8(length)
		copy(buf[2:], m.OUI)
		copy(buf[2+len(m.OUI):], m.Info)
	}
	return nil
}

func decodeDot11InformationElement(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11InformationElement{}
	return decodingLayerDecoder(d, data, p)
}

type Dot11CtrlCTS struct {
	Dot11Ctrl
}

func decodeDot11CtrlCTS(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11CtrlCTS{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11CtrlCTS) LayerType() gopacket.LayerType {
	return LayerTypeDot11CtrlCTS
}
func (m *Dot11CtrlCTS) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11CtrlCTS
}
func (m *Dot11CtrlCTS) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	return m.Dot11Ctrl.DecodeFromBytes(data, df)
}

type Dot11CtrlRTS struct {
	Dot11Ctrl
}

func decodeDot11CtrlRTS(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11CtrlRTS{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11CtrlRTS) LayerType() gopacket.LayerType {
	return LayerTypeDot11CtrlRTS
}
func (m *Dot11CtrlRTS) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11CtrlRTS
}
func (m *Dot11CtrlRTS) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	return m.Dot11Ctrl.DecodeFromBytes(data, df)
}

type Dot11CtrlBlockAckReq struct {
	Dot11Ctrl
}

func decodeDot11CtrlBlockAckReq(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11CtrlBlockAckReq{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11CtrlBlockAckReq) LayerType() gopacket.LayerType {
	return LayerTypeDot11CtrlBlockAckReq
}
func (m *Dot11CtrlBlockAckReq) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11CtrlBlockAckReq
}
func (m *Dot11CtrlBlockAckReq) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	return m.Dot11Ctrl.DecodeFromBytes(data, df)
}

type Dot11CtrlBlockAck struct {
	Dot11Ctrl
}

func decodeDot11CtrlBlockAck(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11CtrlBlockAck{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11CtrlBlockAck) LayerType() gopacket.LayerType  { return LayerTypeDot11CtrlBlockAck }
func (m *Dot11CtrlBlockAck) CanDecode() gopacket.LayerClass { return LayerTypeDot11CtrlBlockAck }
func (m *Dot11CtrlBlockAck) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	return m.Dot11Ctrl.DecodeFromBytes(data, df)
}

type Dot11CtrlPowersavePoll struct {
	Dot11Ctrl
}

func decodeDot11CtrlPowersavePoll(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11CtrlPowersavePoll{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11CtrlPowersavePoll) LayerType() gopacket.LayerType {
	return LayerTypeDot11CtrlPowersavePoll
}
func (m *Dot11CtrlPowersavePoll) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11CtrlPowersavePoll
}
func (m *Dot11CtrlPowersavePoll) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	return m.Dot11Ctrl.DecodeFromBytes(data, df)
}

type Dot11CtrlAck struct {
	Dot11Ctrl
}

func decodeDot11CtrlAck(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11CtrlAck{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11CtrlAck) LayerType() gopacket.LayerType  { return LayerTypeDot11CtrlAck }
func (m *Dot11CtrlAck) CanDecode() gopacket.LayerClass { return LayerTypeDot11CtrlAck }
func (m *Dot11CtrlAck) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	return m.Dot11Ctrl.DecodeFromBytes(data, df)
}

type Dot11CtrlCFEnd struct {
	Dot11Ctrl
}

func decodeDot11CtrlCFEnd(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11CtrlCFEnd{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11CtrlCFEnd) LayerType() gopacket.LayerType {
	return LayerTypeDot11CtrlCFEnd
}
func (m *Dot11CtrlCFEnd) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11CtrlCFEnd
}
func (m *Dot11CtrlCFEnd) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	return m.Dot11Ctrl.DecodeFromBytes(data, df)
}

type Dot11CtrlCFEndAck struct {
	Dot11Ctrl
}

func decodeDot11CtrlCFEndAck(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11CtrlCFEndAck{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11CtrlCFEndAck) LayerType() gopacket.LayerType {
	return LayerTypeDot11CtrlCFEndAck
}
func (m *Dot11CtrlCFEndAck) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11CtrlCFEndAck
}
func (m *Dot11CtrlCFEndAck) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	return m.Dot11Ctrl.DecodeFromBytes(data, df)
}

type Dot11MgmtAssociationReq struct {
	Dot11Mgmt
	CapabilityInfo uint16
	ListenInterval uint16
}

func decodeDot11MgmtAssociationReq(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11MgmtAssociationReq{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11MgmtAssociationReq) LayerType() gopacket.LayerType {
	return LayerTypeDot11MgmtAssociationReq
}
func (m *Dot11MgmtAssociationReq) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11MgmtAssociationReq
}
func (m *Dot11MgmtAssociationReq) NextLayerType() gopacket.LayerType {
	return LayerTypeDot11InformationElement
}
func (m *Dot11MgmtAssociationReq) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 4 {
		df.SetTruncated()
		return fmt.Errorf("Dot11MgmtAssociationReq length %v too short, %v required", len(data), 4)
	}
	m.CapabilityInfo = binary.LittleEndian.Uint16(data[0:2])
	m.ListenInterval = binary.LittleEndian.Uint16(data[2:4])
	m.Payload = data[4:]
	return m.Dot11Mgmt.DecodeFromBytes(data, df)
}

func (m Dot11MgmtAssociationReq) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	buf, err := b.PrependBytes(4)

	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint16(buf[0:2], m.CapabilityInfo)
	binary.LittleEndian.PutUint16(buf[2:4], m.ListenInterval)

	return nil
}

type Dot11MgmtAssociationResp struct {
	Dot11Mgmt
	CapabilityInfo uint16
	Status         Dot11Status
	AID            uint16
}

func decodeDot11MgmtAssociationResp(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11MgmtAssociationResp{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11MgmtAssociationResp) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11MgmtAssociationResp
}
func (m *Dot11MgmtAssociationResp) LayerType() gopacket.LayerType {
	return LayerTypeDot11MgmtAssociationResp
}
func (m *Dot11MgmtAssociationResp) NextLayerType() gopacket.LayerType {
	return LayerTypeDot11InformationElement
}
func (m *Dot11MgmtAssociationResp) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 6 {
		df.SetTruncated()
		return fmt.Errorf("Dot11MgmtAssociationResp length %v too short, %v required", len(data), 6)
	}
	m.CapabilityInfo = binary.LittleEndian.Uint16(data[0:2])
	m.Status = Dot11Status(binary.LittleEndian.Uint16(data[2:4]))
	m.AID = binary.LittleEndian.Uint16(data[4:6])
	m.Payload = data[6:]
	return m.Dot11Mgmt.DecodeFromBytes(data, df)
}

func (m Dot11MgmtAssociationResp) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	buf, err := b.PrependBytes(6)

	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint16(buf[0:2], m.CapabilityInfo)
	binary.LittleEndian.PutUint16(buf[2:4], uint16(m.Status))
	binary.LittleEndian.PutUint16(buf[4:6], m.AID)

	return nil
}

type Dot11MgmtReassociationReq struct {
	Dot11Mgmt
	CapabilityInfo   uint16
	ListenInterval   uint16
	CurrentApAddress net.HardwareAddr
}

func decodeDot11MgmtReassociationReq(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11MgmtReassociationReq{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11MgmtReassociationReq) LayerType() gopacket.LayerType {
	return LayerTypeDot11MgmtReassociationReq
}
func (m *Dot11MgmtReassociationReq) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11MgmtReassociationReq
}
func (m *Dot11MgmtReassociationReq) NextLayerType() gopacket.LayerType {
	return LayerTypeDot11InformationElement
}
func (m *Dot11MgmtReassociationReq) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 10 {
		df.SetTruncated()
		return fmt.Errorf("Dot11MgmtReassociationReq length %v too short, %v required", len(data), 10)
	}
	m.CapabilityInfo = binary.LittleEndian.Uint16(data[0:2])
	m.ListenInterval = binary.LittleEndian.Uint16(data[2:4])
	m.CurrentApAddress = net.HardwareAddr(data[4:10])
	m.Payload = data[10:]
	return m.Dot11Mgmt.DecodeFromBytes(data, df)
}

func (m Dot11MgmtReassociationReq) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	buf, err := b.PrependBytes(10)

	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint16(buf[0:2], m.CapabilityInfo)
	binary.LittleEndian.PutUint16(buf[2:4], m.ListenInterval)

	copy(buf[4:10], m.CurrentApAddress)

	return nil
}

type Dot11MgmtReassociationResp struct {
	Dot11Mgmt
}

func decodeDot11MgmtReassociationResp(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11MgmtReassociationResp{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11MgmtReassociationResp) LayerType() gopacket.LayerType {
	return LayerTypeDot11MgmtReassociationResp
}
func (m *Dot11MgmtReassociationResp) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11MgmtReassociationResp
}
func (m *Dot11MgmtReassociationResp) NextLayerType() gopacket.LayerType {
	return LayerTypeDot11InformationElement
}

type Dot11MgmtProbeReq struct {
	Dot11Mgmt
}

func decodeDot11MgmtProbeReq(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11MgmtProbeReq{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11MgmtProbeReq) LayerType() gopacket.LayerType  { return LayerTypeDot11MgmtProbeReq }
func (m *Dot11MgmtProbeReq) CanDecode() gopacket.LayerClass { return LayerTypeDot11MgmtProbeReq }
func (m *Dot11MgmtProbeReq) NextLayerType() gopacket.LayerType {
	return LayerTypeDot11InformationElement
}

type Dot11MgmtProbeResp struct {
	Dot11Mgmt
	Timestamp uint64
	Interval  uint16
	Flags     uint16
}

func decodeDot11MgmtProbeResp(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11MgmtProbeResp{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11MgmtProbeResp) LayerType() gopacket.LayerType  { return LayerTypeDot11MgmtProbeResp }
func (m *Dot11MgmtProbeResp) CanDecode() gopacket.LayerClass { return LayerTypeDot11MgmtProbeResp }
func (m *Dot11MgmtProbeResp) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 12 {
		df.SetTruncated()

		return fmt.Errorf("Dot11MgmtProbeResp length %v too short, %v required", len(data), 12)
	}

	m.Timestamp = binary.LittleEndian.Uint64(data[0:8])
	m.Interval = binary.LittleEndian.Uint16(data[8:10])
	m.Flags = binary.LittleEndian.Uint16(data[10:12])
	m.Payload = data[12:]

	return m.Dot11Mgmt.DecodeFromBytes(data, df)
}

func (m *Dot11MgmtProbeResp) NextLayerType() gopacket.LayerType {
	return LayerTypeDot11InformationElement
}

func (m Dot11MgmtProbeResp) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	buf, err := b.PrependBytes(12)

	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint64(buf[0:8], m.Timestamp)
	binary.LittleEndian.PutUint16(buf[8:10], m.Interval)
	binary.LittleEndian.PutUint16(buf[10:12], m.Flags)

	return nil
}

type Dot11MgmtMeasurementPilot struct {
	Dot11Mgmt
}

func decodeDot11MgmtMeasurementPilot(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11MgmtMeasurementPilot{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11MgmtMeasurementPilot) LayerType() gopacket.LayerType {
	return LayerTypeDot11MgmtMeasurementPilot
}
func (m *Dot11MgmtMeasurementPilot) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11MgmtMeasurementPilot
}

type Dot11MgmtBeacon struct {
	Dot11Mgmt
	Timestamp uint64
	Interval  uint16
	Flags     uint16
}

func decodeDot11MgmtBeacon(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11MgmtBeacon{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11MgmtBeacon) LayerType() gopacket.LayerType  { return LayerTypeDot11MgmtBeacon }
func (m *Dot11MgmtBeacon) CanDecode() gopacket.LayerClass { return LayerTypeDot11MgmtBeacon }
func (m *Dot11MgmtBeacon) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 12 {
		df.SetTruncated()
		return fmt.Errorf("Dot11MgmtBeacon length %v too short, %v required", len(data), 12)
	}
	m.Timestamp = binary.LittleEndian.Uint64(data[0:8])
	m.Interval = binary.LittleEndian.Uint16(data[8:10])
	m.Flags = binary.LittleEndian.Uint16(data[10:12])
	m.Payload = data[12:]
	return m.Dot11Mgmt.DecodeFromBytes(data, df)
}

func (m *Dot11MgmtBeacon) NextLayerType() gopacket.LayerType { return LayerTypeDot11InformationElement }

func (m Dot11MgmtBeacon) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	buf, err := b.PrependBytes(12)

	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint64(buf[0:8], m.Timestamp)
	binary.LittleEndian.PutUint16(buf[8:10], m.Interval)
	binary.LittleEndian.PutUint16(buf[10:12], m.Flags)

	return nil
}

type Dot11MgmtATIM struct {
	Dot11Mgmt
}

func decodeDot11MgmtATIM(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11MgmtATIM{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11MgmtATIM) LayerType() gopacket.LayerType  { return LayerTypeDot11MgmtATIM }
func (m *Dot11MgmtATIM) CanDecode() gopacket.LayerClass { return LayerTypeDot11MgmtATIM }

type Dot11MgmtDisassociation struct {
	Dot11Mgmt
	Reason Dot11Reason
}

func decodeDot11MgmtDisassociation(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11MgmtDisassociation{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11MgmtDisassociation) LayerType() gopacket.LayerType {
	return LayerTypeDot11MgmtDisassociation
}
func (m *Dot11MgmtDisassociation) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11MgmtDisassociation
}
func (m *Dot11MgmtDisassociation) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 2 {
		df.SetTruncated()
		return fmt.Errorf("Dot11MgmtDisassociation length %v too short, %v required", len(data), 2)
	}
	m.Reason = Dot11Reason(binary.LittleEndian.Uint16(data[0:2]))
	return m.Dot11Mgmt.DecodeFromBytes(data, df)
}

func (m Dot11MgmtDisassociation) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	buf, err := b.PrependBytes(2)

	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint16(buf[0:2], uint16(m.Reason))

	return nil
}

type Dot11MgmtAuthentication struct {
	Dot11Mgmt
	Algorithm Dot11Algorithm
	Sequence  uint16
	Status    Dot11Status
}

func decodeDot11MgmtAuthentication(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11MgmtAuthentication{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11MgmtAuthentication) LayerType() gopacket.LayerType {
	return LayerTypeDot11MgmtAuthentication
}
func (m *Dot11MgmtAuthentication) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11MgmtAuthentication
}
func (m *Dot11MgmtAuthentication) NextLayerType() gopacket.LayerType {
	return LayerTypeDot11InformationElement
}
func (m *Dot11MgmtAuthentication) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 6 {
		df.SetTruncated()
		return fmt.Errorf("Dot11MgmtAuthentication length %v too short, %v required", len(data), 6)
	}
	m.Algorithm = Dot11Algorithm(binary.LittleEndian.Uint16(data[0:2]))
	m.Sequence = binary.LittleEndian.Uint16(data[2:4])
	m.Status = Dot11Status(binary.LittleEndian.Uint16(data[4:6]))
	m.Payload = data[6:]
	return m.Dot11Mgmt.DecodeFromBytes(data, df)
}

func (m Dot11MgmtAuthentication) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	buf, err := b.PrependBytes(6)

	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint16(buf[0:2], uint16(m.Algorithm))
	binary.LittleEndian.PutUint16(buf[2:4], m.Sequence)
	binary.LittleEndian.PutUint16(buf[4:6], uint16(m.Status))

	return nil
}

type Dot11MgmtDeauthentication struct {
	Dot11Mgmt
	Reason Dot11Reason
}

func decodeDot11MgmtDeauthentication(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11MgmtDeauthentication{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11MgmtDeauthentication) LayerType() gopacket.LayerType {
	return LayerTypeDot11MgmtDeauthentication
}
func (m *Dot11MgmtDeauthentication) CanDecode() gopacket.LayerClass {
	return LayerTypeDot11MgmtDeauthentication
}
func (m *Dot11MgmtDeauthentication) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 2 {
		df.SetTruncated()
		return fmt.Errorf("Dot11MgmtDeauthentication length %v too short, %v required", len(data), 2)
	}
	m.Reason = Dot11Reason(binary.LittleEndian.Uint16(data[0:2]))
	return m.Dot11Mgmt.DecodeFromBytes(data, df)
}

func (m Dot11MgmtDeauthentication) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	buf, err := b.PrependBytes(2)

	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint16(buf[0:2], uint16(m.Reason))

	return nil
}

type Dot11MgmtAction struct {
	Dot11Mgmt
}

func decodeDot11MgmtAction(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11MgmtAction{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11MgmtAction) LayerType() gopacket.LayerType  { return LayerTypeDot11MgmtAction }
func (m *Dot11MgmtAction) CanDecode() gopacket.LayerClass { return LayerTypeDot11MgmtAction }

type Dot11MgmtActionNoAck struct {
	Dot11Mgmt
}

func decodeDot11MgmtActionNoAck(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11MgmtActionNoAck{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11MgmtActionNoAck) LayerType() gopacket.LayerType  { return LayerTypeDot11MgmtActionNoAck }
func (m *Dot11MgmtActionNoAck) CanDecode() gopacket.LayerClass { return LayerTypeDot11MgmtActionNoAck }

type Dot11MgmtArubaWLAN struct {
	Dot11Mgmt
}

func decodeDot11MgmtArubaWLAN(data []byte, p gopacket.PacketBuilder) error {
	d := &Dot11MgmtArubaWLAN{}
	return decodingLayerDecoder(d, data, p)
}

func (m *Dot11MgmtArubaWLAN) LayerType() gopacket.LayerType  { return LayerTypeDot11MgmtArubaWLAN }
func (m *Dot11MgmtArubaWLAN) CanDecode() gopacket.LayerClass { return LayerTypeDot11MgmtArubaWLAN }
