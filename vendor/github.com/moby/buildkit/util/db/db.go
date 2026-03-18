package db

import "io"

type DB interface {
	io.Closer
	Transactor
}
