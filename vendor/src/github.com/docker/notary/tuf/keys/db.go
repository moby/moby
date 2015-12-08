package keys

import (
	"errors"

	"github.com/docker/notary/tuf/data"
)

// Various basic key database errors
var (
	ErrWrongType        = errors.New("tuf: invalid key type")
	ErrExists           = errors.New("tuf: key already in db")
	ErrWrongID          = errors.New("tuf: key id mismatch")
	ErrInvalidKey       = errors.New("tuf: invalid key")
	ErrInvalidKeyID     = errors.New("tuf: invalid key id")
	ErrInvalidThreshold = errors.New("tuf: invalid role threshold")
)

// KeyDB is an in memory database of public keys and role associations.
// It is populated when parsing TUF files and used during signature
// verification to look up the keys for a given role
type KeyDB struct {
	roles map[string]*data.Role
	keys  map[string]data.PublicKey
}

// NewDB initializes an empty KeyDB
func NewDB() *KeyDB {
	return &KeyDB{
		roles: make(map[string]*data.Role),
		keys:  make(map[string]data.PublicKey),
	}
}

// AddKey adds a public key to the database
func (db *KeyDB) AddKey(k data.PublicKey) {
	db.keys[k.ID()] = k
}

// AddRole adds a role to the database. Any keys associated with the
// role must have already been added.
func (db *KeyDB) AddRole(r *data.Role) error {
	if !data.ValidRole(r.Name) {
		return data.ErrInvalidRole{Role: r.Name}
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

// GetKey pulls a key out of the database by its ID
func (db *KeyDB) GetKey(id string) data.PublicKey {
	return db.keys[id]
}

// GetRole retrieves a role based on its name
func (db *KeyDB) GetRole(name string) *data.Role {
	return db.roles[name]
}
