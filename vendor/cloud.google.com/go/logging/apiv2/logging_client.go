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
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	loggingpb "google.golang.org/genproto/googleapis/logging/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

var (
	loggingParentPathTemplate = gax.MustCompilePathTemplate("projects/{project}")
	loggingLogPathTemplate    = gax.MustCompilePathTemplate("projects/{project}/logs/{log}")
)

// CallOptions contains the retry settings for each method of Client.
type CallOptions struct {
	DeleteLog                        []gax.CallOption
	WriteLogEntries                  []gax.CallOption
	ListLogEntries                   []gax.CallOption
	ListMonitoredResourceDescriptors []gax.CallOption
}

func defaultClientOptions() []option.ClientOption {
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

func defaultCallOptions() *CallOptions {
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
		{"list", "idempotent"}: {
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
	return &CallOptions{
		DeleteLog:                        retry[[2]string{"default", "idempotent"}],
		WriteLogEntries:                  retry[[2]string{"default", "non_idempotent"}],
		ListLogEntries:                   retry[[2]string{"list", "idempotent"}],
		ListMonitoredResourceDescriptors: retry[[2]string{"default", "idempotent"}],
	}
}

// Client is a client for interacting with Stackdriver Logging API.
type Client struct {
	// The connection to the service.
	conn *grpc.ClientConn

	// The gRPC API client.
	client loggingpb.LoggingServiceV2Client

	// The call options for this service.
	CallOptions *CallOptions

	// The metadata to be sent with each request.
	metadata metadata.MD
}

// NewClient creates a new logging service v2 client.
//
// Service for ingesting and querying logs.
func NewClient(ctx context.Context, opts ...option.ClientOption) (*Client, error) {
	conn, err := transport.DialGRPC(ctx, append(defaultClientOptions(), opts...)...)
	if err != nil {
		return nil, err
	}
	c := &Client{
		conn:        conn,
		CallOptions: defaultCallOptions(),

		client: loggingpb.NewLoggingServiceV2Client(conn),
	}
	c.SetGoogleClientInfo("gax", gax.Version)
	return c, nil
}

// Connection returns the client's connection to the API service.
func (c *Client) Connection() *grpc.ClientConn {
	return c.conn
}

// Close closes the connection to the API service. The user should invoke this when
// the client is no longer required.
func (c *Client) Close() error {
	return c.conn.Close()
}

// SetGoogleClientInfo sets the name and version of the application in
// the `x-goog-api-client` header passed on each request. Intended for
// use by Google-written clients.
func (c *Client) SetGoogleClientInfo(name, version string) {
	goVersion := strings.Replace(runtime.Version(), " ", "_", -1)
	v := fmt.Sprintf("%s/%s %s gax/%s go/%s", name, version, gapicNameVersion, gax.Version, goVersion)
	c.metadata = metadata.Pairs("x-goog-api-client", v)
}

// LoggingParentPath returns the path for the parent resource.
func LoggingParentPath(project string) string {
	path, err := loggingParentPathTemplate.Render(map[string]string{
		"project": project,
	})
	if err != nil {
		panic(err)
	}
	return path
}

// LoggingLogPath returns the path for the log resource.
func LoggingLogPath(project, log string) string {
	path, err := loggingLogPathTemplate.Render(map[string]string{
		"project": project,
		"log":     log,
	})
	if err != nil {
		panic(err)
	}
	return path
}

// DeleteLog deletes all the log entries in a log.
// The log reappears if it receives new entries.
func (c *Client) DeleteLog(ctx context.Context, req *loggingpb.DeleteLogRequest) error {
	md, _ := metadata.FromContext(ctx)
	ctx = metadata.NewContext(ctx, metadata.Join(md, c.metadata))
	err := gax.Invoke(ctx, func(ctx context.Context) error {
		var err error
		_, err = c.client.DeleteLog(ctx, req)
		return err
	}, c.CallOptions.DeleteLog...)
	return err
}

// WriteLogEntries writes log entries to Stackdriver Logging.  All log entries are
// written by this method.
func (c *Client) WriteLogEntries(ctx context.Context, req *loggingpb.WriteLogEntriesRequest) (*loggingpb.WriteLogEntriesResponse, error) {
	md, _ := metadata.FromContext(ctx)
	ctx = metadata.NewContext(ctx, metadata.Join(md, c.metadata))
	var resp *loggingpb.WriteLogEntriesResponse
	err := gax.Invoke(ctx, func(ctx context.Context) error {
		var err error
		resp, err = c.client.WriteLogEntries(ctx, req)
		return err
	}, c.CallOptions.WriteLogEntries...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ListLogEntries lists log entries.  Use this method to retrieve log entries from Cloud
// Logging.  For ways to export log entries, see
// [Exporting Logs](/logging/docs/export).
func (c *Client) ListLogEntries(ctx context.Context, req *loggingpb.ListLogEntriesRequest) *LogEntryIterator {
	md, _ := metadata.FromContext(ctx)
	ctx = metadata.NewContext(ctx, metadata.Join(md, c.metadata))
	it := &LogEntryIterator{}
	it.InternalFetch = func(pageSize int, pageToken string) ([]*loggingpb.LogEntry, string, error) {
		var resp *loggingpb.ListLogEntriesResponse
		req.PageToken = pageToken
		if pageSize > math.MaxInt32 {
			req.PageSize = math.MaxInt32
		} else {
			req.PageSize = int32(pageSize)
		}
		err := gax.Invoke(ctx, func(ctx context.Context) error {
			var err error
			resp, err = c.client.ListLogEntries(ctx, req)
			return err
		}, c.CallOptions.ListLogEntries...)
		if err != nil {
			return nil, "", err
		}
		return resp.Entries, resp.NextPageToken, nil
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

// ListMonitoredResourceDescriptors lists the monitored resource descriptors used by Stackdriver Logging.
func (c *Client) ListMonitoredResourceDescriptors(ctx context.Context, req *loggingpb.ListMonitoredResourceDescriptorsRequest) *MonitoredResourceDescriptorIterator {
	md, _ := metadata.FromContext(ctx)
	ctx = metadata.NewContext(ctx, metadata.Join(md, c.metadata))
	it := &MonitoredResourceDescriptorIterator{}
	it.InternalFetch = func(pageSize int, pageToken string) ([]*monitoredrespb.MonitoredResourceDescriptor, string, error) {
		var resp *loggingpb.ListMonitoredResourceDescriptorsResponse
		req.PageToken = pageToken
		if pageSize > math.MaxInt32 {
			req.PageSize = math.MaxInt32
		} else {
			req.PageSize = int32(pageSize)
		}
		err := gax.Invoke(ctx, func(ctx context.Context) error {
			var err error
			resp, err = c.client.ListMonitoredResourceDescriptors(ctx, req)
			return err
		}, c.CallOptions.ListMonitoredResourceDescriptors...)
		if err != nil {
			return nil, "", err
		}
		return resp.ResourceDescriptors, resp.NextPageToken, nil
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

// LogEntryIterator manages a stream of *loggingpb.LogEntry.
type LogEntryIterator struct {
	items    []*loggingpb.LogEntry
	pageInfo *iterator.PageInfo
	nextFunc func() error

	// InternalFetch is for use by the Google Cloud Libraries only.
	// It is not part of the stable interface of this package.
	//
	// InternalFetch returns results from a single call to the underlying RPC.
	// The number of results is no greater than pageSize.
	// If there are no more results, nextPageToken is empty and err is nil.
	InternalFetch func(pageSize int, pageToken string) (results []*loggingpb.LogEntry, nextPageToken string, err error)
}

// PageInfo supports pagination. See the google.golang.org/api/iterator package for details.
func (it *LogEntryIterator) PageInfo() *iterator.PageInfo {
	return it.pageInfo
}

// Next returns the next result. Its second return value is iterator.Done if there are no more
// results. Once Next returns Done, all subsequent calls will return Done.
func (it *LogEntryIterator) Next() (*loggingpb.LogEntry, error) {
	var item *loggingpb.LogEntry
	if err := it.nextFunc(); err != nil {
		return item, err
	}
	item = it.items[0]
	it.items = it.items[1:]
	return item, nil
}

func (it *LogEntryIterator) bufLen() int {
	return len(it.items)
}

func (it *LogEntryIterator) takeBuf() interface{} {
	b := it.items
	it.items = nil
	return b
}

// MonitoredResourceDescriptorIterator manages a stream of *monitoredrespb.MonitoredResourceDescriptor.
type MonitoredResourceDescriptorIterator struct {
	items    []*monitoredrespb.MonitoredResourceDescriptor
	pageInfo *iterator.PageInfo
	nextFunc func() error

	// InternalFetch is for use by the Google Cloud Libraries only.
	// It is not part of the stable interface of this package.
	//
	// InternalFetch returns results from a single call to the underlying RPC.
	// The number of results is no greater than pageSize.
	// If there are no more results, nextPageToken is empty and err is nil.
	InternalFetch func(pageSize int, pageToken string) (results []*monitoredrespb.MonitoredResourceDescriptor, nextPageToken string, err error)
}

// PageInfo supports pagination. See the google.golang.org/api/iterator package for details.
func (it *MonitoredResourceDescriptorIterator) PageInfo() *iterator.PageInfo {
	return it.pageInfo
}

// Next returns the next result. Its second return value is iterator.Done if there are no more
// results. Once Next returns Done, all subsequent calls will return Done.
func (it *MonitoredResourceDescriptorIterator) Next() (*monitoredrespb.MonitoredResourceDescriptor, error) {
	var item *monitoredrespb.MonitoredResourceDescriptor
	if err := it.nextFunc(); err != nil {
		return item, err
	}
	item = it.items[0]
	it.items = it.items[1:]
	return item, nil
}

func (it *MonitoredResourceDescriptorIterator) bufLen() int {
	return len(it.items)
}

func (it *MonitoredResourceDescriptorIterator) takeBuf() interface{} {
	b := it.items
	it.items = nil
	return b
}
