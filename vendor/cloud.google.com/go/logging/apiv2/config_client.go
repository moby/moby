// Copyright 2016, Google Inc. All rights reserved.
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

// AUTO-GENERATED CODE. DO NOT EDIT.

package logging

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"time"

	gax "github.com/googleapis/gax-go"
	"golang.org/x/net/context"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
	loggingpb "google.golang.org/genproto/googleapis/logging/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

var (
	configParentPathTemplate = gax.MustCompilePathTemplate("projects/{project}")
	configSinkPathTemplate   = gax.MustCompilePathTemplate("projects/{project}/sinks/{sink}")
)

// ConfigCallOptions contains the retry settings for each method of ConfigClient.
type ConfigCallOptions struct {
	ListSinks  []gax.CallOption
	GetSink    []gax.CallOption
	CreateSink []gax.CallOption
	UpdateSink []gax.CallOption
	DeleteSink []gax.CallOption
}

func defaultConfigClientOptions() []option.ClientOption {
	return []option.ClientOption{
		option.WithEndpoint("logging.googleapis.com:443"),
		option.WithScopes(
			"https://www.googleapis.com/auth/cloud-platform",
			"https://www.googleapis.com/auth/cloud-platform.read-only",
			"https://www.googleapis.com/auth/logging.admin",
			"https://www.googleapis.com/auth/logging.read",
			"https://www.googleapis.com/auth/logging.write",
		),
	}
}

func defaultConfigCallOptions() *ConfigCallOptions {
	retry := map[[2]string][]gax.CallOption{
		{"default", "idempotent"}: {
			gax.WithRetry(func() gax.Retryer {
				return gax.OnCodes([]codes.Code{
					codes.DeadlineExceeded,
					codes.Unavailable,
				}, gax.Backoff{
					Initial:    100 * time.Millisecond,
					Max:        1000 * time.Millisecond,
					Multiplier: 1.2,
				})
			}),
		},
	}
	return &ConfigCallOptions{
		ListSinks:  retry[[2]string{"default", "idempotent"}],
		GetSink:    retry[[2]string{"default", "idempotent"}],
		CreateSink: retry[[2]string{"default", "non_idempotent"}],
		UpdateSink: retry[[2]string{"default", "non_idempotent"}],
		DeleteSink: retry[[2]string{"default", "idempotent"}],
	}
}

// ConfigClient is a client for interacting with Stackdriver Logging API.
type ConfigClient struct {
	// The connection to the service.
	conn *grpc.ClientConn

	// The gRPC API client.
	configClient loggingpb.ConfigServiceV2Client

	// The call options for this service.
	CallOptions *ConfigCallOptions

	// The metadata to be sent with each request.
	metadata metadata.MD
}

// NewConfigClient creates a new config service v2 client.
//
// Service for configuring sinks used to export log entries outside of
// Stackdriver Logging.
func NewConfigClient(ctx context.Context, opts ...option.ClientOption) (*ConfigClient, error) {
	conn, err := transport.DialGRPC(ctx, append(defaultConfigClientOptions(), opts...)...)
	if err != nil {
		return nil, err
	}
	c := &ConfigClient{
		conn:        conn,
		CallOptions: defaultConfigCallOptions(),

		configClient: loggingpb.NewConfigServiceV2Client(conn),
	}
	c.SetGoogleClientInfo("gax", gax.Version)
	return c, nil
}

// Connection returns the client's connection to the API service.
func (c *ConfigClient) Connection() *grpc.ClientConn {
	return c.conn
}

// Close closes the connection to the API service. The user should invoke this when
// the client is no longer required.
func (c *ConfigClient) Close() error {
	return c.conn.Close()
}

// SetGoogleClientInfo sets the name and version of the application in
// the `x-goog-api-client` header passed on each request. Intended for
// use by Google-written clients.
func (c *ConfigClient) SetGoogleClientInfo(name, version string) {
	goVersion := strings.Replace(runtime.Version(), " ", "_", -1)
	v := fmt.Sprintf("%s/%s %s gax/%s go/%s", name, version, gapicNameVersion, gax.Version, goVersion)
	c.metadata = metadata.Pairs("x-goog-api-client", v)
}

// ConfigParentPath returns the path for the parent resource.
func ConfigParentPath(project string) string {
	path, err := configParentPathTemplate.Render(map[string]string{
		"project": project,
	})
	if err != nil {
		panic(err)
	}
	return path
}

// ConfigSinkPath returns the path for the sink resource.
func ConfigSinkPath(project, sink string) string {
	path, err := configSinkPathTemplate.Render(map[string]string{
		"project": project,
		"sink":    sink,
	})
	if err != nil {
		panic(err)
	}
	return path
}

// ListSinks lists sinks.
func (c *ConfigClient) ListSinks(ctx context.Context, req *loggingpb.ListSinksRequest) *LogSinkIterator {
	md, _ := metadata.FromContext(ctx)
	ctx = metadata.NewContext(ctx, metadata.Join(md, c.metadata))
	it := &LogSinkIterator{}
	it.InternalFetch = func(pageSize int, pageToken string) ([]*loggingpb.LogSink, string, error) {
		var resp *loggingpb.ListSinksResponse
		req.PageToken = pageToken
		if pageSize > math.MaxInt32 {
			req.PageSize = math.MaxInt32
		} else {
			req.PageSize = int32(pageSize)
		}
		err := gax.Invoke(ctx, func(ctx context.Context) error {
			var err error
			resp, err = c.configClient.ListSinks(ctx, req)
			return err
		}, c.CallOptions.ListSinks...)
		if err != nil {
			return nil, "", err
		}
		return resp.Sinks, resp.NextPageToken, nil
	}
	fetch := func(pageSize int, pageToken string) (string, error) {
		items, nextPageToken, err := it.InternalFetch(pageSize, pageToken)
		if err != nil {
			return "", err
		}
		it.items = append(it.items, items...)
		return nextPageToken, nil
	}
	it.pageInfo, it.nextFunc = iterator.NewPageInfo(fetch, it.bufLen, it.takeBuf)
	return it
}

// GetSink gets a sink.
func (c *ConfigClient) GetSink(ctx context.Context, req *loggingpb.GetSinkRequest) (*loggingpb.LogSink, error) {
	md, _ := metadata.FromContext(ctx)
	ctx = metadata.NewContext(ctx, metadata.Join(md, c.metadata))
	var resp *loggingpb.LogSink
	err := gax.Invoke(ctx, func(ctx context.Context) error {
		var err error
		resp, err = c.configClient.GetSink(ctx, req)
		return err
	}, c.CallOptions.GetSink...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// CreateSink creates a sink.
func (c *ConfigClient) CreateSink(ctx context.Context, req *loggingpb.CreateSinkRequest) (*loggingpb.LogSink, error) {
	md, _ := metadata.FromContext(ctx)
	ctx = metadata.NewContext(ctx, metadata.Join(md, c.metadata))
	var resp *loggingpb.LogSink
	err := gax.Invoke(ctx, func(ctx context.Context) error {
		var err error
		resp, err = c.configClient.CreateSink(ctx, req)
		return err
	}, c.CallOptions.CreateSink...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// UpdateSink updates or creates a sink.
func (c *ConfigClient) UpdateSink(ctx context.Context, req *loggingpb.UpdateSinkRequest) (*loggingpb.LogSink, error) {
	md, _ := metadata.FromContext(ctx)
	ctx = metadata.NewContext(ctx, metadata.Join(md, c.metadata))
	var resp *loggingpb.LogSink
	err := gax.Invoke(ctx, func(ctx context.Context) error {
		var err error
		resp, err = c.configClient.UpdateSink(ctx, req)
		return err
	}, c.CallOptions.UpdateSink...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// DeleteSink deletes a sink.
func (c *ConfigClient) DeleteSink(ctx context.Context, req *loggingpb.DeleteSinkRequest) error {
	md, _ := metadata.FromContext(ctx)
	ctx = metadata.NewContext(ctx, metadata.Join(md, c.metadata))
	err := gax.Invoke(ctx, func(ctx context.Context) error {
		var err error
		_, err = c.configClient.DeleteSink(ctx, req)
		return err
	}, c.CallOptions.DeleteSink...)
	return err
}

// LogSinkIterator manages a stream of *loggingpb.LogSink.
type LogSinkIterator struct {
	items    []*loggingpb.LogSink
	pageInfo *iterator.PageInfo
	nextFunc func() error

	// InternalFetch is for use by the Google Cloud Libraries only.
	// It is not part of the stable interface of this package.
	//
	// InternalFetch returns results from a single call to the underlying RPC.
	// The number of results is no greater than pageSize.
	// If there are no more results, nextPageToken is empty and err is nil.
	InternalFetch func(pageSize int, pageToken string) (results []*loggingpb.LogSink, nextPageToken string, err error)
}

// PageInfo supports pagination. See the google.golang.org/api/iterator package for details.
func (it *LogSinkIterator) PageInfo() *iterator.PageInfo {
	return it.pageInfo
}

// Next returns the next result. Its second return value is iterator.Done if there are no more
// results. Once Next returns Done, all subsequent calls will return Done.
func (it *LogSinkIterator) Next() (*loggingpb.LogSink, error) {
	var item *loggingpb.LogSink
	if err := it.nextFunc(); err != nil {
		return item, err
	}
	item = it.items[0]
	it.items = it.items[1:]
	return item, nil
}

func (it *LogSinkIterator) bufLen() int {
	return len(it.items)
}

func (it *LogSinkIterator) takeBuf() interface{} {
	b := it.items
	it.items = nil
	return b
}
