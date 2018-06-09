package boltdbcachestorage

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/moby/buildkit/solver"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

const (
	resultBucket    = "_result"
	linksBucket     = "_links"
	byResultBucket  = "_byresult"
	backlinksBucket = "_backlinks"
)

type Store struct {
	db *bolt.DB
}

func NewStore(dbPath string) (*Store, error) {
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open database file %s", dbPath)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		for _, b := range []string{resultBucket, linksBucket, byResultBucket, backlinksBucket} {
			if _, err := tx.CreateBucketIfNotExists([]byte(b)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	db.NoSync = true
	return &Store{db: db}, nil
}

func (s *Store) Exists(id string) bool {
	exists := false
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(linksBucket)).Bucket([]byte(id))
		exists = b != nil
		return nil
	})
	if err != nil {
		return false
	}
	return exists
}

func (s *Store) Walk(fn func(id string) error) error {
	ids := make([]string, 0)
	if err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(linksBucket))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if v == nil {
				ids = append(ids, string(k))
			}
		}
		return nil
	}); err != nil {
		return err
	}
	for _, id := range ids {
		if err := fn(id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) WalkResults(id string, fn func(solver.CacheResult) error) error {
	var list []solver.CacheResult
	if err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(resultBucket))
		if b == nil {
			return nil
		}
		b = b.Bucket([]byte(id))
		if b == nil {
			return nil
		}

		return b.ForEach(func(k, v []byte) error {
			var res solver.CacheResult
			if err := json.Unmarshal(v, &res); err != nil {
				return err
			}
			list = append(list, res)
			return nil
		})
	}); err != nil {
		return err
	}
	for _, res := range list {
		if err := fn(res); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Load(id string, resultID string) (solver.CacheResult, error) {
	var res solver.CacheResult
	if err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(resultBucket))
		if b == nil {
			return errors.WithStack(solver.ErrNotFound)
		}
		b = b.Bucket([]byte(id))
		if b == nil {
			return errors.WithStack(solver.ErrNotFound)
		}

		v := b.Get([]byte(resultID))
		if v == nil {
			return errors.WithStack(solver.ErrNotFound)
		}

		return json.Unmarshal(v, &res)
	}); err != nil {
		return solver.CacheResult{}, err
	}
	return res, nil
}

func (s *Store) AddResult(id string, res solver.CacheResult) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.Bucket([]byte(resultBucket)).CreateBucketIfNotExists([]byte(id))
		if err != nil {
			return err
		}
		dt, err := json.Marshal(res)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(res.ID), dt); err != nil {
			return err
		}
		b, err = tx.Bucket([]byte(byResultBucket)).CreateBucketIfNotExists([]byte(res.ID))
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), []byte{}); err != nil {
			return err
		}

		return nil
	})
}

func (s *Store) WalkIDsByResult(resultID string, fn func(string) error) error {
	ids := map[string]struct{}{}
	if err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(byResultBucket))
		if b == nil {
			return nil
		}
		b = b.Bucket([]byte(resultID))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			ids[string(k)] = struct{}{}
			return nil
		})
	}); err != nil {
		return err
	}
	for id := range ids {
		if err := fn(id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Release(resultID string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(byResultBucket))
		if b == nil {
			return errors.WithStack(solver.ErrNotFound)
		}
		b = b.Bucket([]byte(resultID))
		if b == nil {
			return errors.WithStack(solver.ErrNotFound)
		}
		if err := b.ForEach(func(k, v []byte) error {
			return s.releaseHelper(tx, string(k), resultID)
		}); err != nil {
			return err
		}
		return nil
	})
}

func (s *Store) releaseHelper(tx *bolt.Tx, id, resultID string) error {
	results := tx.Bucket([]byte(resultBucket)).Bucket([]byte(id))
	if results == nil {
		return nil
	}

	if err := results.Delete([]byte(resultID)); err != nil {
		return err
	}

	ids := tx.Bucket([]byte(byResultBucket))

	ids = ids.Bucket([]byte(resultID))
	if ids == nil {
		return nil
	}

	if err := ids.Delete([]byte(resultID)); err != nil {
		return err
	}

	if isEmptyBucket(ids) {
		if err := tx.Bucket([]byte(byResultBucket)).DeleteBucket([]byte(resultID)); err != nil {
			return err
		}
	}

	links := tx.Bucket([]byte(resultBucket))
	if results == nil {
		return nil
	}
	links = links.Bucket([]byte(id))

	return s.emptyBranchWithParents(tx, []byte(id))
}

func (s *Store) emptyBranchWithParents(tx *bolt.Tx, id []byte) error {
	results := tx.Bucket([]byte(resultBucket)).Bucket(id)
	if results == nil {
		return nil
	}

	isEmptyLinks := true
	links := tx.Bucket([]byte(linksBucket)).Bucket(id)
	if links != nil {
		isEmptyLinks = isEmptyBucket(links)
	}

	if !isEmptyBucket(results) || !isEmptyLinks {
		return nil
	}

	if backlinks := tx.Bucket([]byte(backlinksBucket)).Bucket(id); backlinks != nil {
		if err := backlinks.ForEach(func(k, v []byte) error {
			if subLinks := tx.Bucket([]byte(linksBucket)).Bucket(k); subLinks != nil {
				if err := subLinks.ForEach(func(k, v []byte) error {
					parts := bytes.Split(k, []byte("@"))
					if len(parts) != 2 {
						return errors.Errorf("invalid key %s", k)
					}
					if bytes.Equal(id, parts[1]) {
						return subLinks.Delete(k)
					}
					return nil
				}); err != nil {
					return err
				}

				if isEmptyBucket(subLinks) {
					if err := tx.Bucket([]byte(linksBucket)).DeleteBucket(k); err != nil {
						return err
					}
				}
			}
			return s.emptyBranchWithParents(tx, k)
		}); err != nil {
			return err
		}
		if err := tx.Bucket([]byte(backlinksBucket)).DeleteBucket(id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) AddLink(id string, link solver.CacheInfoLink, target string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.Bucket([]byte(linksBucket)).CreateBucketIfNotExists([]byte(id))
		if err != nil {
			return err
		}

		dt, err := json.Marshal(link)
		if err != nil {
			return err
		}

		if err := b.Put(bytes.Join([][]byte{dt, []byte(target)}, []byte("@")), []byte{}); err != nil {
			return err
		}

		b, err = tx.Bucket([]byte(backlinksBucket)).CreateBucketIfNotExists([]byte(target))
		if err != nil {
			return err
		}

		if err := b.Put([]byte(id), []byte{}); err != nil {
			return err
		}

		return nil
	})
}

func (s *Store) WalkLinks(id string, link solver.CacheInfoLink, fn func(id string) error) error {
	var links []string
	if err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(linksBucket))
		if b == nil {
			return nil
		}
		b = b.Bucket([]byte(id))
		if b == nil {
			return nil
		}

		dt, err := json.Marshal(link)
		if err != nil {
			return err
		}
		index := bytes.Join([][]byte{dt, {}}, []byte("@"))
		c := b.Cursor()
		k, _ := c.Seek([]byte(index))
		for {
			if k != nil && bytes.HasPrefix(k, index) {
				target := bytes.TrimPrefix(k, index)
				links = append(links, string(target))
				k, _ = c.Next()
			} else {
				break
			}
		}

		return nil
	}); err != nil {
		return err
	}
	for _, l := range links {
		if err := fn(l); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) HasLink(id string, link solver.CacheInfoLink, target string) bool {
	var v bool
	if err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(linksBucket))
		if b == nil {
			return nil
		}
		b = b.Bucket([]byte(id))
		if b == nil {
			return nil
		}

		dt, err := json.Marshal(link)
		if err != nil {
			return err
		}
		v = b.Get(bytes.Join([][]byte{dt, []byte(target)}, []byte("@"))) != nil
		return nil
	}); err != nil {
		return false
	}
	return v
}

func (s *Store) WalkBacklinks(id string, fn func(id string, link solver.CacheInfoLink) error) error {
	var outIDs []string
	var outLinks []solver.CacheInfoLink

	if err := s.db.View(func(tx *bolt.Tx) error {
		links := tx.Bucket([]byte(linksBucket))
		if links == nil {
			return nil
		}
		backLinks := tx.Bucket([]byte(backlinksBucket))
		if backLinks == nil {
			return nil
		}
		b := backLinks.Bucket([]byte(id))
		if b == nil {
			return nil
		}

		if err := b.ForEach(func(bid, v []byte) error {
			b = links.Bucket(bid)
			if b == nil {
				return nil
			}
			if err := b.ForEach(func(k, v []byte) error {
				parts := bytes.Split(k, []byte("@"))
				if len(parts) == 2 {
					if string(parts[1]) != id {
						return nil
					}
					var l solver.CacheInfoLink
					if err := json.Unmarshal(parts[0], &l); err != nil {
						return err
					}
					l.Digest = digest.FromBytes([]byte(fmt.Sprintf("%s@%d", l.Digest, l.Output)))
					l.Output = 0
					outIDs = append(outIDs, string(bid))
					outLinks = append(outLinks, l)
				}
				return nil
			}); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	for i := range outIDs {
		if err := fn(outIDs[i], outLinks[i]); err != nil {
			return err
		}
	}
	return nil
}

func isEmptyBucket(b *bolt.Bucket) bool {
	if b == nil {
		return true
	}
	k, _ := b.Cursor().First()
	return k == nil
}
