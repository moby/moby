// Copyright (C) 2014 Yasuhiro Matsumoto <mattn.jp@gmail.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
// +build libsqlite3

package sqlite3

/*
#cgo CFLAGS: -DUSE_LIBSQLITE3
#cgo LDFLAGS: -lsqlite3
*/
import "C"
