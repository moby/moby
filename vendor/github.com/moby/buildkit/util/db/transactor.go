package db

import (
	bolt "go.etcd.io/bbolt"
)

// Transactor is the database interface for running transactions
type Transactor interface {
	View(fn func(*bolt.Tx) error) error
	Update(fn func(*bolt.Tx) error) error
}
