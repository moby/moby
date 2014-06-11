// +build !linux,!freebsd linux,!amd64,!arm freebsd,!cgo

package graphdb

func NewSqliteConn(root string) (*Database, error) {
	panic("Not implemented")
}
