// Package grpcreflect provides GRPC-specific extensions to protobuf reflection.
// This includes a way to access rich service descriptors for all services that
// a GRPC server exports.
//
// Also included is an easy-to-use client for the GRPC reflection service
// (https://goo.gl/2ILAHf). This client makes it easy to ask a server (that
// supports the reflection service) for metadata on its exported services, which
// could be used to construct a dynamic client. (See the grpcdynamic package in
// this same repo for more on that.)
package grpcreflect
