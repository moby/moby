package goavro

import (
	"fmt"
	"strings"
)

// ErrNotRecord is returned when codepath expecs goavro.Record, but found something else.
type ErrNotRecord struct {
	datum interface{}
}

// Error returns a string representation of the ErrNotRecord error.
func (e ErrNotRecord) Error() string {
	return fmt.Sprintf("expected to find *goavro.Record, but found %T", e.datum)
}

// RecordCache provides simplified way of getting a value from a nested field, while memoizing
// intermediate results in a cache for future lookups.
type RecordCache struct {
	db    map[string]interface{}
	top   *Record
	delim byte
}

// NewRecordCache returns a new RecordCache structure used to get values from nested fields.
//
//    func example(codec Codec, someReader io.Reader) (string, error) {
//    	decoded, err := codec.Decode(someReader)
//    	record, ok := decoded.(*Record)
//    	if !ok {
//    		return "", ErrNotRecord{decoded}
//    	}
//
//    	rc, err := NewRecordCache(record, '/')
//    	if err != nil {
//    		return "", err
//    	}
//    	account, err := rc.Get("com.example.user/com.example.account")
//    	if err != nil {
//    		return "", err
//    	}
//    	s, ok := account.(string)
//    	if !ok {
//    		return "", fmt.Errorf("expected: string; actual: %T", account)
//    	}
//    	return s, nil
//    }
func NewRecordCache(record *Record, delim byte) (*RecordCache, error) {
	return &RecordCache{delim: delim, db: make(map[string]interface{}), top: record}, nil
}

// Get splits the specified name by the stored delimiter, and attempts to retrieve the nested value
// corresponding to the nested fields.
func (rc *RecordCache) Get(name string) (interface{}, error) {
	if val, ok := rc.db[name]; ok {
		return val, nil
	}

	var err error
	var val interface{}
	var parent interface{} = rc.top
	var parentName, childName string

	if index := strings.LastIndexByte(name, rc.delim); index >= 0 {
		parentName, childName = name[:index], name[index+1:]
		parent, err = rc.Get(parentName)
		if err != nil {
			return nil, err
		}
		rc.db[parentName] = parent
		if _, ok := parent.(*Record); !ok {
			return nil, ErrNotRecord{datum: parent}
		}
		name = childName
	}

	val, err = parent.(*Record).GetQualified(name)
	if err != nil {
		if nerr, ok := err.(ErrNoSuchField); ok {
			nerr.path = parentName
			return nil, nerr
		}
		return nil, err
	}

	rc.db[name] = val
	return val, nil
}
