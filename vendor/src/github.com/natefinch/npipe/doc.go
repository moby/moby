// Copyright 2013 Nate Finch. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package npipe provides a pure Go wrapper around Windows named pipes.
//
// !! Note, this package is Windows-only.  There is no code to compile on linux.
//
// Windows named pipe documentation: http://msdn.microsoft.com/en-us/library/windows/desktop/aa365780
//
// Note that the code lives at https://github.com/natefinch/npipe (v2 branch)
// but should be imported as gopkg.in/natefinch/npipe.v2 (the package name is
// still npipe).
//
// npipe provides an interface based on stdlib's net package, with Dial, Listen,
// and Accept functions, as well as associated implementations of net.Conn and
// net.Listener.  It supports rpc over the connection.
//
// Notes
//
// * Deadlines for reading/writing to the connection are only functional in Windows Vista/Server 2008 and above, due to limitations with the Windows API.
//
// * The pipes support byte mode only (no support for message mode)
//
// Examples
//
// The Dial function connects a client to a named pipe:
//   conn, err := npipe.Dial(`\\.\pipe\mypipename`)
//   if err != nil {
//   	<handle error>
//   }
//   fmt.Fprintf(conn, "Hi server!\n")
//   msg, err := bufio.NewReader(conn).ReadString('\n')
//   ...
//
// The Listen function creates servers:
//
//   ln, err := npipe.Listen(`\\.\pipe\mypipename`)
//   if err != nil {
//   	// handle error
//   }
//   for {
//   	conn, err := ln.Accept()
//   	if err != nil {
//   		// handle error
//   		continue
//   	}
//   	go handleConnection(conn)
//   }
package npipe
