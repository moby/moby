// Copyright 2018, OpenCensus Authors
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

package ochttp

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

// The following client HTTP measures are supported for use in custom views.
var (
	ClientRequestCount  = stats.Int64("opencensus.io/http/client/request_count", "Number of HTTP requests started", stats.UnitDimensionless)
	ClientRequestBytes  = stats.Int64("opencensus.io/http/client/request_bytes", "HTTP request body size if set as ContentLength (uncompressed)", stats.UnitBytes)
	ClientResponseBytes = stats.Int64("opencensus.io/http/client/response_bytes", "HTTP response body size (uncompressed)", stats.UnitBytes)
	ClientLatency       = stats.Float64("opencensus.io/http/client/latency", "End-to-end latency", stats.UnitMilliseconds)
)

// The following server HTTP measures are supported for use in custom views:
var (
	ServerRequestCount  = stats.Int64("opencensus.io/http/server/request_count", "Number of HTTP requests started", stats.UnitDimensionless)
	ServerRequestBytes  = stats.Int64("opencensus.io/http/server/request_bytes", "HTTP request body size if set as ContentLength (uncompressed)", stats.UnitBytes)
	ServerResponseBytes = stats.Int64("opencensus.io/http/server/response_bytes", "HTTP response body size (uncompressed)", stats.UnitBytes)
	ServerLatency       = stats.Float64("opencensus.io/http/server/latency", "End-to-end latency", stats.UnitMilliseconds)
)

// The following tags are applied to stats recorded by this package. Host, Path
// and Method are applied to all measures. StatusCode is not applied to
// ClientRequestCount or ServerRequestCount, since it is recorded before the status is known.
var (
	// Host is the value of the HTTP Host header.
	Host, _ = tag.NewKey("http.host")

	// StatusCode is the numeric HTTP response status code,
	// or "error" if a transport error occurred and no status code was read.
	StatusCode, _ = tag.NewKey("http.status")

	// Path is the URL path (not including query string) in the request.
	Path, _ = tag.NewKey("http.path")

	// Method is the HTTP method of the request, capitalized (GET, POST, etc.).
	Method, _ = tag.NewKey("http.method")
)

// Default distributions used by views in this package.
var (
	DefaultSizeDistribution    = view.Distribution(0, 1024, 2048, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216, 67108864, 268435456, 1073741824, 4294967296)
	DefaultLatencyDistribution = view.Distribution(0, 1, 2, 3, 4, 5, 6, 8, 10, 13, 16, 20, 25, 30, 40, 50, 65, 80, 100, 130, 160, 200, 250, 300, 400, 500, 650, 800, 1000, 2000, 5000, 10000, 20000, 50000, 100000)
)

// Package ochttp provides some convenience views.
// You need to subscribe to the views for data to actually be collected.
var (
	ClientRequestCountView = &view.View{
		Name:        "opencensus.io/http/client/request_count",
		Description: "Count of HTTP requests started",
		Measure:     ClientRequestCount,
		Aggregation: view.Count(),
	}

	ClientRequestBytesView = &view.View{
		Name:        "opencensus.io/http/client/request_bytes",
		Description: "Size distribution of HTTP request body",
		Measure:     ClientRequestBytes,
		Aggregation: DefaultSizeDistribution,
	}

	ClientResponseBytesView = &view.View{
		Name:        "opencensus.io/http/client/response_bytes",
		Description: "Size distribution of HTTP response body",
		Measure:     ClientResponseBytes,
		Aggregation: DefaultSizeDistribution,
	}

	ClientLatencyView = &view.View{
		Name:        "opencensus.io/http/client/latency",
		Description: "Latency distribution of HTTP requests",
		Measure:     ClientLatency,
		Aggregation: DefaultLatencyDistribution,
	}

	ClientRequestCountByMethod = &view.View{
		Name:        "opencensus.io/http/client/request_count_by_method",
		Description: "Client request count by HTTP method",
		TagKeys:     []tag.Key{Method},
		Measure:     ClientRequestCount,
		Aggregation: view.Count(),
	}

	ClientResponseCountByStatusCode = &view.View{
		Name:        "opencensus.io/http/client/response_count_by_status_code",
		Description: "Client response count by status code",
		TagKeys:     []tag.Key{StatusCode},
		Measure:     ClientLatency,
		Aggregation: view.Count(),
	}

	ServerRequestCountView = &view.View{
		Name:        "opencensus.io/http/server/request_count",
		Description: "Count of HTTP requests started",
		Measure:     ServerRequestCount,
		Aggregation: view.Count(),
	}

	ServerRequestBytesView = &view.View{
		Name:        "opencensus.io/http/server/request_bytes",
		Description: "Size distribution of HTTP request body",
		Measure:     ServerRequestBytes,
		Aggregation: DefaultSizeDistribution,
	}

	ServerResponseBytesView = &view.View{
		Name:        "opencensus.io/http/server/response_bytes",
		Description: "Size distribution of HTTP response body",
		Measure:     ServerResponseBytes,
		Aggregation: DefaultSizeDistribution,
	}

	ServerLatencyView = &view.View{
		Name:        "opencensus.io/http/server/latency",
		Description: "Latency distribution of HTTP requests",
		Measure:     ServerLatency,
		Aggregation: DefaultLatencyDistribution,
	}

	ServerRequestCountByMethod = &view.View{
		Name:        "opencensus.io/http/server/request_count_by_method",
		Description: "Server request count by HTTP method",
		TagKeys:     []tag.Key{Method},
		Measure:     ServerRequestCount,
		Aggregation: view.Count(),
	}

	ServerResponseCountByStatusCode = &view.View{
		Name:        "opencensus.io/http/server/response_count_by_status_code",
		Description: "Server response count by status code",
		TagKeys:     []tag.Key{StatusCode},
		Measure:     ServerLatency,
		Aggregation: view.Count(),
	}
)

// DefaultClientViews are the default client views provided by this package.
var DefaultClientViews = []*view.View{
	ClientRequestCountView,
	ClientRequestBytesView,
	ClientResponseBytesView,
	ClientLatencyView,
	ClientRequestCountByMethod,
	ClientResponseCountByStatusCode,
}

// DefaultServerViews are the default server views provided by this package.
var DefaultServerViews = []*view.View{
	ServerRequestCountView,
	ServerRequestBytesView,
	ServerResponseBytesView,
	ServerLatencyView,
	ServerRequestCountByMethod,
	ServerResponseCountByStatusCode,
}
