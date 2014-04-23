// +build linux,amd64 freebsd,cgo

package graphdb

import (
	_ "code.google.com/p/gosqlite/sqlite3" // registers sqlite
	"database/sql"
	"os"
)

func NewSqliteConn(root string) (*Database, error) {
	initDatabase := false
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			initDatabase = true
		} else {
			return nil, err
		}
	}
	conn, err := sql.Open("sqlite3", root)
	if err != nil {
		return nil, err
	}
	return NewDatabase(conn, initDatabase)
}
