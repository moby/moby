// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package serf

import (
	"bytes"
	"fmt"

	"github.com/hashicorp/go-metrics/compat"
	"github.com/hashicorp/go-msgpack/v2/codec"
	"github.com/hashicorp/memberlist"
)

// delegate is the memberlist.Delegate implementation that Serf uses.
type delegate struct {
	serf *Serf
}

var _ memberlist.Delegate = &delegate{}

func (d *delegate) NodeMeta(limit int) []byte {
	roleBytes := d.serf.encodeTags(d.serf.config.Tags)
	if len(roleBytes) > limit {
		panic(fmt.Errorf("Node tags '%v' exceeds length limit of %d bytes", d.serf.config.Tags, limit))
	}

	return roleBytes
}

func (d *delegate) NotifyMsg(buf []byte) {
	// If we didn't actually receive any data, then ignore it.
	if len(buf) == 0 {
		return
	}
	metrics.AddSampleWithLabels([]string{"serf", "msgs", "received"}, float32(len(buf)), d.serf.metricLabels)

	rebroadcast := false
	rebroadcastQueue := d.serf.broadcasts
	t := messageType(buf[0])

	if d.serf.config.messageDropper(t) {
		return
	}

	switch t {
	case messageLeaveType:
		var leave messageLeave
		if err := decodeMessage(buf[1:], &leave); err != nil {
			d.serf.logger.Printf("[ERR] serf: Error decoding leave message: %s", err)
			break
		}

		d.serf.logger.Printf("[DEBUG] serf: messageLeaveType: %s", leave.Node)
		rebroadcast = d.serf.handleNodeLeaveIntent(&leave)

	case messageJoinType:
		var join messageJoin
		if err := decodeMessage(buf[1:], &join); err != nil {
			d.serf.logger.Printf("[ERR] serf: Error decoding join message: %s", err)
			break
		}

		d.serf.logger.Printf("[DEBUG] serf: messageJoinType: %s", join.Node)
		rebroadcast = d.serf.handleNodeJoinIntent(&join)

	case messageUserEventType:
		var event messageUserEvent
		if err := decodeMessage(buf[1:], &event); err != nil {
			d.serf.logger.Printf("[ERR] serf: Error decoding user event message: %s", err)
			break
		}

		d.serf.logger.Printf("[DEBUG] serf: messageUserEventType: %s", event.Name)
		rebroadcast = d.serf.handleUserEvent(&event)
		rebroadcastQueue = d.serf.eventBroadcasts

	case messageQueryType:
		var query messageQuery
		if err := decodeMessage(buf[1:], &query); err != nil {
			d.serf.logger.Printf("[ERR] serf: Error decoding query message: %s", err)
			break
		}

		d.serf.logger.Printf("[DEBUG] serf: messageQueryType: %s", query.Name)
		rebroadcast = d.serf.handleQuery(&query)
		rebroadcastQueue = d.serf.queryBroadcasts

	case messageQueryResponseType:
		var resp messageQueryResponse
		if err := decodeMessage(buf[1:], &resp); err != nil {
			d.serf.logger.Printf("[ERR] serf: Error decoding query response message: %s", err)
			break
		}

		d.serf.logger.Printf("[DEBUG] serf: messageQueryResponseType: %v", resp.From)
		d.serf.handleQueryResponse(&resp)

	case messageRelayType:
		var header relayHeader
		var handle codec.MsgpackHandle
		reader := bytes.NewReader(buf[1:])
		decoder := codec.NewDecoder(reader, &handle)
		if err := decoder.Decode(&header); err != nil {
			d.serf.logger.Printf("[ERR] serf: Error decoding relay header: %s", err)
			break
		}

		// The remaining contents are the message itself, so forward that
		raw := make([]byte, reader.Len())
		reader.Read(raw)

		addr := memberlist.Address{
			Addr: header.DestAddr.String(),
			Name: header.DestName,
		}

		d.serf.logger.Printf("[DEBUG] serf: Relaying response to addr: %s", header.DestAddr.String())
		if err := d.serf.memberlist.SendToAddress(addr, raw); err != nil {
			d.serf.logger.Printf("[ERR] serf: Error forwarding message to %s: %s", header.DestAddr.String(), err)
			break
		}

	default:
		d.serf.logger.Printf("[WARN] serf: Received message of unknown type: %d", t)
	}

	if rebroadcast {
		// Copy the buffer since it we cannot rely on the slice not changing
		newBuf := make([]byte, len(buf))
		copy(newBuf, buf)

		rebroadcastQueue.QueueBroadcast(&broadcast{
			msg:    newBuf,
			notify: nil,
		})
	}
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	msgs := d.serf.broadcasts.GetBroadcasts(overhead, limit)

	// Determine the bytes used already
	bytesUsed := 0
	for _, msg := range msgs {
		lm := len(msg)
		bytesUsed += lm + overhead
		metrics.AddSampleWithLabels([]string{"serf", "msgs", "sent"}, float32(lm), d.serf.metricLabels)
	}

	// Get any additional query broadcasts
	queryMsgs := d.serf.queryBroadcasts.GetBroadcasts(overhead, limit-bytesUsed)
	if queryMsgs != nil {
		for _, m := range queryMsgs {
			lm := len(m)
			bytesUsed += lm + overhead
			metrics.AddSampleWithLabels([]string{"serf", "msgs", "sent"}, float32(lm), d.serf.metricLabels)
		}
		msgs = append(msgs, queryMsgs...)
	}

	// Get any additional event broadcasts
	eventMsgs := d.serf.eventBroadcasts.GetBroadcasts(overhead, limit-bytesUsed)
	if eventMsgs != nil {
		for _, m := range eventMsgs {
			lm := len(m)
			bytesUsed += lm + overhead
			metrics.AddSampleWithLabels([]string{"serf", "msgs", "sent"}, float32(lm), d.serf.keyManager.serf.metricLabels)
		}
		msgs = append(msgs, eventMsgs...)
	}

	return msgs
}

func (d *delegate) LocalState(join bool) []byte {
	d.serf.memberLock.RLock()
	defer d.serf.memberLock.RUnlock()
	d.serf.eventLock.RLock()
	defer d.serf.eventLock.RUnlock()

	// Create the message to send
	pp := messagePushPull{
		LTime:        d.serf.clock.Time(),
		StatusLTimes: make(map[string]LamportTime, len(d.serf.members)),
		LeftMembers:  make([]string, 0, len(d.serf.leftMembers)),
		EventLTime:   d.serf.eventClock.Time(),
		Events:       d.serf.eventBuffer,
		QueryLTime:   d.serf.queryClock.Time(),
	}

	// Add all the join LTimes
	for name, member := range d.serf.members {
		pp.StatusLTimes[name] = member.statusLTime
	}

	// Add all the left nodes
	for _, member := range d.serf.leftMembers {
		pp.LeftMembers = append(pp.LeftMembers, member.Name)
	}

	// Encode the push pull state
	buf, err := encodeMessage(messagePushPullType, &pp, d.serf.msgpackUseNewTimeFormat)
	if err != nil {
		d.serf.logger.Printf("[ERR] serf: Failed to encode local state: %v", err)
		return nil
	}
	return buf
}

func (d *delegate) MergeRemoteState(buf []byte, isJoin bool) {
	// Ensure we have a message
	if len(buf) == 0 {
		d.serf.logger.Printf("[ERR] serf: Remote state is zero bytes")
		return
	}

	// Check the message type
	if messageType(buf[0]) != messagePushPullType {
		d.serf.logger.Printf("[ERR] serf: Remote state has bad type prefix: %v", buf[0])
		return
	}

	if d.serf.config.messageDropper(messagePushPullType) {
		return
	}

	// Attempt a decode
	pp := messagePushPull{}
	if err := decodeMessage(buf[1:], &pp); err != nil {
		d.serf.logger.Printf("[ERR] serf: Failed to decode remote state: %v", err)
		return
	}

	// Witness the Lamport clocks first.
	// We subtract 1 since no message with that clock has been sent yet
	if pp.LTime > 0 {
		d.serf.clock.Witness(pp.LTime - 1)
	}
	if pp.EventLTime > 0 {
		d.serf.eventClock.Witness(pp.EventLTime - 1)
	}
	if pp.QueryLTime > 0 {
		d.serf.queryClock.Witness(pp.QueryLTime - 1)
	}

	// Process the left nodes first to avoid the LTimes from incrementing
	// in the wrong order. Note that we don't have the actual Lamport time
	// for the leave message, so we go one past the join time, since the
	// leave must have been accepted after that to get onto the left members
	// list. If we didn't do this then the message would not get processed.
	leftMap := make(map[string]struct{}, len(pp.LeftMembers))
	leave := messageLeave{}
	for _, name := range pp.LeftMembers {
		leftMap[name] = struct{}{}
		leave.LTime = pp.StatusLTimes[name] + 1
		leave.Node = name
		d.serf.handleNodeLeaveIntent(&leave)
	}

	// Update any other LTimes
	join := messageJoin{}
	for name, statusLTime := range pp.StatusLTimes {
		// Skip the left nodes
		if _, ok := leftMap[name]; ok {
			continue
		}

		// Create an artificial join message
		join.LTime = statusLTime
		join.Node = name
		d.serf.handleNodeJoinIntent(&join)
	}

	// If we are doing a join, and eventJoinIgnore is set
	// then we set the eventMinTime to the EventLTime. This
	// prevents any of the incoming events from being processed
	eventJoinIgnore := d.serf.eventJoinIgnore.Load().(bool)
	if isJoin && eventJoinIgnore {
		d.serf.eventLock.Lock()
		if pp.EventLTime > d.serf.eventMinTime {
			d.serf.eventMinTime = pp.EventLTime
		}
		d.serf.eventLock.Unlock()
	}

	// Process all the events
	userEvent := messageUserEvent{}
	for _, events := range pp.Events {
		if events == nil {
			continue
		}
		userEvent.LTime = events.LTime
		for _, e := range events.Events {
			userEvent.Name = e.Name
			userEvent.Payload = e.Payload
			d.serf.handleUserEvent(&userEvent)
		}
	}
}
