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

type PFDirection uint8

const (
	PFDirectionInOut PFDirection = 0
	PFDirectionIn    PFDirection = 1
	PFDirectionOut   PFDirection = 2
)

// PFLog provides the layer for 'pf' packet-filter logging, as described at
// http://www.freebsd.org/cgi/man.cgi?query=pflog&sektion=4
type PFLog struct {
	BaseLayer
	Length              uint8
	Family              ProtocolFamily
	Action, Reason      uint8
	IFName, Ruleset     []byte
	RuleNum, SubruleNum uint32
	UID                 uint32
	PID                 int32
	RuleUID             uint32
	RulePID             int32
	Direction           PFDirection
	// The remainder is padding
}

func (pf *PFLog) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 60 {
		df.SetTruncated()
		return errors.New("PFLog data less than 60 bytes")
	}
	pf.Length = data[0]
	pf.Family = ProtocolFamily(data[1])
	pf.Action = data[2]
	pf.Reason = data[3]
	pf.IFName = data[4:20]
	pf.Ruleset = data[20:36]
	pf.RuleNum = binary.BigEndian.Uint32(data[36:40])
	pf.SubruleNum = binary.BigEndian.Uint32(data[40:44])
	pf.UID = binary.BigEndian.Uint32(data[44:48])
	pf.PID = int32(binary.BigEndian.Uint32(data[48:52]))
	pf.RuleUID = binary.BigEndian.Uint32(data[52:56])
	pf.RulePID = int32(binary.BigEndian.Uint32(data[56:60]))
	pf.Direction = PFDirection(data[60])
	if pf.Length%4 != 1 {
		return errors.New("PFLog header length should be 3 less than multiple of 4")
	}
	actualLength := int(pf.Length) + 3
	if len(data) < actualLength {
		return fmt.Errorf("PFLog data size < %d", actualLength)
	}
	pf.Contents = data[:actualLength]
	pf.Payload = data[actualLength:]
	return nil
}

// LayerType returns layers.LayerTypePFLog
func (pf *PFLog) LayerType() gopacket.LayerType { return LayerTypePFLog }

func (pf *PFLog) CanDecode() gopacket.LayerClass { return LayerTypePFLog }

func (pf *PFLog) NextLayerType() gopacket.LayerType {
	return pf.Family.LayerType()
}

func decodePFLog(data []byte, p gopacket.PacketBuilder) error {
	pf := &PFLog{}
	return decodingLayerDecoder(pf, data, p)
}
