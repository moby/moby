// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sqlite provides access to the SQLite library, version 3.
package sqlite

/*
#cgo LDFLAGS: -lsqlite3

#include <sqlite3.h>
#include <stdlib.h>

// These wrappers are necessary because SQLITE_TRANSIENT
// is a pointer constant, and cgo doesn't translate them correctly.
// The definition in sqlite3.h is:
//
// typedef void (*sqlite3_destructor_type)(void*);
// #define SQLITE_STATIC      ((sqlite3_destructor_type)0)
// #define SQLITE_TRANSIENT   ((sqlite3_destructor_type)-1)

static int my_bind_text(sqlite3_stmt *stmt, int n, char *p, int np) {
	return sqlite3_bind_text(stmt, n, p, np, SQLITE_TRANSIENT);
}
static int my_bind_blob(sqlite3_stmt *stmt, int n, void *p, int np) {
	return sqlite3_bind_blob(stmt, n, p, np, SQLITE_TRANSIENT);
}

*/
import "C"

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"
	"unsafe"
)

type Errno int

func (e Errno) Error() string {
	s := errText[e]
	if s == "" {
		return fmt.Sprintf("errno %d", int(e))
	}
	return s
}

var (
	ErrError      error = Errno(1)   //    /* SQL error or missing database */
	ErrInternal   error = Errno(2)   //    /* Internal logic error in SQLite */
	ErrPerm       error = Errno(3)   //    /* Access permission denied */
	ErrAbort      error = Errno(4)   //    /* Callback routine requested an abort */
	ErrBusy       error = Errno(5)   //    /* The database file is locked */
	ErrLocked     error = Errno(6)   //    /* A table in the database is locked */
	ErrNoMem      error = Errno(7)   //    /* A malloc() failed */
	ErrReadOnly   error = Errno(8)   //    /* Attempt to write a readonly database */
	ErrInterrupt  error = Errno(9)   //    /* Operation terminated by sqlite3_interrupt()*/
	ErrIOErr      error = Errno(10)  //    /* Some kind of disk I/O error occurred */
	ErrCorrupt    error = Errno(11)  //    /* The database disk image is malformed */
	ErrFull       error = Errno(13)  //    /* Insertion failed because database is full */
	ErrCantOpen   error = Errno(14)  //    /* Unable to open the database file */
	ErrEmpty      error = Errno(16)  //    /* Database is empty */
	ErrSchema     error = Errno(17)  //    /* The database schema changed */
	ErrTooBig     error = Errno(18)  //    /* String or BLOB exceeds size limit */
	ErrConstraint error = Errno(19)  //    /* Abort due to constraint violation */
	ErrMismatch   error = Errno(20)  //    /* Data type mismatch */
	ErrMisuse     error = Errno(21)  //    /* Library used incorrectly */
	ErrNolfs      error = Errno(22)  //    /* Uses OS features not supported on host */
	ErrAuth       error = Errno(23)  //    /* Authorization denied */
	ErrFormat     error = Errno(24)  //    /* Auxiliary database format error */
	ErrRange      error = Errno(25)  //    /* 2nd parameter to sqlite3_bind out of range */
	ErrNotDB      error = Errno(26)  //    /* File opened that is not a database file */
	Row                 = Errno(100) //   /* sqlite3_step() has another row ready */
	Done                = Errno(101) //   /* sqlite3_step() has finished executing */
)

var errText = map[Errno]string{
	1:   "SQL error or missing database",
	2:   "Internal logic error in SQLite",
	3:   "Access permission denied",
	4:   "Callback routine requested an abort",
	5:   "The database file is locked",
	6:   "A table in the database is locked",
	7:   "A malloc() failed",
	8:   "Attempt to write a readonly database",
	9:   "Operation terminated by sqlite3_interrupt()*/",
	10:  "Some kind of disk I/O error occurred",
	11:  "The database disk image is malformed",
	12:  "NOT USED. Table or record not found",
	13:  "Insertion failed because database is full",
	14:  "Unable to open the database file",
	15:  "NOT USED. Database lock protocol error",
	16:  "Database is empty",
	17:  "The database schema changed",
	18:  "String or BLOB exceeds size limit",
	19:  "Abort due to constraint violation",
	20:  "Data type mismatch",
	21:  "Library used incorrectly",
	22:  "Uses OS features not supported on host",
	23:  "Authorization denied",
	24:  "Auxiliary database format error",
	25:  "2nd parameter to sqlite3_bind out of range",
	26:  "File opened that is not a database file",
	100: "sqlite3_step() has another row ready",
	101: "sqlite3_step() has finished executing",
}

func (c *Conn) error(rv C.int) error {
	if c == nil || c.db == nil {
		return errors.New("nil sqlite database")
	}
	if rv == 0 {
		return nil
	}
	if rv == 21 { // misuse
		return Errno(rv)
	}
	return errors.New(Errno(rv).Error() + ": " + C.GoString(C.sqlite3_errmsg(c.db)))
}

type Conn struct {
	db *C.sqlite3
}

func Version() string {
	p := C.sqlite3_libversion()
	return C.GoString(p)
}

func Open(filename string) (*Conn, error) {
	if C.sqlite3_threadsafe() == 0 {
		return nil, errors.New("sqlite library was not compiled for thread-safe operation")
	}

	var db *C.sqlite3
	name := C.CString(filename)
	defer C.free(unsafe.Pointer(name))
	rv := C.sqlite3_open_v2(name, &db,
		C.SQLITE_OPEN_FULLMUTEX|
			C.SQLITE_OPEN_READWRITE|
			C.SQLITE_OPEN_CREATE,
		nil)
	if rv != 0 {
		return nil, Errno(rv)
	}
	if db == nil {
		return nil, errors.New("sqlite succeeded without returning a database")
	}
	return &Conn{db}, nil
}

func NewBackup(dst *Conn, dstTable string, src *Conn, srcTable string) (*Backup, error) {
	dname := C.CString(dstTable)
	sname := C.CString(srcTable)
	defer C.free(unsafe.Pointer(dname))
	defer C.free(unsafe.Pointer(sname))

	sb := C.sqlite3_backup_init(dst.db, dname, src.db, sname)
	if sb == nil {
		return nil, dst.error(C.sqlite3_errcode(dst.db))
	}
	return &Backup{sb, dst, src}, nil
}

type Backup struct {
	sb       *C.sqlite3_backup
	dst, src *Conn
}

func (b *Backup) Step(npage int) error {
	rv := C.sqlite3_backup_step(b.sb, C.int(npage))
	if rv == 0 || Errno(rv) == ErrBusy || Errno(rv) == ErrLocked {
		return nil
	}
	return Errno(rv)
}

type BackupStatus struct {
	Remaining int
	PageCount int
}

func (b *Backup) Status() BackupStatus {
	return BackupStatus{int(C.sqlite3_backup_remaining(b.sb)), int(C.sqlite3_backup_pagecount(b.sb))}
}

func (b *Backup) Run(npage int, period time.Duration, c chan<- BackupStatus) error {
	var err error
	for {
		err = b.Step(npage)
		if err != nil {
			break
		}
		if c != nil {
			c <- b.Status()
		}
		time.Sleep(period)
	}
	return b.dst.error(C.sqlite3_errcode(b.dst.db))
}

func (b *Backup) Close() error {
	if b.sb == nil {
		return errors.New("backup already closed")
	}
	C.sqlite3_backup_finish(b.sb)
	b.sb = nil
	return nil
}

func (c *Conn) BusyTimeout(ms int) error {
	rv := C.sqlite3_busy_timeout(c.db, C.int(ms))
	if rv == 0 {
		return nil
	}
	return Errno(rv)
}

func (c *Conn) Exec(cmd string, args ...interface{}) error {
	s, err := c.Prepare(cmd)
	if err != nil {
		return err
	}
	defer s.Finalize()
	err = s.Exec(args...)
	if err != nil {
		return err
	}
	rv := C.sqlite3_step(s.stmt)
	if Errno(rv) != Done {
		return c.error(rv)
	}
	return nil
}

type Stmt struct {
	c    *Conn
	stmt *C.sqlite3_stmt
	err  error
	t0   time.Time
	sql  string
	args string
}

func (c *Conn) Prepare(cmd string) (*Stmt, error) {
	if c == nil || c.db == nil {
		return nil, errors.New("nil sqlite database")
	}
	cmdstr := C.CString(cmd)
	defer C.free(unsafe.Pointer(cmdstr))
	var stmt *C.sqlite3_stmt
	var tail *C.char
	rv := C.sqlite3_prepare_v2(c.db, cmdstr, C.int(len(cmd)+1), &stmt, &tail)
	if rv != 0 {
		return nil, c.error(rv)
	}
	return &Stmt{c: c, stmt: stmt, sql: cmd, t0: time.Now()}, nil
}

func (s *Stmt) Exec(args ...interface{}) error {
	s.args = fmt.Sprintf(" %v", []interface{}(args))
	rv := C.sqlite3_reset(s.stmt)
	if rv != 0 {
		return s.c.error(rv)
	}

	n := int(C.sqlite3_bind_parameter_count(s.stmt))
	if n != len(args) {
		return errors.New(fmt.Sprintf("incorrect argument count for Stmt.Exec: have %d want %d", len(args), n))
	}

	for i, v := range args {
		var str string
		switch v := v.(type) {
		case []byte:
			var p *byte
			if len(v) > 0 {
				p = &v[0]
			}
			if rv := C.my_bind_blob(s.stmt, C.int(i+1), unsafe.Pointer(p), C.int(len(v))); rv != 0 {
				return s.c.error(rv)
			}
			continue

		case bool:
			if v {
				str = "1"
			} else {
				str = "0"
			}

		default:
			str = fmt.Sprint(v)
		}

		cstr := C.CString(str)
		rv := C.my_bind_text(s.stmt, C.int(i+1), cstr, C.int(len(str)))
		C.free(unsafe.Pointer(cstr))
		if rv != 0 {
			return s.c.error(rv)
		}
	}
	return nil
}

func (s *Stmt) Error() error {
	return s.err
}

func (s *Stmt) Next() bool {
	rv := C.sqlite3_step(s.stmt)
	err := Errno(rv)
	if err == Row {
		return true
	}
	if err != Done {
		s.err = s.c.error(rv)
	}
	return false
}

func (s *Stmt) Reset() error {
	C.sqlite3_reset(s.stmt)
	return nil
}

func (s *Stmt) Scan(args ...interface{}) error {
	n := int(C.sqlite3_column_count(s.stmt))
	if n != len(args) {
		return errors.New(fmt.Sprintf("incorrect argument count for Stmt.Scan: have %d want %d", len(args), n))
	}

	for i, v := range args {
		n := C.sqlite3_column_bytes(s.stmt, C.int(i))
		p := C.sqlite3_column_blob(s.stmt, C.int(i))
		if p == nil && n > 0 {
			return errors.New("got nil blob")
		}
		var data []byte
		if n > 0 {
			data = (*[1 << 30]byte)(unsafe.Pointer(p))[0:n]
		}
		switch v := v.(type) {
		case *[]byte:
			*v = data
		case *string:
			*v = string(data)
		case *bool:
			*v = string(data) == "1"
		case *int:
			x, err := strconv.Atoi(string(data))
			if err != nil {
				return errors.New("arg " + strconv.Itoa(i) + " as int: " + err.Error())
			}
			*v = x
		case *int64:
			x, err := strconv.ParseInt(string(data), 10, 64)
			if err != nil {
				return errors.New("arg " + strconv.Itoa(i) + " as int64: " + err.Error())
			}
			*v = x
		case *float64:
			x, err := strconv.ParseFloat(string(data), 64)
			if err != nil {
				return errors.New("arg " + strconv.Itoa(i) + " as float64: " + err.Error())
			}
			*v = x
		default:
			return errors.New("unsupported type in Scan: " + reflect.TypeOf(v).String())
		}
	}
	return nil
}

func (s *Stmt) SQL() string {
	return s.sql + s.args
}

func (s *Stmt) Nanoseconds() int64 {
	return time.Now().Sub(s.t0).Nanoseconds()
}

func (s *Stmt) Finalize() error {
	rv := C.sqlite3_finalize(s.stmt)
	if rv != 0 {
		return s.c.error(rv)
	}
	return nil
}

func (c *Conn) Close() error {
	if c == nil || c.db == nil {
		return errors.New("nil sqlite database")
	}
	rv := C.sqlite3_close(c.db)
	if rv != 0 {
		return c.error(rv)
	}
	c.db = nil
	return nil
}
