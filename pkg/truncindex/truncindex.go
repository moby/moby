package truncindex

import (
	"fmt"
	"index/suffixarray"
	"strings"
	"sync"
)

// TruncIndex allows the retrieval of string identifiers by any of their unique prefixes.
// This is used to retrieve image and container IDs by more convenient shorthand prefixes.
type TruncIndex struct {
	sync.RWMutex
	index *suffixarray.Index
	ids   map[string]bool
	bytes []byte
}

func NewTruncIndex(ids []string) (idx *TruncIndex) {
	idx = &TruncIndex{
		ids:   make(map[string]bool),
		bytes: []byte{' '},
	}
	for _, id := range ids {
		idx.ids[id] = true
		idx.bytes = append(idx.bytes, []byte(id+" ")...)
	}
	idx.index = suffixarray.New(idx.bytes)
	return
}

func (idx *TruncIndex) addId(id string) error {
	if strings.Contains(id, " ") {
		return fmt.Errorf("Illegal character: ' '")
	}
	if _, exists := idx.ids[id]; exists {
		return fmt.Errorf("Id already exists: %s", id)
	}
	idx.ids[id] = true
	idx.bytes = append(idx.bytes, []byte(id+" ")...)
	return nil
}

func (idx *TruncIndex) Add(id string) error {
	idx.Lock()
	defer idx.Unlock()
	if err := idx.addId(id); err != nil {
		return err
	}
	idx.index = suffixarray.New(idx.bytes)
	return nil
}

func (idx *TruncIndex) AddWithoutSuffixarrayUpdate(id string) error {
	idx.Lock()
	defer idx.Unlock()
	return idx.addId(id)
}

func (idx *TruncIndex) UpdateSuffixarray() {
	idx.Lock()
	defer idx.Unlock()
	idx.index = suffixarray.New(idx.bytes)
}

func (idx *TruncIndex) Delete(id string) error {
	idx.Lock()
	defer idx.Unlock()
	if _, exists := idx.ids[id]; !exists {
		return fmt.Errorf("No such id: %s", id)
	}
	before, after, err := idx.lookup(id)
	if err != nil {
		return err
	}
	delete(idx.ids, id)
	idx.bytes = append(idx.bytes[:before], idx.bytes[after:]...)
	idx.index = suffixarray.New(idx.bytes)
	return nil
}

func (idx *TruncIndex) lookup(s string) (int, int, error) {
	offsets := idx.index.Lookup([]byte(" "+s), -1)
	//log.Printf("lookup(%s): %v (index bytes: '%s')\n", s, offsets, idx.index.Bytes())
	if offsets == nil || len(offsets) == 0 || len(offsets) > 1 {
		return -1, -1, fmt.Errorf("No such id: %s", s)
	}
	offsetBefore := offsets[0] + 1
	offsetAfter := offsetBefore + strings.Index(string(idx.bytes[offsetBefore:]), " ")
	return offsetBefore, offsetAfter, nil
}

func (idx *TruncIndex) Get(s string) (string, error) {
	idx.RLock()
	defer idx.RUnlock()
	before, after, err := idx.lookup(s)
	//log.Printf("Get(%s) bytes=|%s| before=|%d| after=|%d|\n", s, idx.bytes, before, after)
	if err != nil {
		return "", err
	}
	return string(idx.bytes[before:after]), err
}
