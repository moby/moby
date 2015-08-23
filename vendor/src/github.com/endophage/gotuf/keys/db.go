package keys

import (
	"errors"

	"github.com/endophage/gotuf/data"
)

var (
	ErrWrongType        = errors.New("tuf: invalid key type")
	ErrExists           = errors.New("tuf: key already in db")
	ErrWrongID          = errors.New("tuf: key id mismatch")
	ErrInvalidKey       = errors.New("tuf: invalid key")
	ErrInvalidRole      = errors.New("tuf: invalid role")
	ErrInvalidKeyID     = errors.New("tuf: invalid key id")
	ErrInvalidThreshold = errors.New("tuf: invalid role threshold")
)

type KeyDB struct {
	roles map[string]*data.Role
	keys  map[string]data.PublicKey
}

func NewDB() *KeyDB {
	return &KeyDB{
		roles: make(map[string]*data.Role),
		keys:  make(map[string]data.PublicKey),
	}
}

func (db *KeyDB) AddKey(k data.PublicKey) {
	db.keys[k.ID()] = k
}

func (db *KeyDB) AddRole(r *data.Role) error {
	if !data.ValidRole(r.Name) {
		return ErrInvalidRole
	}
	if r.Threshold < 1 {
		return ErrInvalidThreshold
	}

	// validate all key ids are in the keys maps
	for _, id := range r.KeyIDs {
		if _, ok := db.keys[id]; !ok {
			return ErrInvalidKeyID
		}
	}

	db.roles[r.Name] = r
	return nil
}

func (db *KeyDB) GetKey(id string) data.PublicKey {
	return db.keys[id]
}

func (db *KeyDB) GetRole(name string) *data.Role {
	return db.roles[name]
}
