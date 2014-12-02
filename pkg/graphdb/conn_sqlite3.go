// +build cgo

package graphdb

import (
	"database/sql"

	_ "code.google.com/p/gosqlite/sqlite3" // registers sqlite
)

func NewSqliteConn(root string) (*Database, error) {
	conn, err := sql.Open("sqlite3", root)
	if err != nil {
		return nil, err
	}

	return NewDatabase(conn)
}
