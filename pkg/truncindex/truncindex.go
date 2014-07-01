package truncindex

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/tchap/go-patricia/patricia"
)

var (
	ErrNoID = errors.New("prefix can't be empty")
)

func init() {
	// Change patricia max prefix per node length,
	// because our len(ID) always 64
	patricia.MaxPrefixPerNode = 64
}

// TruncIndex allows the retrieval of string identifiers by any of their unique prefixes.
// This is used to retrieve image and container IDs by more convenient shorthand prefixes.
type TruncIndex struct {
	sync.RWMutex
	trie *patricia.Trie
	ids  map[string]struct{}
}

func NewTruncIndex(ids []string) (idx *TruncIndex) {
	idx = &TruncIndex{
		ids:  make(map[string]struct{}),
		trie: patricia.NewTrie(),
	}
	for _, id := range ids {
		idx.addId(id)
	}
	return
}

func (idx *TruncIndex) addId(id string) error {
	if strings.Contains(id, " ") {
		return fmt.Errorf("Illegal character: ' '")
	}
	if id == "" {
		return ErrNoID
	}
	if _, exists := idx.ids[id]; exists {
		return fmt.Errorf("Id already exists: '%s'", id)
	}
	idx.ids[id] = struct{}{}
	if inserted := idx.trie.Insert(patricia.Prefix(id), struct{}{}); !inserted {
		return fmt.Errorf("Failed to insert id: %s", id)
	}
	return nil
}

func (idx *TruncIndex) Add(id string) error {
	idx.Lock()
	defer idx.Unlock()
	if err := idx.addId(id); err != nil {
		return err
	}
	return nil
}

func (idx *TruncIndex) Delete(id string) error {
	idx.Lock()
	defer idx.Unlock()
	if _, exists := idx.ids[id]; !exists || id == "" {
		return fmt.Errorf("No such id: '%s'", id)
	}
	delete(idx.ids, id)
	if deleted := idx.trie.Delete(patricia.Prefix(id)); !deleted {
		return fmt.Errorf("No such id: '%s'", id)
	}
	return nil
}

func (idx *TruncIndex) Get(s string) (string, error) {
	idx.RLock()
	defer idx.RUnlock()
	var (
		id string
	)
	if s == "" {
		return "", ErrNoID
	}
	subTreeVisitFunc := func(prefix patricia.Prefix, item patricia.Item) error {
		if id != "" {
			// we haven't found the ID if there are two or more IDs
			id = ""
			return fmt.Errorf("we've found two entries")
		}
		id = string(prefix)
		return nil
	}

	if err := idx.trie.VisitSubtree(patricia.Prefix(s), subTreeVisitFunc); err != nil {
		return "", fmt.Errorf("No such id: %s", s)
	}
	if id != "" {
		return id, nil
	}
	return "", fmt.Errorf("No such id: %s", s)
}
