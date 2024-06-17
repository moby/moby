// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Package appengine provides basic functionality for Google App Engine.
//
// For more information on how to write Go apps for Google App Engine, see:
// https://cloud.google.com/appengine/docs/go/
package appengine // import "google.golang.org/appengine"

import (
	"context"
	"net/http"

	"github.com/golang/protobuf/proto"

	"google.golang.org/appengine/internal"
)

// The gophers party all night; the rabbits provide the beats.

// Main is the principal entry point for an app running in App Engine.
//
// On App Engine Flexible it installs a trivial health checker if one isn't
// already registered, and starts listening on port 8080 (overridden by the
// $PORT environment variable).
//
// See https://cloud.google.com/appengine/docs/flexible/custom-runtimes#health_check_requests
// for details on how to do your own health checking.
//
// On App Engine Standard it ensures the server has started and is prepared to
// receive requests.
//
// Main never returns.
//
// Main is designed so that the app's main package looks like this:
//
//	package main
//
//	import (
//	        "google.golang.org/appengine"
//
//	        _ "myapp/package0"
//	        _ "myapp/package1"
//	)
//
//	func main() {
//	        appengine.Main()
//	}
//
// The "myapp/packageX" packages are expected to register HTTP handlers
// in their init functions.
func Main() {
	internal.Main()
}

// Middleware wraps an http handler so that it can make GAE API calls
var Middleware func(http.Handler) http.Handler = internal.Middleware

// IsDevAppServer reports whether the App Engine app is running in the
// development App Server.
func IsDevAppServer() bool {
	return internal.IsDevAppServer()
}

// IsStandard reports whether the App Engine app is running in the standard
// environment. This includes both the first generation runtimes (<= Go 1.9)
// and the second generation runtimes (>= Go 1.11).
func IsStandard() bool {
	return internal.IsStandard()
}

// IsFlex reports whether the App Engine app is running in the flexible environment.
func IsFlex() bool {
	return internal.IsFlex()
}

// IsAppEngine reports whether the App Engine app is running on App Engine, in either
// the standard or flexible environment.
func IsAppEngine() bool {
	return internal.IsAppEngine()
}

// IsSecondGen reports whether the App Engine app is running on the second generation
// runtimes (>= Go 1.11).
func IsSecondGen() bool {
	return internal.IsSecondGen()
}

// NewContext returns a context for an in-flight HTTP request.
// This function is cheap.
func NewContext(req *http.Request) context.Context {
	return internal.ReqContext(req)
}

// WithContext returns a copy of the parent context
// and associates it with an in-flight HTTP request.
// This function is cheap.
func WithContext(parent context.Context, req *http.Request) context.Context {
	return internal.WithContext(parent, req)
}

// BlobKey is a key for a blobstore blob.
//
// Conceptually, this type belongs in the blobstore package, but it lives in
// the appengine package to avoid a circular dependency: blobstore depends on
// datastore, and datastore needs to refer to the BlobKey type.
type BlobKey string

// GeoPoint represents a location as latitude/longitude in degrees.
type GeoPoint struct {
	Lat, Lng float64
}

// Valid returns whether a GeoPoint is within [-90, 90] latitude and [-180, 180] longitude.
func (g GeoPoint) Valid() bool {
	return -90 <= g.Lat && g.Lat <= 90 && -180 <= g.Lng && g.Lng <= 180
}

// APICallFunc defines a function type for handling an API call.
// See WithCallOverride.
type APICallFunc func(ctx context.Context, service, method string, in, out proto.Message) error

// WithAPICallFunc returns a copy of the parent context
// that will cause API calls to invoke f instead of their normal operation.
//
// This is intended for advanced users only.
func WithAPICallFunc(ctx context.Context, f APICallFunc) context.Context {
	return internal.WithCallOverride(ctx, internal.CallOverrideFunc(f))
}

// APICall performs an API call.
//
// This is not intended for general use; it is exported for use in conjunction
// with WithAPICallFunc.
func APICall(ctx context.Context, service, method string, in, out proto.Message) error {
	return internal.Call(ctx, service, method, in, out)
}
