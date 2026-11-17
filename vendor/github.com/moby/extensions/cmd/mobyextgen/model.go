// SPDX-FileCopyrightText: Copyright The Moby Authors
// SPDX-License-Identifier: Apache-2.0

package main

// model

type point struct {
	importPath    string // package import path
	protoPath     string // module-relative path of the .proto
	licenseHeader string // SPDX notices from the contract interface's source file
	id            string // proto package: the point id, or the explicit package in service mode
	service       string // gRPC service name: the contract interface name
	iface         string // Go service interface name
	isPoint       bool   // whether the contract declares an extensions.Point
	isSingle      bool   // whether the point was declared with DefineSinglePoint
	doc           string // documentation on the provider interface
	methods       []method
	messages      []message
}

// grpcService returns the fully-qualified gRPC service name.
func (p point) grpcService() string { return p.id + "." + p.service }

func (p point) protogenImport() string { return p.importPath + "/protogen" }

type method struct {
	kind      methodKind
	name      string // method/rpc name
	request   string // request message name
	response  string // response message name
	bareError bool   // true only for legacy unary `M(ctx, *Req) error`
	doc       string // method documentation
}

type methodKind int

const (
	methodUnary methodKind = iota
	methodServerStreaming
	methodClientStreaming
)

type message struct {
	name     string
	fields   []field
	reserved []int // field numbers burned by a removed field
}

type fieldKind int

const (
	scalarSingle fieldKind = iota
	scalarRepeated
	scalarMap
	messageSingle
	messageRepeated
)

type field struct {
	goName      string // Go field name on the contract struct (e.g. ContainerID)
	protoName   string // proto3 field name (e.g. container_id)
	protoGoName string // Go field name protoc-gen-go emits for protoName (e.g. ContainerId)
	number      int
	protoType   string // proto type token, or the message name for messageSingle/messageRepeated
	mapKey      string // proto key type for scalarMap
	kind        fieldKind
}
