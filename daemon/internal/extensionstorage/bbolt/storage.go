// Package bbolt implements persistent record storage using bbolt.
package bbolt

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/moby/extensions"
	"github.com/moby/moby/v2/errdefs"
	daemonconfigv0 "github.com/moby/moby/v2/extpoints/daemonconfig/v0"
	storagekv "github.com/moby/moby/v2/extpoints/storage/kv/v0"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ID identifies the daemon's bbolt storage provider.
const ID extensions.ExtensionID = "org.mobyproject.storage.bbolt.v0"

// Extension is a reusable definition of the bbolt storage provider.
// The database is opened lazily on the first storage operation.
// Neither extension shutdown nor removal deletes persistent data.
var Extension definition

type definition struct{}

func (definition) Declaration() extensions.Declaration {
	s := &storage{}
	return extensions.Declaration{
		ID:           ID,
		Providers:    []extensions.Provider{storagekv.Point.Provide(s)},
		Dependencies: []extensions.Dependency{daemonconfigv0.Point.Dependency()},
		Init:         s.init,
		Shutdown:     s.shutdown,
	}
}

type storage struct {
	mu       sync.Mutex
	dataRoot string
	db       *bolt.DB
	closed   bool
}

func (s *storage) init(ctx context.Context, _ extensions.Config, resolver extensions.Resolver) error {
	provider, err := daemonconfigv0.Point.Single(resolver)
	if err != nil {
		return fmt.Errorf("resolve daemon config: %w", err)
	}
	response, err := provider.Get(ctx, &daemonconfigv0.GetRequest{})
	if err != nil {
		return fmt.Errorf("get daemon config: %w", err)
	}
	if response == nil || response.RootDir == "" {
		return errors.New("daemon config persistent root is required")
	}
	if !filepath.IsAbs(response.RootDir) {
		return fmt.Errorf("daemon config persistent root %q must be absolute", response.RootDir)
	}
	s.dataRoot = response.RootDir
	return nil
}

func (s *storage) shutdown(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// transaction also serializes lazy opening and shutdown with active operations.
func (s *storage) transaction(ctx context.Context, write bool, fn func(*bolt.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.closed {
		return errdefs.Unavailable(status.Error(codes.Unavailable, "extension storage is closed"))
	}
	if s.dataRoot == "" {
		return errors.New("extension storage is not initialized")
	}
	if s.db == nil {
		db, err := openDatabase(s.dataRoot)
		if err != nil {
			return err
		}
		s.db = db
	}
	operation := func(tx *bolt.Tx) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := fn(tx); err != nil {
			return err
		}
		return ctx.Err()
	}
	if write {
		return s.db.Update(operation)
	}
	return s.db.View(operation)
}

func (s *storage) Get(ctx context.Context, req *storagekv.RecordRequest) (*storagekv.GetResponse, error) {
	if err := validateRecord(req); err != nil {
		return nil, err
	}
	resp := &storagekv.GetResponse{}
	err := s.transaction(ctx, false, func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(req.Namespace))
		if bucket == nil || bucket.Get([]byte(req.Key)) == nil {
			return recordNotFound(req.Key)
		}
		// Bolt values are valid only for the lifetime of the transaction.
		resp.Value = bytes.Clone(bucket.Get([]byte(req.Key)))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *storage) Create(ctx context.Context, req *storagekv.WriteRequest) error {
	return s.write(ctx, req, true)
}

func (s *storage) Update(ctx context.Context, req *storagekv.WriteRequest) error {
	return s.write(ctx, req, false)
}

func (s *storage) write(ctx context.Context, req *storagekv.WriteRequest, create bool) error {
	if req == nil {
		return invalidRequest("write request is required")
	}
	if err := validateRecord(&storagekv.RecordRequest{Namespace: req.Namespace, Key: req.Key}); err != nil {
		return err
	}
	if len(req.Value) > storagekv.MaxValueSize {
		return invalidRequest("record value exceeds maximum size")
	}
	return s.transaction(ctx, true, func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(req.Namespace))
		exists := bucket != nil && bucket.Get([]byte(req.Key)) != nil
		if create && exists {
			return errdefs.Conflict(status.Errorf(codes.AlreadyExists, "record %q already exists", req.Key))
		}
		if !create && !exists {
			return recordNotFound(req.Key)
		}
		if bucket == nil {
			var err error
			bucket, err = tx.CreateBucket([]byte(req.Namespace))
			if err != nil {
				return err
			}
		}
		return bucket.Put([]byte(req.Key), req.Value)
	})
}

func (s *storage) Delete(ctx context.Context, req *storagekv.RecordRequest) error {
	if err := validateRecord(req); err != nil {
		return err
	}
	return s.transaction(ctx, true, func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(req.Namespace))
		if bucket == nil {
			return nil
		}
		return bucket.Delete([]byte(req.Key))
	})
}

// listCursor binds an exclusive key position to the original query.
// It carries no authority; callers may already read any namespace.
type listCursor struct {
	Namespace string `json:"namespace"`
	Prefix    string `json:"prefix"`
	After     string `json:"after"`
}

func (s *storage) List(ctx context.Context, req *storagekv.ListRequest) (*storagekv.ListResponse, error) {
	if req == nil {
		return nil, invalidRequest("list request is required")
	}
	if err := validateNamespace(req.Namespace); err != nil {
		return nil, err
	}
	if !validKey(req.Prefix, true) {
		return nil, invalidRequest("invalid key prefix")
	}
	if req.Limit > storagekv.MaxPageSize {
		return nil, invalidRequest("page limit exceeds maximum size")
	}
	limit := req.Limit
	if limit == 0 {
		limit = storagekv.DefaultPageSize
	}
	var after string
	if req.Cursor != "" {
		// Bound allocation before decoding untrusted cursor data.
		if len(req.Cursor) > 8*(storagekv.MaxNamespaceSize+2*storagekv.MaxKeySize) {
			return nil, invalidRequest("invalid list cursor")
		}
		data, err := base64.RawURLEncoding.DecodeString(req.Cursor)
		var cursor listCursor
		if err != nil || json.Unmarshal(data, &cursor) != nil || cursor.Namespace != req.Namespace || cursor.Prefix != req.Prefix || !validKey(cursor.After, false) || !strings.HasPrefix(cursor.After, req.Prefix) {
			return nil, invalidRequest("invalid list cursor")
		}
		after = cursor.After
	}
	resp := &storagekv.ListResponse{}
	err := s.transaction(ctx, false, func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(req.Namespace))
		if bucket == nil {
			return nil
		}
		cursor := bucket.Cursor()
		start := req.Prefix
		if after != "" {
			start = after
		}
		key, _ := cursor.Seek([]byte(start))
		if after != "" && string(key) == after {
			key, _ = cursor.Next()
		}
		for ; key != nil && bytes.HasPrefix(key, []byte(req.Prefix)); key, _ = cursor.Next() {
			if uint32(len(resp.Keys)) == limit {
				data, err := json.Marshal(listCursor{Namespace: req.Namespace, Prefix: req.Prefix, After: resp.Keys[len(resp.Keys)-1]})
				if err != nil {
					return err
				}
				resp.NextCursor = base64.RawURLEncoding.EncodeToString(data)
				break
			}
			resp.Keys = append(resp.Keys, string(key))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func validateRecord(req *storagekv.RecordRequest) error {
	if req == nil {
		return invalidRequest("record request is required")
	}
	if err := validateNamespace(req.Namespace); err != nil {
		return err
	}
	if !validKey(req.Key, false) {
		return invalidRequest("invalid record key")
	}
	return nil
}

func validateNamespace(namespace string) error {
	if namespace == "" || len(namespace) > storagekv.MaxNamespaceSize {
		return invalidRequest("invalid storage namespace")
	}
	for i := 0; i < len(namespace); i++ {
		c := namespace[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			continue
		}
		if i > 0 && (c == '.' || c == '_' || c == '-') {
			continue
		}
		return invalidRequest("invalid storage namespace")
	}
	return nil
}

func validKey(key string, allowEmpty bool) bool {
	return (allowEmpty || key != "") && len(key) <= storagekv.MaxKeySize && utf8.ValidString(key) && !strings.ContainsRune(key, 0)
}

func invalidRequest(message string) error {
	return errdefs.InvalidParameter(status.Error(codes.InvalidArgument, message))
}

func recordNotFound(key string) error {
	return errdefs.NotFound(status.Errorf(codes.NotFound, "record %q not found", key))
}
