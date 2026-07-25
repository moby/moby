// Copyright 2016 Google LLC
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

// Package longrunning supports Long Running Operations for the Google Cloud Libraries.
// See google.golang.org/genproto/googleapis/longrunning for its service definition.
//
// Users of the Google Cloud Libraries will typically not use this package directly.
// Instead they will call functions returning Operations and call their methods.
//
// This package is still experimental and subject to change.
package longrunning // import "cloud.google.com/go/longrunning"

import (
	"context"
	"errors"
	"fmt"
	"time"

	autogen "cloud.google.com/go/longrunning/autogen"
	pb "cloud.google.com/go/longrunning/autogen/longrunningpb"
	gax "github.com/googleapis/gax-go/v2"
	"github.com/googleapis/gax-go/v2/apierror"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
	"google.golang.org/protobuf/types/known/anypb"
)

// ErrNoMetadata is the error returned by Metadata if the operation contains no metadata.
var ErrNoMetadata = errors.New("operation contains no metadata")

// Operation represents the result of an API call that may not be ready yet.
type Operation struct {
	c               operationsClient
	proto           *pb.Operation
	opName          string
	initSpanContext trace.SpanContext
}

type operationsClient interface {
	GetOperation(context.Context, *pb.GetOperationRequest, ...gax.CallOption) (*pb.Operation, error)
	CancelOperation(context.Context, *pb.CancelOperationRequest, ...gax.CallOption) error
	DeleteOperation(context.Context, *pb.DeleteOperationRequest, ...gax.CallOption) error
}

// InternalNewOperation is for use by the Google Cloud Libraries only.
//
// InternalNewOperation returns an long-running operation, abstracting the raw pb.Operation.
func InternalNewOperation(inner *autogen.OperationsClient, proto *pb.Operation) *Operation {
	return InternalNewOperationWithMetadata(inner, proto, "")
}

// InternalNewOperationWithMetadata is for use by the Google Cloud Libraries only.
//
// InternalNewOperationWithMetadata returns a long-running operation, abstracting the raw pb.Operation,
// and storing the service-specific operation metadata/type name for tracing.
//
// InternalNewOperationWithMetadata is an EXPERIMENTAL API and may be changed or removed in the future.
func InternalNewOperationWithMetadata(inner *autogen.OperationsClient, proto *pb.Operation, opName string) *Operation {
	return &Operation{
		c:      inner,
		proto:  proto,
		opName: opName,
	}
}

// SetParentSpanContext sets the parent/initiating span context for the operation,
// which is used to create a Span Link from the LRO Wait span.
//
// SetParentSpanContext is an EXPERIMENTAL API and may be changed or removed in the future.
func (op *Operation) SetParentSpanContext(sc trace.SpanContext) {
	op.initSpanContext = sc
}

// Name returns the name of the long-running operation.
// The name is assigned by the server and is unique within the service
// from which the operation is created.
func (op *Operation) Name() string {
	return op.proto.Name
}

// Done reports whether the long-running operation has completed.
func (op *Operation) Done() bool {
	return op.proto.Done
}

// Metadata unmarshals op's metadata into meta.
// If op does not contain any metadata, Metadata returns ErrNoMetadata and meta is unmodified.
func (op *Operation) Metadata(meta protoadapt.MessageV1) error {
	if m := op.proto.Metadata; m != nil {
		metav2 := protoadapt.MessageV2Of(meta)
		return anypb.UnmarshalTo(m, metav2, proto.UnmarshalOptions{AllowPartial: true, DiscardUnknown: true})
	}
	return ErrNoMetadata
}

// Poll fetches the latest state of a long-running operation.
//
// If Poll fails, the error is returned and op is unmodified.
// If Poll succeeds and the operation has completed with failure,
// the error is returned and op.Done will return true.
// If Poll succeeds and the operation has completed successfully,
// op.Done will return true; if resp != nil, the response of the operation
// is stored in resp.
func (op *Operation) Poll(ctx context.Context, resp protoadapt.MessageV1, opts ...gax.CallOption) error {
	if !op.Done() {
		p, err := op.c.GetOperation(ctx, &pb.GetOperationRequest{Name: op.Name()}, opts...)
		if err != nil {
			return err
		}
		op.proto = p
	}
	if !op.Done() {
		return nil
	}

	switch r := op.proto.Result.(type) {
	case *pb.Operation_Error:
		err, _ := apierror.FromError(status.ErrorProto(r.Error))
		return err
	case *pb.Operation_Response:
		if resp == nil {
			return nil
		}
		respv2 := protoadapt.MessageV2Of(resp)
		return anypb.UnmarshalTo(r.Response, respv2, proto.UnmarshalOptions{AllowPartial: true, DiscardUnknown: true})
	default:
		return fmt.Errorf("unsupported result type %[1]T: %[1]v", r)
	}
}

// DefaultWaitInterval is the polling interval used by Operation.Wait.
const DefaultWaitInterval = 60 * time.Second

// Wait is equivalent to WaitWithInterval using DefaultWaitInterval.
func (op *Operation) Wait(ctx context.Context, resp protoadapt.MessageV1, opts ...gax.CallOption) error {
	return op.WaitWithInterval(ctx, resp, DefaultWaitInterval, opts...)
}

// WaitWithInterval blocks until the operation is completed.
// If resp != nil, Wait stores the response in resp.
// WaitWithInterval polls every interval, except initially
// when it polls using exponential backoff.
//
// See documentation of Poll for error-handling information.
func (op *Operation) WaitWithInterval(ctx context.Context, resp protoadapt.MessageV1, interval time.Duration, opts ...gax.CallOption) error {
	return op.waitWithInterval(ctx, resp, interval, gax.Sleep, opts...)
}

func (op *Operation) waitWithInterval(ctx context.Context, resp protoadapt.MessageV1, interval time.Duration, sl sleeper, opts ...gax.CallOption) error {
	bo := gax.Backoff{
		Initial: 1 * time.Second,
		Max:     interval,
	}
	if bo.Max < bo.Initial {
		bo.Max = bo.Initial
	}

	if !gax.IsFeatureEnabled("TRACING") {
		return op.wait(ctx, resp, &bo, sl, opts...)
	}

	spanName := op.opName
	if spanName == "" {
		spanName = "*longrunning.Operation.Wait"
	} else {
		spanName = spanName + ".Wait"
	}

	var startOpts []trace.SpanStartOption
	if op.initSpanContext.IsValid() {
		startOpts = append(startOpts, trace.WithLinks(trace.Link{SpanContext: op.initSpanContext}))
	}

	tracer := otel.GetTracerProvider().Tracer("cloud.google.com/go")
	ctx, span := tracer.Start(ctx, spanName, startOpts...)
	defer span.End()
	span.SetAttributes(
		attribute.String("gcp.resource.destination.id", op.Name()),
	)

	err := op.waitTraced(ctx, resp, &bo, sl, opts...)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}
	return err
}

type sleeper func(context.Context, time.Duration) error

// wait implements Wait, taking exponentialBackoff and sleeper arguments for testing.
func (op *Operation) wait(ctx context.Context, resp protoadapt.MessageV1, bo *gax.Backoff, sl sleeper, opts ...gax.CallOption) error {
	for {
		if err := op.Poll(ctx, resp, opts...); err != nil {
			return err
		}
		if op.Done() {
			return nil
		}
		if err := sl(ctx, bo.Pause()); err != nil {
			return err
		}
	}
}

func (op *Operation) waitTraced(ctx context.Context, resp protoadapt.MessageV1, bo *gax.Backoff, sl sleeper, opts ...gax.CallOption) error {
	tracer := otel.GetTracerProvider().Tracer("cloud.google.com/go")
	pollAttempt := 0

	for {
		pollAttempt++
		pollSpanName := "*longrunning.OperationsClient.GetOperation"

		pollCtx, pollSpan := tracer.Start(ctx, pollSpanName)
		pollSpan.SetAttributes(
			attribute.String("gcp.resource.destination.id", op.Name()),
			attribute.Int("gcp.longrunning.poll_attempt_count", pollAttempt),
		)

		err := op.Poll(pollCtx, resp, opts...)

		pollSpan.SetAttributes(attribute.Bool("gcp.longrunning.done", op.Done()))
		if op.Done() {
			var statusCode int
			if err != nil {
				if apiErr, ok := apierror.FromError(err); ok {
					statusCode = int(apiErr.GRPCStatus().Code())
				} else {
					statusCode = int(grpccodes.Unknown)
				}
			}
			pollSpan.SetAttributes(attribute.Int("gcp.longrunning.status_code", statusCode))
		}

		if err != nil {
			pollSpan.SetStatus(codes.Error, err.Error())
			pollSpan.RecordError(err)
			pollSpan.End()
			return err
		}

		pollSpan.End()

		if op.Done() {
			return nil
		}

		_, sleepSpan := tracer.Start(ctx, "LRO Sleep")
		sleepErr := sl(ctx, bo.Pause())
		if sleepErr != nil {
			sleepSpan.SetStatus(codes.Error, sleepErr.Error())
			sleepSpan.RecordError(sleepErr)
			sleepSpan.End()
			return sleepErr
		}
		sleepSpan.End()
	}
}

// Cancel starts asynchronous cancellation on a long-running operation. The server
// makes a best effort to cancel the operation, but success is not
// guaranteed. If the server doesn't support this method, it returns
// status.Code(err) == codes.Unimplemented. Clients can use
// Poll or other methods to check whether the cancellation succeeded or whether the
// operation completed despite cancellation. On successful cancellation,
// the operation is not deleted; instead, op.Poll returns an error
// with code Canceled.
func (op *Operation) Cancel(ctx context.Context, opts ...gax.CallOption) error {
	return op.c.CancelOperation(ctx, &pb.CancelOperationRequest{Name: op.Name()}, opts...)
}

// Delete deletes a long-running operation. This method indicates that the client is
// no longer interested in the operation result. It does not cancel the
// operation. If the server doesn't support this method, status.Code(err) == codes.Unimplemented.
func (op *Operation) Delete(ctx context.Context, opts ...gax.CallOption) error {
	return op.c.DeleteOperation(ctx, &pb.DeleteOperationRequest{Name: op.Name()}, opts...)
}
