/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package boltutil

import (
	"context"

	bolt "go.etcd.io/bbolt"
)

type transactionKey struct{}

// WithTransaction returns a new context holding the provided
// bolt transaction. Functions which require a bolt transaction will
// first check to see if a transaction is already created on the
// context before creating their own.
func WithTransaction(ctx context.Context, tx *bolt.Tx) context.Context {
	return context.WithValue(ctx, transactionKey{}, tx)
}

// Transaction returns the transaction from the context
// if it has one.
func Transaction(ctx context.Context) (tx *bolt.Tx, ok bool) {
	tx, ok = ctx.Value(transactionKey{}).(*bolt.Tx)
	return
}
