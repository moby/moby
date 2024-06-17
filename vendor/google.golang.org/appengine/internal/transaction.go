// Copyright 2014 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package internal

// This file implements hooks for applying datastore transactions.

import (
	"context"
	"errors"
	"reflect"

	"github.com/golang/protobuf/proto"

	basepb "google.golang.org/appengine/internal/base"
	pb "google.golang.org/appengine/internal/datastore"
)

var transactionSetters = make(map[reflect.Type]reflect.Value)

// RegisterTransactionSetter registers a function that sets transaction information
// in a protocol buffer message. f should be a function with two arguments,
// the first being a protocol buffer type, and the second being *datastore.Transaction.
func RegisterTransactionSetter(f interface{}) {
	v := reflect.ValueOf(f)
	transactionSetters[v.Type().In(0)] = v
}

// applyTransaction applies the transaction t to message pb
// by using the relevant setter passed to RegisterTransactionSetter.
func applyTransaction(pb proto.Message, t *pb.Transaction) {
	v := reflect.ValueOf(pb)
	if f, ok := transactionSetters[v.Type()]; ok {
		f.Call([]reflect.Value{v, reflect.ValueOf(t)})
	}
}

var transactionKey = "used for *Transaction"

func transactionFromContext(ctx context.Context) *transaction {
	t, _ := ctx.Value(&transactionKey).(*transaction)
	return t
}

func withTransaction(ctx context.Context, t *transaction) context.Context {
	return context.WithValue(ctx, &transactionKey, t)
}

type transaction struct {
	transaction pb.Transaction
	finished    bool
}

var ErrConcurrentTransaction = errors.New("internal: concurrent transaction")

func RunTransactionOnce(c context.Context, f func(context.Context) error, xg bool, readOnly bool, previousTransaction *pb.Transaction) (*pb.Transaction, error) {
	if transactionFromContext(c) != nil {
		return nil, errors.New("nested transactions are not supported")
	}

	// Begin the transaction.
	t := &transaction{}
	req := &pb.BeginTransactionRequest{
		App: proto.String(FullyQualifiedAppID(c)),
	}
	if xg {
		req.AllowMultipleEg = proto.Bool(true)
	}
	if previousTransaction != nil {
		req.PreviousTransaction = previousTransaction
	}
	if readOnly {
		req.Mode = pb.BeginTransactionRequest_READ_ONLY.Enum()
	} else {
		req.Mode = pb.BeginTransactionRequest_READ_WRITE.Enum()
	}
	if err := Call(c, "datastore_v3", "BeginTransaction", req, &t.transaction); err != nil {
		return nil, err
	}

	// Call f, rolling back the transaction if f returns a non-nil error, or panics.
	// The panic is not recovered.
	defer func() {
		if t.finished {
			return
		}
		t.finished = true
		// Ignore the error return value, since we are already returning a non-nil
		// error (or we're panicking).
		Call(c, "datastore_v3", "Rollback", &t.transaction, &basepb.VoidProto{})
	}()
	if err := f(withTransaction(c, t)); err != nil {
		return &t.transaction, err
	}
	t.finished = true

	// Commit the transaction.
	res := &pb.CommitResponse{}
	err := Call(c, "datastore_v3", "Commit", &t.transaction, res)
	if ae, ok := err.(*APIError); ok {
		/* TODO: restore this conditional
		if appengine.IsDevAppServer() {
		*/
		// The Python Dev AppServer raises an ApplicationError with error code 2 (which is
		// Error.CONCURRENT_TRANSACTION) and message "Concurrency exception.".
		if ae.Code == int32(pb.Error_BAD_REQUEST) && ae.Detail == "ApplicationError: 2 Concurrency exception." {
			return &t.transaction, ErrConcurrentTransaction
		}
		if ae.Code == int32(pb.Error_CONCURRENT_TRANSACTION) {
			return &t.transaction, ErrConcurrentTransaction
		}
	}
	return &t.transaction, err
}
