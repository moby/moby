// Copyright The OpenTelemetry Authors
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

// Package netconv provides OpenTelemetry network semantic conventions for
// tracing telemetry.
package netconv // import "go.opentelemetry.io/otel/semconv/v1.17.0/netconv"

import (
	"net"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/semconv/internal/v2"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

var nc = &internal.NetConv{
	NetHostNameKey:     semconv.NetHostNameKey,
	NetHostPortKey:     semconv.NetHostPortKey,
	NetPeerNameKey:     semconv.NetPeerNameKey,
	NetPeerPortKey:     semconv.NetPeerPortKey,
	NetSockFamilyKey:   semconv.NetSockFamilyKey,
	NetSockPeerAddrKey: semconv.NetSockPeerAddrKey,
	NetSockPeerPortKey: semconv.NetSockPeerPortKey,
	NetSockHostAddrKey: semconv.NetSockHostAddrKey,
	NetSockHostPortKey: semconv.NetSockHostPortKey,
	NetTransportOther:  semconv.NetTransportOther,
	NetTransportTCP:    semconv.NetTransportTCP,
	NetTransportUDP:    semconv.NetTransportUDP,
	NetTransportInProc: semconv.NetTransportInProc,
}

// Transport returns a trace attribute describing the transport protocol of the
// passed network. See the net.Dial for information about acceptable network
// values.
func Transport(network string) attribute.KeyValue {
	return nc.Transport(network)
}

// Client returns trace attributes for a client network connection to address.
// See net.Dial for information about acceptable address values, address should
// be the same as the one used to create conn. If conn is nil, only network
// peer attributes will be returned that describe address. Otherwise, the
// socket level information about conn will also be included.
func Client(address string, conn net.Conn) []attribute.KeyValue {
	return nc.Client(address, conn)
}

// Server returns trace attributes for a network listener listening at address.
// See net.Listen for information about acceptable address values, address
// should be the same as the one used to create ln. If ln is nil, only network
// host attributes will be returned that describe address. Otherwise, the
// socket level information about ln will also be included.
func Server(address string, ln net.Listener) []attribute.KeyValue {
	return nc.Server(address, ln)
}
