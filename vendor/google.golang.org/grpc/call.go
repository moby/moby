/*
 *
 * Copyright 2014 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package grpc

import (
	"golang.org/x/net/context"
)

// Invoke sends the RPC request on the wire and returns after response is
// received.  This is typically called by generated code.
//
// All errors returned by Invoke are compatible with the status package.
func (cc *ClientConn) Invoke(ctx context.Context, method string, args, reply interface{}, opts ...CallOption) error {
	if cc.dopts.unaryInt != nil {
		return cc.dopts.unaryInt(ctx, method, args, reply, cc, invoke, opts...)
	}
	return invoke(ctx, method, args, reply, cc, opts...)
}

// Invoke sends the RPC request on the wire and returns after response is
// received.  This is typically called by generated code.
//
// DEPRECATED: Use ClientConn.Invoke instead.
func Invoke(ctx context.Context, method string, args, reply interface{}, cc *ClientConn, opts ...CallOption) error {
	return cc.Invoke(ctx, method, args, reply, opts...)
}

var unaryStreamDesc = &StreamDesc{ServerStreams: false, ClientStreams: false}

func invoke(ctx context.Context, method string, req, reply interface{}, cc *ClientConn, opts ...CallOption) error {
	// TODO: implement retries in clientStream and make this simply
	// newClientStream, SendMsg, RecvMsg.
	firstAttempt := true
	for {
		csInt, err := newClientStream(ctx, unaryStreamDesc, cc, method, opts...)
		if err != nil {
			return err
		}
		cs := csInt.(*clientStream)
		if err := cs.SendMsg(req); err != nil {
			if !cs.c.failFast && cs.s.Unprocessed() && firstAttempt {
				// TODO: Add a field to header for grpc-transparent-retry-attempts
				firstAttempt = false
				continue
			}
			return err
		}
		if err := cs.RecvMsg(reply); err != nil {
			if !cs.c.failFast && cs.s.Unprocessed() && firstAttempt {
				// TODO: Add a field to header for grpc-transparent-retry-attempts
				firstAttempt = false
				continue
			}
			return err
		}
		return nil
	}
}
