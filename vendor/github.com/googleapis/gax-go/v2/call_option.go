// Copyright 2016, Google Inc.
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//     * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//     * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//     * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package gax

import (
	"errors"
	"math/rand"
	"time"

	"google.golang.org/api/googleapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CallOption is an option used by Invoke to control behaviors of RPC calls.
// CallOption works by modifying relevant fields of CallSettings.
type CallOption interface {
	// Resolve applies the option by modifying cs.
	Resolve(cs *CallSettings)
}

// Retryer is used by Invoke to determine retry behavior.
type Retryer interface {
	// Retry reports whether a request should be retried and how long to pause before retrying
	// if the previous attempt returned with err. Invoke never calls Retry with nil error.
	Retry(err error) (pause time.Duration, shouldRetry bool)
}

type retryerOption func() Retryer

func (o retryerOption) Resolve(s *CallSettings) {
	s.Retry = o
}

// WithRetry sets CallSettings.Retry to fn.
func WithRetry(fn func() Retryer) CallOption {
	return retryerOption(fn)
}

// OnErrorFunc returns a Retryer that retries if and only if the previous attempt
// returns an error that satisfies shouldRetry.
//
// Pause times between retries are specified by bo. bo is only used for its
// parameters; each Retryer has its own copy.
func OnErrorFunc(bo Backoff, shouldRetry func(err error) bool) Retryer {
	return &errorRetryer{
		shouldRetry: shouldRetry,
		backoff:     bo,
	}
}

type errorRetryer struct {
	backoff     Backoff
	shouldRetry func(err error) bool
}

func (r *errorRetryer) Retry(err error) (time.Duration, bool) {
	if r.shouldRetry(err) {
		return r.backoff.Pause(), true
	}

	return 0, false
}

// OnCodes returns a Retryer that retries if and only if
// the previous attempt returns a GRPC error whose error code is stored in cc.
// Pause times between retries are specified by bo.
//
// bo is only used for its parameters; each Retryer has its own copy.
func OnCodes(cc []codes.Code, bo Backoff) Retryer {
	return &boRetryer{
		backoff: bo,
		codes:   append([]codes.Code(nil), cc...),
	}
}

type boRetryer struct {
	backoff Backoff
	codes   []codes.Code
}

func (r *boRetryer) Retry(err error) (time.Duration, bool) {
	st, ok := status.FromError(err)
	if !ok {
		return 0, false
	}
	c := st.Code()
	for _, rc := range r.codes {
		if c == rc {
			return r.backoff.Pause(), true
		}
	}
	return 0, false
}

// OnHTTPCodes returns a Retryer that retries if and only if
// the previous attempt returns a googleapi.Error whose status code is stored in
// cc. Pause times between retries are specified by bo.
//
// bo is only used for its parameters; each Retryer has its own copy.
func OnHTTPCodes(bo Backoff, cc ...int) Retryer {
	codes := make(map[int]bool, len(cc))
	for _, c := range cc {
		codes[c] = true
	}

	return &httpRetryer{
		backoff: bo,
		codes:   codes,
	}
}

type httpRetryer struct {
	backoff Backoff
	codes   map[int]bool
}

func (r *httpRetryer) Retry(err error) (time.Duration, bool) {
	var gerr *googleapi.Error
	if !errors.As(err, &gerr) {
		return 0, false
	}

	if r.codes[gerr.Code] {
		return r.backoff.Pause(), true
	}

	return 0, false
}

// Backoff implements exponential backoff. The wait time between retries is a
// random value between 0 and the "retry period" - the time between retries. The
// retry period starts at Initial and increases by the factor of Multiplier
// every retry, but is capped at Max.
//
// Note: MaxNumRetries / RPCDeadline is specifically not provided. These should
// be built on top of Backoff.
type Backoff struct {
	// Initial is the initial value of the retry period, defaults to 1 second.
	Initial time.Duration

	// Max is the maximum value of the retry period, defaults to 30 seconds.
	Max time.Duration

	// Multiplier is the factor by which the retry period increases.
	// It should be greater than 1 and defaults to 2.
	Multiplier float64

	// cur is the current retry period.
	cur time.Duration
}

// Pause returns the next time.Duration that the caller should use to backoff.
func (bo *Backoff) Pause() time.Duration {
	if bo.Initial == 0 {
		bo.Initial = time.Second
	}
	if bo.cur == 0 {
		bo.cur = bo.Initial
	}
	if bo.Max == 0 {
		bo.Max = 30 * time.Second
	}
	if bo.Multiplier < 1 {
		bo.Multiplier = 2
	}
	// Select a duration between 1ns and the current max. It might seem
	// counterintuitive to have so much jitter, but
	// https://www.awsarchitectureblog.com/2015/03/backoff.html argues that
	// that is the best strategy.
	d := time.Duration(1 + rand.Int63n(int64(bo.cur)))
	bo.cur = time.Duration(float64(bo.cur) * bo.Multiplier)
	if bo.cur > bo.Max {
		bo.cur = bo.Max
	}
	return d
}

type grpcOpt []grpc.CallOption

func (o grpcOpt) Resolve(s *CallSettings) {
	s.GRPC = o
}

type pathOpt struct {
	p string
}

func (p pathOpt) Resolve(s *CallSettings) {
	s.Path = p.p
}

type timeoutOpt struct {
	t time.Duration
}

func (t timeoutOpt) Resolve(s *CallSettings) {
	s.timeout = t.t
}

// WithPath applies a Path override to the HTTP-based APICall.
//
// This is for internal use only.
func WithPath(p string) CallOption {
	return &pathOpt{p: p}
}

// WithGRPCOptions allows passing gRPC call options during client creation.
func WithGRPCOptions(opt ...grpc.CallOption) CallOption {
	return grpcOpt(append([]grpc.CallOption(nil), opt...))
}

// WithTimeout is a convenience option for setting a context.WithTimeout on the
// singular context.Context used for **all** APICall attempts. Calculated from
// the start of the first APICall attempt.
// If the context.Context provided to Invoke already has a Deadline set, that
// will always be respected over the deadline calculated using this option.
func WithTimeout(t time.Duration) CallOption {
	return &timeoutOpt{t: t}
}

// CallSettings allow fine-grained control over how calls are made.
type CallSettings struct {
	// Retry returns a Retryer to be used to control retry logic of a method call.
	// If Retry is nil or the returned Retryer is nil, the call will not be retried.
	Retry func() Retryer

	// CallOptions to be forwarded to GRPC.
	GRPC []grpc.CallOption

	// Path is an HTTP override for an APICall.
	Path string

	// Timeout defines the amount of time that Invoke has to complete.
	// Unexported so it cannot be changed by the code in an APICall.
	timeout time.Duration
}
