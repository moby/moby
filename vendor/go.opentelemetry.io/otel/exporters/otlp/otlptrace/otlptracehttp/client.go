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

package otlptracehttp // import "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp/internal"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp/internal/otlpconfig"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp/internal/retry"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

const contentTypeProto = "application/x-protobuf"

var gzPool = sync.Pool{
	New: func() interface{} {
		w := gzip.NewWriter(io.Discard)
		return w
	},
}

// Keep it in sync with golang's DefaultTransport from net/http! We
// have our own copy to avoid handling a situation where the
// DefaultTransport is overwritten with some different implementation
// of http.RoundTripper or it's modified by other package.
var ourTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

type client struct {
	name        string
	cfg         otlpconfig.SignalConfig
	generalCfg  otlpconfig.Config
	requestFunc retry.RequestFunc
	client      *http.Client
	stopCh      chan struct{}
	stopOnce    sync.Once
}

var _ otlptrace.Client = (*client)(nil)

// NewClient creates a new HTTP trace client.
func NewClient(opts ...Option) otlptrace.Client {
	cfg := otlpconfig.NewHTTPConfig(asHTTPOptions(opts)...)

	httpClient := &http.Client{
		Transport: ourTransport,
		Timeout:   cfg.Traces.Timeout,
	}
	if cfg.Traces.TLSCfg != nil {
		transport := ourTransport.Clone()
		transport.TLSClientConfig = cfg.Traces.TLSCfg
		httpClient.Transport = transport
	}

	stopCh := make(chan struct{})
	return &client{
		name:        "traces",
		cfg:         cfg.Traces,
		generalCfg:  cfg,
		requestFunc: cfg.RetryConfig.RequestFunc(evaluate),
		stopCh:      stopCh,
		client:      httpClient,
	}
}

// Start does nothing in a HTTP client.
func (d *client) Start(ctx context.Context) error {
	// nothing to do
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return nil
}

// Stop shuts down the client and interrupt any in-flight request.
func (d *client) Stop(ctx context.Context) error {
	d.stopOnce.Do(func() {
		close(d.stopCh)
	})
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return nil
}

// UploadTraces sends a batch of spans to the collector.
func (d *client) UploadTraces(ctx context.Context, protoSpans []*tracepb.ResourceSpans) error {
	pbRequest := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: protoSpans,
	}
	rawRequest, err := proto.Marshal(pbRequest)
	if err != nil {
		return err
	}

	ctx, cancel := d.contextWithStop(ctx)
	defer cancel()

	request, err := d.newRequest(rawRequest)
	if err != nil {
		return err
	}

	return d.requestFunc(ctx, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		request.reset(ctx)
		resp, err := d.client.Do(request.Request)
		if err != nil {
			return err
		}

		if resp != nil && resp.Body != nil {
			defer func() {
				if err := resp.Body.Close(); err != nil {
					otel.Handle(err)
				}
			}()
		}

		switch sc := resp.StatusCode; {
		case sc >= 200 && sc <= 299:
			// Success, do not retry.
			// Read the partial success message, if any.
			var respData bytes.Buffer
			if _, err := io.Copy(&respData, resp.Body); err != nil {
				return err
			}

			if respData.Len() != 0 {
				var respProto coltracepb.ExportTraceServiceResponse
				if err := proto.Unmarshal(respData.Bytes(), &respProto); err != nil {
					return err
				}

				if respProto.PartialSuccess != nil {
					msg := respProto.PartialSuccess.GetErrorMessage()
					n := respProto.PartialSuccess.GetRejectedSpans()
					if n != 0 || msg != "" {
						err := internal.TracePartialSuccessError(n, msg)
						otel.Handle(err)
					}
				}
			}
			return nil

		case sc == http.StatusTooManyRequests, sc == http.StatusServiceUnavailable:
			// Retry-able failures.  Drain the body to reuse the connection.
			if _, err := io.Copy(io.Discard, resp.Body); err != nil {
				otel.Handle(err)
			}
			return newResponseError(resp.Header)
		default:
			return fmt.Errorf("failed to send to %s: %s", request.URL, resp.Status)
		}
	})
}

func (d *client) newRequest(body []byte) (request, error) {
	u := url.URL{Scheme: d.getScheme(), Host: d.cfg.Endpoint, Path: d.cfg.URLPath}
	r, err := http.NewRequest(http.MethodPost, u.String(), nil)
	if err != nil {
		return request{Request: r}, err
	}

	userAgent := "OTel OTLP Exporter Go/" + otlptrace.Version()
	r.Header.Set("User-Agent", userAgent)

	for k, v := range d.cfg.Headers {
		r.Header.Set(k, v)
	}
	r.Header.Set("Content-Type", contentTypeProto)

	req := request{Request: r}
	switch Compression(d.cfg.Compression) {
	case NoCompression:
		r.ContentLength = (int64)(len(body))
		req.bodyReader = bodyReader(body)
	case GzipCompression:
		// Ensure the content length is not used.
		r.ContentLength = -1
		r.Header.Set("Content-Encoding", "gzip")

		gz := gzPool.Get().(*gzip.Writer)
		defer gzPool.Put(gz)

		var b bytes.Buffer
		gz.Reset(&b)

		if _, err := gz.Write(body); err != nil {
			return req, err
		}
		// Close needs to be called to ensure body if fully written.
		if err := gz.Close(); err != nil {
			return req, err
		}

		req.bodyReader = bodyReader(b.Bytes())
	}

	return req, nil
}

// MarshalLog is the marshaling function used by the logging system to represent this Client.
func (d *client) MarshalLog() interface{} {
	return struct {
		Type     string
		Endpoint string
		Insecure bool
	}{
		Type:     "otlphttphttp",
		Endpoint: d.cfg.Endpoint,
		Insecure: d.cfg.Insecure,
	}
}

// bodyReader returns a closure returning a new reader for buf.
func bodyReader(buf []byte) func() io.ReadCloser {
	return func() io.ReadCloser {
		return io.NopCloser(bytes.NewReader(buf))
	}
}

// request wraps an http.Request with a resettable body reader.
type request struct {
	*http.Request

	// bodyReader allows the same body to be used for multiple requests.
	bodyReader func() io.ReadCloser
}

// reset reinitializes the request Body and uses ctx for the request.
func (r *request) reset(ctx context.Context) {
	r.Body = r.bodyReader()
	r.Request = r.Request.WithContext(ctx)
}

// retryableError represents a request failure that can be retried.
type retryableError struct {
	throttle int64
}

// newResponseError returns a retryableError and will extract any explicit
// throttle delay contained in headers.
func newResponseError(header http.Header) error {
	var rErr retryableError
	if s, ok := header["Retry-After"]; ok {
		if t, err := strconv.ParseInt(s[0], 10, 64); err == nil {
			rErr.throttle = t
		}
	}
	return rErr
}

func (e retryableError) Error() string {
	return "retry-able request failure"
}

// evaluate returns if err is retry-able. If it is and it includes an explicit
// throttling delay, that delay is also returned.
func evaluate(err error) (bool, time.Duration) {
	if err == nil {
		return false, 0
	}

	rErr, ok := err.(retryableError)
	if !ok {
		return false, 0
	}

	return true, time.Duration(rErr.throttle)
}

func (d *client) getScheme() string {
	if d.cfg.Insecure {
		return "http"
	}
	return "https"
}

func (d *client) contextWithStop(ctx context.Context) (context.Context, context.CancelFunc) {
	// Unify the parent context Done signal with the client's stop
	// channel.
	ctx, cancel := context.WithCancel(ctx)
	go func(ctx context.Context, cancel context.CancelFunc) {
		select {
		case <-ctx.Done():
			// Nothing to do, either cancelled or deadline
			// happened.
		case <-d.stopCh:
			cancel()
		}
	}(ctx, cancel)
	return ctx, cancel
}
