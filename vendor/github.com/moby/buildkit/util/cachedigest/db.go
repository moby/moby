package cachedigest

import (
	"context"
	"crypto/sha256"
	"sync"

	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"
)

var ErrInvalidEncoding = errors.Errorf("invalid encoding")
var ErrNotFound = errors.Errorf("not found")

const bucketName = "byhash"

type DB struct {
	db *bbolt.DB
	wg sync.WaitGroup
}

var defaultDB = &DB{}

func SetDefaultDB(db *DB) {
	defaultDB = db
}

func GetDefaultDB() *DB {
	return defaultDB
}

func NewDB(path string) (*DB, error) {
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	return &DB{db: db}, nil
}

func (d *DB) Close() error {
	if d.db != nil {
		d.wg.Wait()
		return d.db.Close()
	}
	return nil
}

func (d *DB) NewHash(typ Type) *Hash {
	return &Hash{
		h:   sha256.New(),
		typ: typ,
		db:  d,
	}
}

func (d *DB) FromBytes(dt []byte, typ Type) (digest.Digest, error) {
	dgst := digest.FromBytes(dt)
	d.saveFrames(dgst.String(), []Frame{
		{ID: FrameIDType, Data: []byte(string(typ))},
		{ID: FrameIDData, Data: dt},
	})
	return dgst, nil
}

func (d *DB) saveFrames(key string, frames []Frame) {
	if d.db == nil {
		return
	}

	d.wg.Go(func() {
		val, err := encodeFrames(frames)
		if err != nil {
			// Optionally log error
			return
		}
		_ = d.db.Update(func(tx *bbolt.Tx) error {
			b, err := tx.CreateBucketIfNotExists([]byte(bucketName))
			if err != nil {
				return err
			}
			return b.Put([]byte(key), val)
		})
	})
}

func (d *DB) Get(ctx context.Context, dgst string) (Type, []Frame, error) {
	if d.db == nil {
		return "", nil, errors.WithStack(ErrNotFound)
	}
	parsed, err := digest.Parse(dgst)
	if err != nil {
		return "", nil, errors.Wrap(err, "invalid digest key")
	}
	var typ Type
	var resultFrames []Frame
	err = d.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return errors.WithStack(ErrNotFound)
		}
		val := b.Get([]byte(parsed.String()))
		if val == nil {
			return errors.WithStack(ErrNotFound)
		}
		frames, err := decodeFrames(val)
		if err != nil {
			return err
		}
		for _, f := range frames {
			switch f.ID {
			case FrameIDType:
				typ = Type(f.Data)
			case FrameIDData, FrameIDSkip:
				resultFrames = append(resultFrames, f)
			}
		}
		return nil
	})
	if err != nil {
		return "", nil, err
	}
	return typ, resultFrames, nil
}

func (d *DB) All(ctx context.Context, cb func(key string, typ Type, frames []Frame) error) error {
	if d.db == nil {
		return nil
	}
	return d.db.View(func(tx *bbolt.Tx) error {
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		default:
		}
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			keyStr := string(k)
			_, err := digest.Parse(keyStr)
			if err != nil {
				return errors.Wrapf(err, "invalid digest key: %s", keyStr)
			}
			frames, err := decodeFrames(v)
			if err != nil {
				return err
			}
			var typ Type
			var dataFrames []Frame
			for _, f := range frames {
				switch f.ID {
				case FrameIDType:
					typ = Type(f.Data)
				case FrameIDData, FrameIDSkip:
					dataFrames = append(dataFrames, f)
				}
			}
			return cb(keyStr, typ, dataFrames)
		})
	})
}

func (d *DB) Wait() {
	d.wg.Wait()
}
