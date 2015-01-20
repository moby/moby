// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sqlite3 provides access to the SQLite library, version 3.
//
// The package has no exported API.
// It registers a driver for the standard Go database/sql package.
//
//	import _ "code.google.com/p/gosqlite/sqlite3"
//
// (For an alternate, earlier API, see the code.google.com/p/gosqlite/sqlite package.)
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
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unsafe"
)

func init() {
	sql.Register("sqlite3", impl{})
}

type errno int

func (e errno) Error() string {
	s := errText[e]
	if s == "" {
		return fmt.Sprintf("errno %d", int(e))
	}
	return s
}

var (
	errError      error = errno(1)   //    /* SQL error or missing database */
	errInternal   error = errno(2)   //    /* Internal logic error in SQLite */
	errPerm       error = errno(3)   //    /* Access permission denied */
	errAbort      error = errno(4)   //    /* Callback routine requested an abort */
	errBusy       error = errno(5)   //    /* The database file is locked */
	errLocked     error = errno(6)   //    /* A table in the database is locked */
	errNoMem      error = errno(7)   //    /* A malloc() failed */
	errReadOnly   error = errno(8)   //    /* Attempt to write a readonly database */
	errInterrupt  error = errno(9)   //    /* Operation terminated by sqlite3_interrupt()*/
	errIOErr      error = errno(10)  //    /* Some kind of disk I/O error occurred */
	errCorrupt    error = errno(11)  //    /* The database disk image is malformed */
	errFull       error = errno(13)  //    /* Insertion failed because database is full */
	errCantOpen   error = errno(14)  //    /* Unable to open the database file */
	errEmpty      error = errno(16)  //    /* Database is empty */
	errSchema     error = errno(17)  //    /* The database schema changed */
	errTooBig     error = errno(18)  //    /* String or BLOB exceeds size limit */
	errConstraint error = errno(19)  //    /* Abort due to constraint violation */
	errMismatch   error = errno(20)  //    /* Data type mismatch */
	errMisuse     error = errno(21)  //    /* Library used incorrectly */
	errNolfs      error = errno(22)  //    /* Uses OS features not supported on host */
	errAuth       error = errno(23)  //    /* Authorization denied */
	errFormat     error = errno(24)  //    /* Auxiliary database format error */
	errRange      error = errno(25)  //    /* 2nd parameter to sqlite3_bind out of range */
	errNotDB      error = errno(26)  //    /* File opened that is not a database file */
	stepRow             = errno(100) //   /* sqlite3_step() has another row ready */
	stepDone            = errno(101) //   /* sqlite3_step() has finished executing */
)

var errText = map[errno]string{
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

type impl struct{}

func (impl) Open(name string) (driver.Conn, error) {
	if C.sqlite3_threadsafe() == 0 {
		return nil, errors.New("sqlite library was not compiled for thread-safe operation")
	}

	var db *C.sqlite3
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	rv := C.sqlite3_open_v2(cname, &db,
		C.SQLITE_OPEN_FULLMUTEX|
			C.SQLITE_OPEN_READWRITE|
			C.SQLITE_OPEN_CREATE,
		nil)
	if rv != 0 {
		return nil, errno(rv)
	}
	if db == nil {
		return nil, errors.New("sqlite succeeded without returning a database")
	}
	return &conn{db: db}, nil
}

type conn struct {
	db     *C.sqlite3
	closed bool
	tx     bool
}

func (c *conn) error(rv C.int) error {
	if rv == 0 {
		return nil
	}
	if rv == 21 || c.closed {
		return errno(rv)
	}
	return errors.New(errno(rv).Error() + ": " + C.GoString(C.sqlite3_errmsg(c.db)))
}

func (c *conn) Prepare(cmd string) (driver.Stmt, error) {
	if c.closed {
		panic("database/sql/driver: misuse of sqlite driver: Prepare after Close")
	}
	cmdstr := C.CString(cmd)
	defer C.free(unsafe.Pointer(cmdstr))
	var s *C.sqlite3_stmt
	var tail *C.char
	rv := C.sqlite3_prepare_v2(c.db, cmdstr, C.int(len(cmd)+1), &s, &tail)
	if rv != 0 {
		return nil, c.error(rv)
	}
	return &stmt{c: c, stmt: s, sql: cmd, t0: time.Now()}, nil
}

func (c *conn) Close() error {
	if c.closed {
		panic("database/sql/driver: misuse of sqlite driver: multiple Close")
	}
	c.closed = true
	rv := C.sqlite3_close(c.db)
	c.db = nil
	return c.error(rv)
}

func (c *conn) exec(cmd string) error {
	cstring := C.CString(cmd)
	defer C.free(unsafe.Pointer(cstring))
	rv := C.sqlite3_exec(c.db, cstring, nil, nil, nil)
	return c.error(rv)
}

func (c *conn) Begin() (driver.Tx, error) {
	if c.tx {
		panic("database/sql/driver: misuse of sqlite driver: multiple Tx")
	}
	if err := c.exec("BEGIN TRANSACTION"); err != nil {
		return nil, err
	}
	c.tx = true
	return &tx{c}, nil
}

type tx struct {
	c *conn
}

func (t *tx) Commit() error {
	if t.c == nil || !t.c.tx {
		panic("database/sql/driver: misuse of sqlite driver: extra Commit")
	}
	t.c.tx = false
	err := t.c.exec("COMMIT TRANSACTION")
	t.c = nil
	return err
}

func (t *tx) Rollback() error {
	if t.c == nil || !t.c.tx {
		panic("database/sql/driver: misuse of sqlite driver: extra Rollback")
	}
	t.c.tx = false
	err := t.c.exec("ROLLBACK")
	t.c = nil
	return err
}

type stmt struct {
	c        *conn
	stmt     *C.sqlite3_stmt
	err      error
	t0       time.Time
	sql      string
	args     string
	closed   bool
	rows     bool
	colnames []string
	coltypes []string
}

func (s *stmt) Close() error {
	if s.rows {
		panic("database/sql/driver: misuse of sqlite driver: Close with active Rows")
	}
	if s.closed {
		panic("database/sql/driver: misuse of sqlite driver: double Close of Stmt")
	}
	s.closed = true
	rv := C.sqlite3_finalize(s.stmt)
	if rv != 0 {
		return s.c.error(rv)
	}
	return nil
}

func (s *stmt) NumInput() int {
	if s.closed {
		panic("database/sql/driver: misuse of sqlite driver: NumInput after Close")
	}
	return int(C.sqlite3_bind_parameter_count(s.stmt))
}

func (s *stmt) reset() error {
	return s.c.error(C.sqlite3_reset(s.stmt))
}

func (s *stmt) start(args []driver.Value) error {
	if err := s.reset(); err != nil {
		return err
	}

	n := int(C.sqlite3_bind_parameter_count(s.stmt))
	if n != len(args) {
		return fmt.Errorf("incorrect argument count for command: have %d want %d", len(args), n)
	}

	for i, v := range args {
		var str string
		switch v := v.(type) {
		case nil:
			if rv := C.sqlite3_bind_null(s.stmt, C.int(i+1)); rv != 0 {
				return s.c.error(rv)
			}
			continue

		case float64:
			if rv := C.sqlite3_bind_double(s.stmt, C.int(i+1), C.double(v)); rv != 0 {
				return s.c.error(rv)
			}
			continue

		case int64:
			if rv := C.sqlite3_bind_int64(s.stmt, C.int(i+1), C.sqlite3_int64(v)); rv != 0 {
				return s.c.error(rv)
			}
			continue

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
			var vi int64
			if v {
				vi = 1
			}
			if rv := C.sqlite3_bind_int64(s.stmt, C.int(i+1), C.sqlite3_int64(vi)); rv != 0 {
				return s.c.error(rv)
			}
			continue

		case time.Time:
			str = v.UTC().Format(timefmt[0])

		case string:
			str = v

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

func (s *stmt) Exec(args []driver.Value) (driver.Result, error) {
	if s.closed {
		panic("database/sql/driver: misuse of sqlite driver: Exec after Close")
	}
	if s.rows {
		panic("database/sql/driver: misuse of sqlite driver: Exec with active Rows")
	}

	err := s.start(args)
	if err != nil {
		return nil, err
	}

	rv := C.sqlite3_step(s.stmt)
	if errno(rv) != stepDone {
		if rv == 0 {
			rv = 21 // errMisuse
		}
		return nil, s.c.error(rv)
	}

	id := int64(C.sqlite3_last_insert_rowid(s.c.db))
	rows := int64(C.sqlite3_changes(s.c.db))
	return &result{id, rows}, nil
}

func (s *stmt) Query(args []driver.Value) (driver.Rows, error) {
	if s.closed {
		panic("database/sql/driver: misuse of sqlite driver: Query after Close")
	}
	if s.rows {
		panic("database/sql/driver: misuse of sqlite driver: Query with active Rows")
	}

	err := s.start(args)
	if err != nil {
		return nil, err
	}

	s.rows = true
	if s.colnames == nil {
		n := int64(C.sqlite3_column_count(s.stmt))
		s.colnames = make([]string, n)
		s.coltypes = make([]string, n)
		for i := range s.colnames {
			s.colnames[i] = C.GoString(C.sqlite3_column_name(s.stmt, C.int(i)))
			s.coltypes[i] = strings.ToLower(C.GoString(C.sqlite3_column_decltype(s.stmt, C.int(i))))
		}
	}
	return &rows{s}, nil
}

type rows struct {
	s *stmt
}

func (r *rows) Columns() []string {
	if r.s == nil {
		panic("database/sql/driver: misuse of sqlite driver: Columns of closed Rows")
	}
	return r.s.colnames
}

const maxslice = 1<<31 - 1

var timefmt = []string{
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04",
	"2006-01-02T15:04",
	"2006-01-02",
}

func (r *rows) Next(dst []driver.Value) error {
	if r.s == nil {
		panic("database/sql/driver: misuse of sqlite driver: Next of closed Rows")
	}

	rv := C.sqlite3_step(r.s.stmt)
	if errno(rv) != stepRow {
		if errno(rv) == stepDone {
			return io.EOF
		}
		if rv == 0 {
			rv = 21
		}
		return r.s.c.error(rv)
	}

	for i := range dst {
		switch typ := C.sqlite3_column_type(r.s.stmt, C.int(i)); typ {
		default:
			return fmt.Errorf("unexpected sqlite3 column type %d", typ)
		case C.SQLITE_INTEGER:
			val := int64(C.sqlite3_column_int64(r.s.stmt, C.int(i)))
			switch r.s.coltypes[i] {
			case "timestamp", "datetime":
				dst[i] = time.Unix(val, 0).UTC()
			case "boolean":
				dst[i] = val > 0
			default:
				dst[i] = val
			}

		case C.SQLITE_FLOAT:
			dst[i] = float64(C.sqlite3_column_double(r.s.stmt, C.int(i)))

		case C.SQLITE_BLOB, C.SQLITE_TEXT:
			n := int(C.sqlite3_column_bytes(r.s.stmt, C.int(i)))
			var b []byte
			if n > 0 {
				p := C.sqlite3_column_blob(r.s.stmt, C.int(i))
				b = (*[maxslice]byte)(unsafe.Pointer(p))[:n]
			}
			dst[i] = b
			switch r.s.coltypes[i] {
			case "timestamp", "datetime":
				dst[i] = time.Time{}
				s := string(b)
				for _, f := range timefmt {
					if t, err := time.Parse(f, s); err == nil {
						dst[i] = t
						break
					}
				}
			}

		case C.SQLITE_NULL:
			dst[i] = nil
		}
	}
	return nil
}

func (r *rows) Close() error {
	if r.s == nil {
		panic("database/sql/driver: misuse of sqlite driver: Close of closed Rows")
	}
	r.s.rows = false
	r.s = nil
	return nil
}

type result struct {
	id   int64
	rows int64
}

func (r *result) LastInsertId() (int64, error) {
	return r.id, nil
}

func (r *result) RowsAffected() (int64, error) {
	return r.rows, nil
}
