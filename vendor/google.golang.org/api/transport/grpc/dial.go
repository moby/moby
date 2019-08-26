// Copyright 2015 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package grpc supports network connections to GRPC servers.
// This package is not intended for use by end developers. Use the
// google.golang.org/api/option package to configure API clients.
package grpc

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"

	"go.opencensus.io/plugin/ocgrpc"
	"golang.org/x/oauth2"
	"google.golang.org/api/internal"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	grpcgoogle "google.golang.org/grpc/credentials/google"
	"google.golang.org/grpc/credentials/oauth"

	// Install grpclb, which is required for direct path.
	_ "google.golang.org/grpc/balancer/grpclb"
)

// Set at init time by dial_appengine.go. If nil, we're not on App Engine.
var appengineDialerHook func(context.Context) grpc.DialOption

// Set at init time by dial_socketopt.go. If nil, socketopt is not supported.
var timeoutDialerOption grpc.DialOption

// Dial returns a GRPC connection for use communicating with a Google cloud
// service, configured with the given ClientOptions.
func Dial(ctx context.Context, opts ...option.ClientOption) (*grpc.ClientConn, error) {
	return dial(ctx, false, opts)
}

// DialInsecure returns an insecure GRPC connection for use communicating
// with fake or mock Google cloud service implementations, such as emulators.
// The connection is configured with the given ClientOptions.
func DialInsecure(ctx context.Context, opts ...option.ClientOption) (*grpc.ClientConn, error) {
	return dial(ctx, true, opts)
}

func dial(ctx context.Context, insecure bool, opts []option.ClientOption) (*grpc.ClientConn, error) {
	var o internal.DialSettings
	for _, opt := range opts {
		opt.Apply(&o)
	}
	if err := o.Validate(); err != nil {
		return nil, err
	}
	if o.HTTPClient != nil {
		return nil, errors.New("unsupported HTTP client specified")
	}
	if o.GRPCConn != nil {
		return o.GRPCConn, nil
	}
	var grpcOpts []grpc.DialOption
	if insecure {
		grpcOpts = []grpc.DialOption{grpc.WithInsecure()}
	} else if !o.NoAuth {
		if o.APIKey != "" {
			log.Print("API keys are not supported for gRPC APIs. Remove the WithAPIKey option from your client-creating call.")
		}
		creds, err := internal.Creds(ctx, &o)
		if err != nil {
			return nil, err
		}
		// Attempt Direct Path only if:
		// * The endpoint is a host:port (or dns:///host:port).
		// * Credentials are obtained via GCE metadata server, using the default
		//   service account.
		// * Opted in via GOOGLE_CLOUD_ENABLE_DIRECT_PATH environment variable.
		//   For example, GOOGLE_CLOUD_ENABLE_DIRECT_PATH=spanner,pubsub
		if isDirectPathEnabled(o.Endpoint) && isTokenSourceDirectPathCompatible(creds.TokenSource) {
			if !strings.HasPrefix(o.Endpoint, "dns:///") {
				o.Endpoint = "dns:///" + o.Endpoint
			}
			grpcOpts = []grpc.DialOption{
				grpc.WithCredentialsBundle(
					grpcgoogle.NewComputeEngineCredentials(),
				),
			}
			// TODO(cbro): add support for system parameters (quota project, request reason) via chained interceptor.
		} else {
			grpcOpts = []grpc.DialOption{
				grpc.WithPerRPCCredentials(grpcTokenSource{
					TokenSource:   oauth.TokenSource{creds.TokenSource},
					quotaProject:  o.QuotaProject,
					requestReason: o.RequestReason,
				}),
				grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, "")),
			}
		}
	}

	if appengineDialerHook != nil {
		// Use the Socket API on App Engine.
		// appengine dialer will override socketopt dialer
		grpcOpts = append(grpcOpts, appengineDialerHook(ctx))
	}

	// Add tracing, but before the other options, so that clients can override the
	// gRPC stats handler.
	// This assumes that gRPC options are processed in order, left to right.
	grpcOpts = addOCStatsHandler(grpcOpts)
	grpcOpts = append(grpcOpts, o.GRPCDialOpts...)
	if o.UserAgent != "" {
		grpcOpts = append(grpcOpts, grpc.WithUserAgent(o.UserAgent))
	}

	// TODO(weiranf): This socketopt dialer will be used by default at some
	// point when isDirectPathEnabled will default to true, we guard it by
	// the Directpath env var for now once we can introspect user defined
	// dialer (https://github.com/grpc/grpc-go/issues/2795).
	if timeoutDialerOption != nil && isDirectPathEnabled(o.Endpoint) {
		grpcOpts = append(grpcOpts, timeoutDialerOption)
	}

	return grpc.DialContext(ctx, o.Endpoint, grpcOpts...)
}

func addOCStatsHandler(opts []grpc.DialOption) []grpc.DialOption {
	return append(opts, grpc.WithStatsHandler(&ocgrpc.ClientHandler{}))
}

// grpcTokenSource supplies PerRPCCredentials from an oauth.TokenSource.
type grpcTokenSource struct {
	oauth.TokenSource

	// Additional metadata attached as headers.
	quotaProject  string
	requestReason string
}

// GetRequestMetadata gets the request metadata as a map from a grpcTokenSource.
func (ts grpcTokenSource) GetRequestMetadata(ctx context.Context, uri ...string) (
	map[string]string, error) {
	metadata, err := ts.TokenSource.GetRequestMetadata(ctx, uri...)
	if err != nil {
		return nil, err
	}

	// Attach system parameter
	if ts.quotaProject != "" {
		metadata["X-goog-user-project"] = ts.quotaProject
	}
	if ts.requestReason != "" {
		metadata["X-goog-request-reason"] = ts.requestReason
	}
	return metadata, nil
}

func isTokenSourceDirectPathCompatible(ts oauth2.TokenSource) bool {
	if ts == nil {
		return false
	}
	tok, err := ts.Token()
	if err != nil {
		return false
	}
	if tok == nil {
		return false
	}
	if source, _ := tok.Extra("oauth2.google.tokenSource").(string); source != "compute-metadata" {
		return false
	}
	if acct, _ := tok.Extra("oauth2.google.serviceAccount").(string); acct != "default" {
		return false
	}
	return true
}

func isDirectPathEnabled(endpoint string) bool {
	// Only host:port is supported, not other schemes (e.g., "tcp://" or "unix://").
	// Also don't try direct path if the user has chosen an alternate name resolver
	// (i.e., via ":///" prefix).
	//
	// TODO(cbro): once gRPC has introspectible options, check the user hasn't
	// provided a custom dialer in gRPC options.
	if strings.Contains(endpoint, "://") && !strings.HasPrefix(endpoint, "dns:///") {
		return false
	}

	// Only try direct path if the user has opted in via the environment variable.
	whitelist := strings.Split(os.Getenv("GOOGLE_CLOUD_ENABLE_DIRECT_PATH"), ",")
	for _, api := range whitelist {
		// Ignore empty string since an empty env variable splits into [""]
		if api != "" && strings.Contains(endpoint, api) {
			return true
		}
	}
	return false
}
