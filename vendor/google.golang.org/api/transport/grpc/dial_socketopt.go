// Copyright 2019 Google LLC
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

// +build go1.11,linux

package grpc

import (
	"context"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
)

const (
	// defaultTCPUserTimeout is the default TCP_USER_TIMEOUT socket option. By
	// default is 20 seconds.
	tcpUserTimeoutMilliseconds = 20000
)

func init() {
	// timeoutDialerOption is a grpc.DialOption that contains dialer with
	// socket option TCP_USER_TIMEOUT. This dialer requires go versions 1.11+.
	timeoutDialerOption = grpc.WithContextDialer(dialTCPUserTimeout)
}

func dialTCPUserTimeout(ctx context.Context, addr string) (net.Conn, error) {
	control := func(network, address string, c syscall.RawConn) error {
		var syscallErr error
		controlErr := c.Control(func(fd uintptr) {
			syscallErr = syscall.SetsockoptInt(
				int(fd), syscall.IPPROTO_TCP, unix.TCP_USER_TIMEOUT, tcpUserTimeoutMilliseconds)
		})
		if syscallErr != nil {
			return syscallErr
		}
		if controlErr != nil {
			return controlErr
		}
		return nil
	}
	d := &net.Dialer{
		Control: control,
	}
	return d.DialContext(ctx, "tcp", addr)
}
