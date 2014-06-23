// +build cgo

package graphdb

import (
	"database/sql"
	"os"

	_ "code.google.com/p/gosqlite/sqlite3" // registers sqlite
)

func NewSqliteConn(root string) (*Database, error) {
	initDatabase := false

	stat, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			initDatabase = true
		} else {
			return nil, err
		}
	}

	if stat != nil && stat.Size() == 0 {
		initDatabase = true
	}

	conn, err := sql.Open("sqlite3", root)
	if err != nil {
		return nil, err
	}

	return NewDatabase(conn, initDatabase)
}
