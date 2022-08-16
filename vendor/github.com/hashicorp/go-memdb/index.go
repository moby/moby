package memdb

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/bits"
	"reflect"
	"strings"
)

// Indexer is an interface used for defining indexes. Indexes are used
// for efficient lookup of objects in a MemDB table. An Indexer must also
// implement one of SingleIndexer or MultiIndexer.
//
// Indexers are primarily responsible for returning the lookup key as
// a byte slice. The byte slice is the key data in the underlying data storage.
type Indexer interface {
	// FromArgs is called to build the exact index key from a list of arguments.
	FromArgs(args ...interface{}) ([]byte, error)
}

// SingleIndexer is an interface used for defining indexes that generate a
// single value per object
type SingleIndexer interface {
	// FromObject extracts the index value from an object. The return values
	// are whether the index value was found, the index value, and any error
	// while extracting the index value, respectively.
	FromObject(raw interface{}) (bool, []byte, error)
}

// MultiIndexer is an interface used for defining indexes that generate
// multiple values per object. Each value is stored as a seperate index
// pointing to the same object.
//
// For example, an index that extracts the first and last name of a person
// and allows lookup based on eitherd would be a MultiIndexer. The FromObject
// of this example would split the first and last name and return both as
// values.
type MultiIndexer interface {
	// FromObject extracts index values from an object. The return values
	// are the same as a SingleIndexer except there can be multiple index
	// values.
	FromObject(raw interface{}) (bool, [][]byte, error)
}

// PrefixIndexer is an optional interface on top of an Indexer that allows
// indexes to support prefix-based iteration.
type PrefixIndexer interface {
	// PrefixFromArgs is the same as FromArgs for an Indexer except that
	// the index value returned should return all prefix-matched values.
	PrefixFromArgs(args ...interface{}) ([]byte, error)
}

// StringFieldIndex is used to extract a field from an object
// using reflection and builds an index on that field.
type StringFieldIndex struct {
	Field     string
	Lowercase bool
}

func (s *StringFieldIndex) FromObject(obj interface{}) (bool, []byte, error) {
	v := reflect.ValueOf(obj)
	v = reflect.Indirect(v) // Dereference the pointer if any

	fv := v.FieldByName(s.Field)
	isPtr := fv.Kind() == reflect.Ptr
	fv = reflect.Indirect(fv)
	if !isPtr && !fv.IsValid() {
		return false, nil,
			fmt.Errorf("field '%s' for %#v is invalid %v ", s.Field, obj, isPtr)
	}

	if isPtr && !fv.IsValid() {
		val := ""
		return false, []byte(val), nil
	}

	val := fv.String()
	if val == "" {
		return false, nil, nil
	}

	if s.Lowercase {
		val = strings.ToLower(val)
	}

	// Add the null character as a terminator
	val += "\x00"
	return true, []byte(val), nil
}

func (s *StringFieldIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	arg, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}
	if s.Lowercase {
		arg = strings.ToLower(arg)
	}
	// Add the null character as a terminator
	arg += "\x00"
	return []byte(arg), nil
}

func (s *StringFieldIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	val, err := s.FromArgs(args...)
	if err != nil {
		return nil, err
	}

	// Strip the null terminator, the rest is a prefix
	n := len(val)
	if n > 0 {
		return val[:n-1], nil
	}
	return val, nil
}

// StringSliceFieldIndex builds an index from a field on an object that is a
// string slice ([]string). Each value within the string slice can be used for
// lookup.
type StringSliceFieldIndex struct {
	Field     string
	Lowercase bool
}

func (s *StringSliceFieldIndex) FromObject(obj interface{}) (bool, [][]byte, error) {
	v := reflect.ValueOf(obj)
	v = reflect.Indirect(v) // Dereference the pointer if any

	fv := v.FieldByName(s.Field)
	if !fv.IsValid() {
		return false, nil,
			fmt.Errorf("field '%s' for %#v is invalid", s.Field, obj)
	}

	if fv.Kind() != reflect.Slice || fv.Type().Elem().Kind() != reflect.String {
		return false, nil, fmt.Errorf("field '%s' is not a string slice", s.Field)
	}

	length := fv.Len()
	vals := make([][]byte, 0, length)
	for i := 0; i < fv.Len(); i++ {
		val := fv.Index(i).String()
		if val == "" {
			continue
		}

		if s.Lowercase {
			val = strings.ToLower(val)
		}

		// Add the null character as a terminator
		val += "\x00"
		vals = append(vals, []byte(val))
	}
	if len(vals) == 0 {
		return false, nil, nil
	}
	return true, vals, nil
}

func (s *StringSliceFieldIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	arg, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}
	if s.Lowercase {
		arg = strings.ToLower(arg)
	}
	// Add the null character as a terminator
	arg += "\x00"
	return []byte(arg), nil
}

func (s *StringSliceFieldIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	val, err := s.FromArgs(args...)
	if err != nil {
		return nil, err
	}

	// Strip the null terminator, the rest is a prefix
	n := len(val)
	if n > 0 {
		return val[:n-1], nil
	}
	return val, nil
}

// StringMapFieldIndex is used to extract a field of type map[string]string
// from an object using reflection and builds an index on that field.
//
// Note that although FromArgs in theory supports using either one or
// two arguments, there is a bug: FromObject only creates an index
// using key/value, and does not also create an index using key. This
// means a lookup using one argument will never actually work.
//
// It is currently left as-is to prevent backwards compatibility
// issues.
//
// TODO: Fix this in the next major bump.
type StringMapFieldIndex struct {
	Field     string
	Lowercase bool
}

var MapType = reflect.MapOf(reflect.TypeOf(""), reflect.TypeOf("")).Kind()

func (s *StringMapFieldIndex) FromObject(obj interface{}) (bool, [][]byte, error) {
	v := reflect.ValueOf(obj)
	v = reflect.Indirect(v) // Dereference the pointer if any

	fv := v.FieldByName(s.Field)
	if !fv.IsValid() {
		return false, nil, fmt.Errorf("field '%s' for %#v is invalid", s.Field, obj)
	}

	if fv.Kind() != MapType {
		return false, nil, fmt.Errorf("field '%s' is not a map[string]string", s.Field)
	}

	length := fv.Len()
	vals := make([][]byte, 0, length)
	for _, key := range fv.MapKeys() {
		k := key.String()
		if k == "" {
			continue
		}
		val := fv.MapIndex(key).String()

		if s.Lowercase {
			k = strings.ToLower(k)
			val = strings.ToLower(val)
		}

		// Add the null character as a terminator
		k += "\x00" + val + "\x00"

		vals = append(vals, []byte(k))
	}
	if len(vals) == 0 {
		return false, nil, nil
	}
	return true, vals, nil
}

// WARNING: Because of a bug in FromObject, this function will never return
// a value when using the single-argument version.
func (s *StringMapFieldIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) > 2 || len(args) == 0 {
		return nil, fmt.Errorf("must provide one or two arguments")
	}
	key, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}
	if s.Lowercase {
		key = strings.ToLower(key)
	}
	// Add the null character as a terminator
	key += "\x00"

	if len(args) == 2 {
		val, ok := args[1].(string)
		if !ok {
			return nil, fmt.Errorf("argument must be a string: %#v", args[1])
		}
		if s.Lowercase {
			val = strings.ToLower(val)
		}
		// Add the null character as a terminator
		key += val + "\x00"
	}

	return []byte(key), nil
}

// IntFieldIndex is used to extract an int field from an object using
// reflection and builds an index on that field.
type IntFieldIndex struct {
	Field string
}

func (i *IntFieldIndex) FromObject(obj interface{}) (bool, []byte, error) {
	v := reflect.ValueOf(obj)
	v = reflect.Indirect(v) // Dereference the pointer if any

	fv := v.FieldByName(i.Field)
	if !fv.IsValid() {
		return false, nil,
			fmt.Errorf("field '%s' for %#v is invalid", i.Field, obj)
	}

	// Check the type
	k := fv.Kind()
	size, ok := IsIntType(k)
	if !ok {
		return false, nil, fmt.Errorf("field %q is of type %v; want an int", i.Field, k)
	}

	// Get the value and encode it
	val := fv.Int()
	buf := make([]byte, size)
	binary.PutVarint(buf, val)

	return true, buf, nil
}

func (i *IntFieldIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}

	v := reflect.ValueOf(args[0])
	if !v.IsValid() {
		return nil, fmt.Errorf("%#v is invalid", args[0])
	}

	k := v.Kind()
	size, ok := IsIntType(k)
	if !ok {
		return nil, fmt.Errorf("arg is of type %v; want a int", k)
	}

	val := v.Int()
	buf := make([]byte, size)
	binary.PutVarint(buf, val)

	return buf, nil
}

// IsIntType returns whether the passed type is a type of int and the number
// of bytes needed to encode the type.
func IsIntType(k reflect.Kind) (size int, okay bool) {
	switch k {
	case reflect.Int:
		return binary.MaxVarintLen64, true
	case reflect.Int8:
		return 2, true
	case reflect.Int16:
		return binary.MaxVarintLen16, true
	case reflect.Int32:
		return binary.MaxVarintLen32, true
	case reflect.Int64:
		return binary.MaxVarintLen64, true
	default:
		return 0, false
	}
}

// UintFieldIndex is used to extract a uint field from an object using
// reflection and builds an index on that field.
type UintFieldIndex struct {
	Field string
}

func (u *UintFieldIndex) FromObject(obj interface{}) (bool, []byte, error) {
	v := reflect.ValueOf(obj)
	v = reflect.Indirect(v) // Dereference the pointer if any

	fv := v.FieldByName(u.Field)
	if !fv.IsValid() {
		return false, nil,
			fmt.Errorf("field '%s' for %#v is invalid", u.Field, obj)
	}

	// Check the type
	k := fv.Kind()
	size, ok := IsUintType(k)
	if !ok {
		return false, nil, fmt.Errorf("field %q is of type %v; want a uint", u.Field, k)
	}

	// Get the value and encode it
	val := fv.Uint()
	buf := encodeUInt(val, size)

	return true, buf, nil
}

func (u *UintFieldIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}

	v := reflect.ValueOf(args[0])
	if !v.IsValid() {
		return nil, fmt.Errorf("%#v is invalid", args[0])
	}

	k := v.Kind()
	size, ok := IsUintType(k)
	if !ok {
		return nil, fmt.Errorf("arg is of type %v; want a uint", k)
	}

	val := v.Uint()
	buf := encodeUInt(val, size)

	return buf, nil
}

func encodeUInt(val uint64, size int) []byte {
	buf := make([]byte, size)

	switch size {
	case 1:
		buf[0] = uint8(val)
	case 2:
		binary.BigEndian.PutUint16(buf, uint16(val))
	case 4:
		binary.BigEndian.PutUint32(buf, uint32(val))
	case 8:
		binary.BigEndian.PutUint64(buf, val)
	}

	return buf
}

// IsUintType returns whether the passed type is a type of uint and the number
// of bytes needed to encode the type.
func IsUintType(k reflect.Kind) (size int, okay bool) {
	switch k {
	case reflect.Uint:
		return bits.UintSize / 8, true
	case reflect.Uint8:
		return 1, true
	case reflect.Uint16:
		return 2, true
	case reflect.Uint32:
		return 4, true
	case reflect.Uint64:
		return 8, true
	default:
		return 0, false
	}
}

// BoolFieldIndex is used to extract an boolean field from an object using
// reflection and builds an index on that field.
type BoolFieldIndex struct {
	Field string
}

func (i *BoolFieldIndex) FromObject(obj interface{}) (bool, []byte, error) {
	v := reflect.ValueOf(obj)
	v = reflect.Indirect(v) // Dereference the pointer if any

	fv := v.FieldByName(i.Field)
	if !fv.IsValid() {
		return false, nil,
			fmt.Errorf("field '%s' for %#v is invalid", i.Field, obj)
	}

	// Check the type
	k := fv.Kind()
	if k != reflect.Bool {
		return false, nil, fmt.Errorf("field %q is of type %v; want a bool", i.Field, k)
	}

	// Get the value and encode it
	buf := make([]byte, 1)
	if fv.Bool() {
		buf[0] = 1
	}

	return true, buf, nil
}

func (i *BoolFieldIndex) FromArgs(args ...interface{}) ([]byte, error) {
	return fromBoolArgs(args)
}

// UUIDFieldIndex is used to extract a field from an object
// using reflection and builds an index on that field by treating
// it as a UUID. This is an optimization to using a StringFieldIndex
// as the UUID can be more compactly represented in byte form.
type UUIDFieldIndex struct {
	Field string
}

func (u *UUIDFieldIndex) FromObject(obj interface{}) (bool, []byte, error) {
	v := reflect.ValueOf(obj)
	v = reflect.Indirect(v) // Dereference the pointer if any

	fv := v.FieldByName(u.Field)
	if !fv.IsValid() {
		return false, nil,
			fmt.Errorf("field '%s' for %#v is invalid", u.Field, obj)
	}

	val := fv.String()
	if val == "" {
		return false, nil, nil
	}

	buf, err := u.parseString(val, true)
	return true, buf, err
}

func (u *UUIDFieldIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	switch arg := args[0].(type) {
	case string:
		return u.parseString(arg, true)
	case []byte:
		if len(arg) != 16 {
			return nil, fmt.Errorf("byte slice must be 16 characters")
		}
		return arg, nil
	default:
		return nil,
			fmt.Errorf("argument must be a string or byte slice: %#v", args[0])
	}
}

func (u *UUIDFieldIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	switch arg := args[0].(type) {
	case string:
		return u.parseString(arg, false)
	case []byte:
		return arg, nil
	default:
		return nil,
			fmt.Errorf("argument must be a string or byte slice: %#v", args[0])
	}
}

// parseString parses a UUID from the string. If enforceLength is false, it will
// parse a partial UUID. An error is returned if the input, stripped of hyphens,
// is not even length.
func (u *UUIDFieldIndex) parseString(s string, enforceLength bool) ([]byte, error) {
	// Verify the length
	l := len(s)
	if enforceLength && l != 36 {
		return nil, fmt.Errorf("UUID must be 36 characters")
	} else if l > 36 {
		return nil, fmt.Errorf("Invalid UUID length. UUID have 36 characters; got %d", l)
	}

	hyphens := strings.Count(s, "-")
	if hyphens > 4 {
		return nil, fmt.Errorf(`UUID should have maximum of 4 "-"; got %d`, hyphens)
	}

	// The sanitized length is the length of the original string without the "-".
	sanitized := strings.Replace(s, "-", "", -1)
	sanitizedLength := len(sanitized)
	if sanitizedLength%2 != 0 {
		return nil, fmt.Errorf("Input (without hyphens) must be even length")
	}

	dec, err := hex.DecodeString(sanitized)
	if err != nil {
		return nil, fmt.Errorf("Invalid UUID: %v", err)
	}

	return dec, nil
}

// FieldSetIndex is used to extract a field from an object using reflection and
// builds an index on whether the field is set by comparing it against its
// type's nil value.
type FieldSetIndex struct {
	Field string
}

func (f *FieldSetIndex) FromObject(obj interface{}) (bool, []byte, error) {
	v := reflect.ValueOf(obj)
	v = reflect.Indirect(v) // Dereference the pointer if any

	fv := v.FieldByName(f.Field)
	if !fv.IsValid() {
		return false, nil,
			fmt.Errorf("field '%s' for %#v is invalid", f.Field, obj)
	}

	if fv.Interface() == reflect.Zero(fv.Type()).Interface() {
		return true, []byte{0}, nil
	}

	return true, []byte{1}, nil
}

func (f *FieldSetIndex) FromArgs(args ...interface{}) ([]byte, error) {
	return fromBoolArgs(args)
}

// ConditionalIndex builds an index based on a condition specified by a passed
// user function. This function may examine the passed object and return a
// boolean to encapsulate an arbitrarily complex conditional.
type ConditionalIndex struct {
	Conditional ConditionalIndexFunc
}

// ConditionalIndexFunc is the required function interface for a
// ConditionalIndex.
type ConditionalIndexFunc func(obj interface{}) (bool, error)

func (c *ConditionalIndex) FromObject(obj interface{}) (bool, []byte, error) {
	// Call the user's function
	res, err := c.Conditional(obj)
	if err != nil {
		return false, nil, fmt.Errorf("ConditionalIndexFunc(%#v) failed: %v", obj, err)
	}

	if res {
		return true, []byte{1}, nil
	}

	return true, []byte{0}, nil
}

func (c *ConditionalIndex) FromArgs(args ...interface{}) ([]byte, error) {
	return fromBoolArgs(args)
}

// fromBoolArgs is a helper that expects only a single boolean argument and
// returns a single length byte array containing either a one or zero depending
// on whether the passed input is true or false respectively.
func fromBoolArgs(args []interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}

	if val, ok := args[0].(bool); !ok {
		return nil, fmt.Errorf("argument must be a boolean type: %#v", args[0])
	} else if val {
		return []byte{1}, nil
	}

	return []byte{0}, nil
}

// CompoundIndex is used to build an index using multiple sub-indexes
// Prefix based iteration is supported as long as the appropriate prefix
// of indexers support it. All sub-indexers are only assumed to expect
// a single argument.
type CompoundIndex struct {
	Indexes []Indexer

	// AllowMissing results in an index based on only the indexers
	// that return data. If true, you may end up with 2/3 columns
	// indexed which might be useful for an index scan. Otherwise,
	// the CompoundIndex requires all indexers to be satisfied.
	AllowMissing bool
}

func (c *CompoundIndex) FromObject(raw interface{}) (bool, []byte, error) {
	var out []byte
	for i, idxRaw := range c.Indexes {
		idx, ok := idxRaw.(SingleIndexer)
		if !ok {
			return false, nil, fmt.Errorf("sub-index %d error: %s", i, "sub-index must be a SingleIndexer")
		}
		ok, val, err := idx.FromObject(raw)
		if err != nil {
			return false, nil, fmt.Errorf("sub-index %d error: %v", i, err)
		}
		if !ok {
			if c.AllowMissing {
				break
			} else {
				return false, nil, nil
			}
		}
		out = append(out, val...)
	}
	return true, out, nil
}

func (c *CompoundIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != len(c.Indexes) {
		return nil, fmt.Errorf("non-equivalent argument count and index fields")
	}
	var out []byte
	for i, arg := range args {
		val, err := c.Indexes[i].FromArgs(arg)
		if err != nil {
			return nil, fmt.Errorf("sub-index %d error: %v", i, err)
		}
		out = append(out, val...)
	}
	return out, nil
}

func (c *CompoundIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	if len(args) > len(c.Indexes) {
		return nil, fmt.Errorf("more arguments than index fields")
	}
	var out []byte
	for i, arg := range args {
		if i+1 < len(args) {
			val, err := c.Indexes[i].FromArgs(arg)
			if err != nil {
				return nil, fmt.Errorf("sub-index %d error: %v", i, err)
			}
			out = append(out, val...)
		} else {
			prefixIndexer, ok := c.Indexes[i].(PrefixIndexer)
			if !ok {
				return nil, fmt.Errorf("sub-index %d does not support prefix scanning", i)
			}
			val, err := prefixIndexer.PrefixFromArgs(arg)
			if err != nil {
				return nil, fmt.Errorf("sub-index %d error: %v", i, err)
			}
			out = append(out, val...)
		}
	}
	return out, nil
}

// CompoundMultiIndex is used to build an index using multiple
// sub-indexes.
//
// Unlike CompoundIndex, CompoundMultiIndex can have both
// SingleIndexer and MultiIndexer sub-indexers. However, each
// MultiIndexer adds considerable overhead/complexity in terms of
// the number of indexes created under-the-hood. It is not suggested
// to use more than one or two, if possible.
//
// Another change from CompoundIndexer is that if AllowMissing is
// set, not only is it valid to have empty index fields, but it will
// still create index values up to the first empty index. This means
// that if you have a value with an empty field, rather than using a
// prefix for lookup, you can simply pass in less arguments. As an
// example, if {Foo, Bar} is indexed but Bar is missing for a value
// and AllowMissing is set, an index will still be created for {Foo}
// and it is valid to do a lookup passing in only Foo as an argument.
// Note that the ordering isn't guaranteed -- it's last-insert wins,
// but this is true if you have two objects that have the same
// indexes not using AllowMissing anyways.
//
// Because StringMapFieldIndexers can take a varying number of args,
// it is currently a requirement that whenever it is used, two
// arguments must _always_ be provided for it. In theory we only
// need one, except a bug in that indexer means the single-argument
// version will never work. You can leave the second argument nil,
// but it will never produce a value. We support this for whenever
// that bug is fixed, likely in a next major version bump.
//
// Prefix-based indexing is not currently supported.
type CompoundMultiIndex struct {
	Indexes []Indexer

	// AllowMissing results in an index based on only the indexers
	// that return data. If true, you may end up with 2/3 columns
	// indexed which might be useful for an index scan. Otherwise,
	// CompoundMultiIndex requires all indexers to be satisfied.
	AllowMissing bool
}

func (c *CompoundMultiIndex) FromObject(raw interface{}) (bool, [][]byte, error) {
	// At each entry, builder is storing the results from the next index
	builder := make([][][]byte, 0, len(c.Indexes))
	// Start with something higher to avoid resizing if possible
	out := make([][]byte, 0, len(c.Indexes)^3)

forloop:
	// This loop goes through each indexer and adds the value(s) provided to the next
	// entry in the slice. We can then later walk it like a tree to construct the indices.
	for i, idxRaw := range c.Indexes {
		switch idx := idxRaw.(type) {
		case SingleIndexer:
			ok, val, err := idx.FromObject(raw)
			if err != nil {
				return false, nil, fmt.Errorf("single sub-index %d error: %v", i, err)
			}
			if !ok {
				if c.AllowMissing {
					break forloop
				} else {
					return false, nil, nil
				}
			}
			builder = append(builder, [][]byte{val})

		case MultiIndexer:
			ok, vals, err := idx.FromObject(raw)
			if err != nil {
				return false, nil, fmt.Errorf("multi sub-index %d error: %v", i, err)
			}
			if !ok {
				if c.AllowMissing {
					break forloop
				} else {
					return false, nil, nil
				}
			}

			// Add each of the new values to each of the old values
			builder = append(builder, vals)

		default:
			return false, nil, fmt.Errorf("sub-index %d does not satisfy either SingleIndexer or MultiIndexer", i)
		}
	}

	// We are walking through the builder slice essentially in a depth-first fashion,
	// building the prefix and leaves as we go. If AllowMissing is false, we only insert
	// these full paths to leaves. Otherwise, we also insert each prefix along the way.
	// This allows for lookup in FromArgs when AllowMissing is true that does not contain
	// the full set of arguments. e.g. for {Foo, Bar} where an object has only the Foo
	// field specified as "abc", it is valid to call FromArgs with just "abc".
	var walkVals func([]byte, int)
	walkVals = func(currPrefix []byte, depth int) {
		if depth == len(builder)-1 {
			// These are the "leaves", so append directly
			for _, v := range builder[depth] {
				out = append(out, append(currPrefix, v...))
			}
			return
		}
		for _, v := range builder[depth] {
			nextPrefix := append(currPrefix, v...)
			if c.AllowMissing {
				out = append(out, nextPrefix)
			}
			walkVals(nextPrefix, depth+1)
		}
	}

	walkVals(nil, 0)

	return true, out, nil
}

func (c *CompoundMultiIndex) FromArgs(args ...interface{}) ([]byte, error) {
	var stringMapCount int
	var argCount int
	for _, index := range c.Indexes {
		if argCount >= len(args) {
			break
		}
		if _, ok := index.(*StringMapFieldIndex); ok {
			// We require pairs for StringMapFieldIndex, but only got one
			if argCount+1 >= len(args) {
				return nil, errors.New("invalid number of arguments")
			}
			stringMapCount++
			argCount += 2
		} else {
			argCount++
		}
	}
	argCount = 0

	switch c.AllowMissing {
	case true:
		if len(args) > len(c.Indexes)+stringMapCount {
			return nil, errors.New("too many arguments")
		}

	default:
		if len(args) != len(c.Indexes)+stringMapCount {
			return nil, errors.New("number of arguments does not equal number of indexers")
		}
	}

	var out []byte
	var val []byte
	var err error
	for i, idx := range c.Indexes {
		if argCount >= len(args) {
			// We're done; should only hit this if AllowMissing
			break
		}
		if _, ok := idx.(*StringMapFieldIndex); ok {
			if args[argCount+1] == nil {
				val, err = idx.FromArgs(args[argCount])
			} else {
				val, err = idx.FromArgs(args[argCount : argCount+2]...)
			}
			argCount += 2
		} else {
			val, err = idx.FromArgs(args[argCount])
			argCount++
		}
		if err != nil {
			return nil, fmt.Errorf("sub-index %d error: %v", i, err)
		}
		out = append(out, val...)
	}
	return out, nil
}
