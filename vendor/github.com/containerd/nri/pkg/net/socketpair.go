/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package net

import (
	"fmt"
	"net"
	"os"
)

// SocketPair contains the os.File of a connected pair of sockets.
type SocketPair struct {
	local, peer *os.File
}

// NewSocketPair returns a connected pair of sockets.
func NewSocketPair() (SocketPair, error) {
	fds, err := newSocketPairCLOEXEC()
	if err != nil {
		return SocketPair{nil, nil}, fmt.Errorf("failed to create socketpair: %w", err)
	}

	filename := fmt.Sprintf("socketpair-#%d:%d", fds[0], fds[1])

	return SocketPair{
		os.NewFile(uintptr(fds[0]), filename+"[0]"),
		os.NewFile(uintptr(fds[1]), filename+"[1]"),
	}, nil
}

// LocalFile returns the local end of the socketpair as an *os.File.
func (sp SocketPair) LocalFile() *os.File {
	return sp.local
}

// PeerFile returns the peer end of the socketpair as an *os.File.
func (sp SocketPair) PeerFile() *os.File {
	return sp.peer
}

// LocalConn returns a net.Conn for the local end of the socketpair.
// This closes LocalFile().
func (sp SocketPair) LocalConn() (net.Conn, error) {
	file := sp.LocalFile()
	defer file.Close()
	conn, err := net.FileConn(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create net.Conn for %s: %w", file.Name(), err)
	}
	return conn, nil
}

// PeerConn returns a net.Conn for the peer end of the socketpair.
// This closes PeerFile().
func (sp SocketPair) PeerConn() (net.Conn, error) {
	file := sp.PeerFile()
	defer file.Close()
	conn, err := net.FileConn(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create net.Conn for %s: %w", file.Name(), err)
	}
	return conn, nil
}

// Close closes both ends of the socketpair.
func (sp SocketPair) Close() {
	sp.LocalClose()
	sp.PeerClose()
}

// LocalClose closes the local end of the socketpair.
func (sp SocketPair) LocalClose() {
	sp.local.Close()
}

// PeerClose closes the peer end of the socketpair.
func (sp SocketPair) PeerClose() {
	sp.peer.Close()
}
