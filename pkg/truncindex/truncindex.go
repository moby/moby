// Package truncindex provides a general 'index tree', used by Docker
// in order to be able to reference containers by only a few unambiguous
// characters of their id.
package truncindex

import (
	"errors"
	"strings"
	"sync"

	"github.com/tchap/go-patricia/patricia"
)

var (
	// ErrEmptyPrefix is returned if the prefix was empty.
	ErrEmptyPrefix = errors.New("Prefix can't be empty")

	// ErrAmbiguousPrefix is returned if the prefix was ambiguous
	// (multiple ids for the prefix).
	ErrAmbiguousPrefix = errors.New("Multiple IDs found with provided prefix")

	// ErrIllegalChar is returned when a space is in the ID
	ErrIllegalChar = errors.New("illegal character: ' '")

	// ErrNoSuchID is returned if the TruncIndex doesn't contain the given id
	ErrNoSuchID = errors.New("no such id")

	// ErrIDAlreadyExists is returned if the id is already in the TruncIndex
	ErrIDAlreadyExists = errors.New("id already exists")

	// ErrInsertionFailed is returned if something went wrong inserting the given
	// id in the TruncIndex
	ErrInsertionFailed = errors.New("failed to insert id")
)

// TruncIndex allows the retrieval of string identifiers by any of their unique prefixes.
// This is used to retrieve image and container IDs by more convenient shorthand prefixes.
type TruncIndex struct {
	sync.RWMutex
	trie *patricia.Trie
	ids  map[string]struct{}
}

// NewTruncIndex creates a new TruncIndex and initializes it with a list of IDs.
func NewTruncIndex(ids []string) (idx *TruncIndex) {
	idx = &TruncIndex{
		ids: make(map[string]struct{}),

		// Change patricia max prefix per node length,
		// because our len(ID) always 64
		trie: patricia.NewTrie(patricia.MaxPrefixPerNode(64)),
	}
	for _, id := range ids {
		idx.addID(id)
	}
	return
}

func (idx *TruncIndex) addID(id string) error {
	if strings.Contains(id, " ") {
		return ErrIllegalChar
	}
	if id == "" {
		return ErrEmptyPrefix
	}
	if _, exists := idx.ids[id]; exists {
		return ErrIDAlreadyExists
	}
	idx.ids[id] = struct{}{}
	if inserted := idx.trie.Insert(patricia.Prefix(id), struct{}{}); !inserted {
		return ErrInsertionFailed
	}
	return nil
}

// Add adds a new ID to the TruncIndex.
func (idx *TruncIndex) Add(id string) error {
	idx.Lock()
	defer idx.Unlock()
	if err := idx.addID(id); err != nil {
		return err
	}
	return nil
}

// Delete removes an ID from the TruncIndex. If there are multiple IDs
// with the given prefix, an error is thrown.
func (idx *TruncIndex) Delete(id string) error {
	idx.Lock()
	defer idx.Unlock()
	if _, exists := idx.ids[id]; !exists || id == "" {
		return ErrNoSuchID
	}
	delete(idx.ids, id)
	if deleted := idx.trie.Delete(patricia.Prefix(id)); !deleted {
		return ErrNoSuchID
	}
	return nil
}

// Get retrieves an ID from the TruncIndex. If there are multiple IDs
// with the given prefix, an error is thrown.
func (idx *TruncIndex) Get(s string) (string, error) {
	if s == "" {
		return "", ErrEmptyPrefix
	}
	var (
		id string
	)
	subTreeVisitFunc := func(prefix patricia.Prefix, item patricia.Item) error {
		if id != "" {
			// we haven't found the ID if there are two or more IDs
			id = ""
			return ErrAmbiguousPrefix
		}
		id = string(prefix)
		return nil
	}

	idx.RLock()
	defer idx.RUnlock()
	if err := idx.trie.VisitSubtree(patricia.Prefix(s), subTreeVisitFunc); err != nil {
		return "", err
	}
	if id != "" {
		return id, nil
	}
	return "", ErrNoSuchID
}

// Iterate iterates over all stored IDs, and passes each of them to the given handler.
func (idx *TruncIndex) Iterate(handler func(id string)) {
	idx.trie.Visit(func(prefix patricia.Prefix, item patricia.Item) error {
		handler(string(prefix))
		return nil
	})
}
