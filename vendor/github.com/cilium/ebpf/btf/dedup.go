package btf

import (
	"errors"
	"fmt"
	"hash/maphash"
	"slices"
)

// deduper deduplicates BTF types by finding all types in a Type graph that are
// Equivalent and replaces them with a single instance.
//
// See doc comments in types.go to understand the various ways in which Types
// can relate to each other and how they are compared for equality. We separate
// Identity (same memory location), Equivalence (same shape/layout), and
// Compatibility (CO-RE compatible) to be explicit about intent.
//
// This deduper opportunistically uses a combination of Identity and Equivalence
// to find types that can be deduplicated.
type deduper struct {
	visited   map[Type]struct{}
	hashCache map[hashCacheKey]uint64

	// Set of types that have been deduplicated.
	done map[Type]Type

	// Map of hash to types with that hash.
	hashed  map[uint64][]Type
	eqCache map[typKey]bool

	seed maphash.Seed
}

func newDeduper() *deduper {
	return &deduper{
		make(map[Type]struct{}),
		make(map[hashCacheKey]uint64),
		make(map[Type]Type),
		make(map[uint64][]Type),
		make(map[typKey]bool),
		maphash.MakeSeed(),
	}
}

func (d *deduper) deduplicate(t Type) (Type, error) {
	// If we have already attempted to deduplicate this exact type, return the
	// result.
	if done, ok := d.done[t]; ok {
		return done, nil
	}

	// Visit the subtree, if a type has children, attempt to replace it with a
	// deduplicated version of those children.
	for t := range postorder(t, d.visited) {
		for c := range children(t) {
			var err error
			*c, err = d.hashInsert(*c)
			if err != nil {
				return nil, err
			}
		}
	}

	// Finally, deduplicate the root type itself.
	return d.hashInsert(t)
}

// hashInsert attempts to deduplicate t by hashing it and comparing against
// other types with the same hash. Returns the Type to be used as the common
// substitute at this position in the graph.
func (d *deduper) hashInsert(t Type) (Type, error) {
	// If we have deduplicated this type before, return the result of that
	// deduplication.
	if done, ok := d.done[t]; ok {
		return done, nil
	}

	// Compute the hash of this type. Types with the same hash are candidates for
	// deduplication.
	hash, err := d.hash(t, -1)
	if err != nil {
		return nil, err
	}

	// A hash collision is possible, so we need to compare against all candidates
	// with the same hash.
	for _, candidate := range d.hashed[hash] {
		// Pre-size the visited slice, experimentation on VMLinux shows a capacity
		// of 16 to give the best performance.
		const visitedCapacity = 16
		err := d.typesEquivalent(candidate, t, make([]Type, 0, visitedCapacity))
		if errors.Is(err, errNotEquivalent) {
			continue
		}
		if err != nil {
			return nil, err
		}

		// Found a Type that's both Equivalent and hashes to the same value, choose
		// it as the deduplicated version.
		d.done[t] = candidate

		return candidate, nil
	}

	d.hashed[hash] = append(d.hashed[hash], t)

	return t, nil
}

// The hash of a Type is the same given its pointer and depth budget.
type hashCacheKey struct {
	t           Type
	depthBudget int
}

// hash computes a hash for t. The produced hash is the same for Types which
// are similar. The hash can collide such that two different Types may produce
// the same hash, so equivalence must be checked explicitly. It will recurse
// into children. The initial call should use a depthBudget of -1.
func (d *deduper) hash(t Type, depthBudget int) (uint64, error) {
	if depthBudget == 0 {
		return 0, nil
	}

	h := &maphash.Hash{}
	h.SetSeed(d.seed)

	switch t := t.(type) {
	case *Void:
		maphash.WriteComparable(h, kindUnknown)

	case *Int:
		maphash.WriteComparable(h, kindInt)
		maphash.WriteComparable(h, *t)

	case *Pointer:
		maphash.WriteComparable(h, kindPointer)
		// If the depth budget is positive, decrement it every time we follow a
		// pointer.
		if depthBudget > 0 {
			depthBudget--
		}

		// If this is the first time we are following a pointer, set the depth
		// budget. This limits amount of recursion we do when hashing pointers that
		// form cycles. This is cheaper than tracking visited types and works
		// because hash collisions are allowed.
		if depthBudget < 0 {
			depthBudget = 1

			// Double pointers are common in C. However, with a depth budget of 1, all
			// double pointers would hash the same, causing a performance issue when
			// checking equivalence. So we give double pointers a bit more budget.
			if _, ok := t.Target.(*Pointer); ok {
				depthBudget = 2
			}
		}
		sub, err := d.hash(t.Target, depthBudget)
		if err != nil {
			return 0, err
		}
		maphash.WriteComparable(h, sub)

	case *Array:
		maphash.WriteComparable(h, kindArray)
		maphash.WriteComparable(h, t.Nelems)
		sub, err := d.hash(t.Index, depthBudget)
		if err != nil {
			return 0, err
		}
		maphash.WriteComparable(h, sub)
		_, err = d.hash(t.Type, depthBudget)
		if err != nil {
			return 0, err
		}
		maphash.WriteComparable(h, sub)

	case *Struct, *Union:
		// Check the cache to avoid recomputing the hash for this type and depth
		// budget.
		key := hashCacheKey{t, depthBudget}
		if cached, ok := d.hashCache[key]; ok {
			return cached, nil
		}

		var members []Member
		switch t := t.(type) {
		case *Struct:
			maphash.WriteComparable(h, kindStruct)
			maphash.WriteComparable(h, t.Name)
			maphash.WriteComparable(h, t.Size)
			members = t.Members

		case *Union:
			maphash.WriteComparable(h, kindUnion)
			maphash.WriteComparable(h, t.Name)
			maphash.WriteComparable(h, t.Size)
			members = t.Members
		}

		maphash.WriteComparable(h, len(members))
		for _, m := range members {
			maphash.WriteComparable(h, m.Name)
			maphash.WriteComparable(h, m.Offset)
			sub, err := d.hash(m.Type, depthBudget)
			if err != nil {
				return 0, err
			}
			maphash.WriteComparable(h, sub)
		}

		sum := h.Sum64()
		d.hashCache[key] = sum
		return sum, nil

	case *Enum:
		maphash.WriteComparable(h, kindEnum)
		maphash.WriteComparable(h, t.Name)
		maphash.WriteComparable(h, t.Size)
		maphash.WriteComparable(h, t.Signed)
		for _, v := range t.Values {
			maphash.WriteComparable(h, v)
		}

	case *Fwd:
		maphash.WriteComparable(h, kindForward)
		maphash.WriteComparable(h, *t)

	case *Typedef:
		maphash.WriteComparable(h, kindTypedef)
		maphash.WriteComparable(h, t.Name)
		sub, err := d.hash(t.Type, depthBudget)
		if err != nil {
			return 0, err
		}
		maphash.WriteComparable(h, sub)

	case *Volatile:
		maphash.WriteComparable(h, kindVolatile)
		sub, err := d.hash(t.Type, depthBudget)
		if err != nil {
			return 0, err
		}
		maphash.WriteComparable(h, sub)

	case *Const:
		maphash.WriteComparable(h, kindConst)
		sub, err := d.hash(t.Type, depthBudget)
		if err != nil {
			return 0, err
		}
		maphash.WriteComparable(h, sub)

	case *Restrict:
		maphash.WriteComparable(h, kindRestrict)
		sub, err := d.hash(t.Type, depthBudget)
		if err != nil {
			return 0, err
		}
		maphash.WriteComparable(h, sub)

	case *Func:
		maphash.WriteComparable(h, kindFunc)
		maphash.WriteComparable(h, t.Name)
		sub, err := d.hash(t.Type, depthBudget)
		if err != nil {
			return 0, err
		}
		maphash.WriteComparable(h, sub)

	case *FuncProto:
		// It turns out that pointers to function prototypes are common in C code,
		// function pointers. Function prototypes frequently have similar patterns
		// of [ptr, ptr] -> int, or [ptr, ptr, ptr] -> int. Causing frequent hash
		// collisions, for the default depth budget of 1. So allow one additional
		// level of pointers when we encounter a function prototype.
		if depthBudget >= 0 {
			depthBudget++
		}

		maphash.WriteComparable(h, kindFuncProto)
		for _, p := range t.Params {
			maphash.WriteComparable(h, p.Name)
			sub, err := d.hash(p.Type, depthBudget)
			if err != nil {
				return 0, err
			}
			maphash.WriteComparable(h, sub)
		}
		sub, err := d.hash(t.Return, depthBudget)
		if err != nil {
			return 0, err
		}
		maphash.WriteComparable(h, sub)

	case *Var:
		maphash.WriteComparable(h, kindVar)
		maphash.WriteComparable(h, t.Name)
		maphash.WriteComparable(h, t.Linkage)
		sub, err := d.hash(t.Type, depthBudget)
		if err != nil {
			return 0, err
		}
		maphash.WriteComparable(h, sub)

	case *Datasec:
		maphash.WriteComparable(h, kindDatasec)
		maphash.WriteComparable(h, t.Name)
		for _, v := range t.Vars {
			maphash.WriteComparable(h, v.Offset)
			maphash.WriteComparable(h, v.Size)
			sub, err := d.hash(v.Type, depthBudget)
			if err != nil {
				return 0, err
			}
			maphash.WriteComparable(h, sub)
		}

	case *declTag:
		maphash.WriteComparable(h, kindDeclTag)
		maphash.WriteComparable(h, t.Value)
		maphash.WriteComparable(h, t.Index)
		sub, err := d.hash(t.Type, depthBudget)
		if err != nil {
			return 0, err
		}
		maphash.WriteComparable(h, sub)

	case *TypeTag:
		maphash.WriteComparable(h, kindTypeTag)
		maphash.WriteComparable(h, t.Value)
		sub, err := d.hash(t.Type, depthBudget)
		if err != nil {
			return 0, err
		}
		maphash.WriteComparable(h, sub)

	case *Float:
		maphash.WriteComparable(h, kindFloat)
		maphash.WriteComparable(h, *t)

	default:
		return 0, fmt.Errorf("unsupported type for hashing: %T", t)
	}

	return h.Sum64(), nil
}

type typKey struct {
	a Type
	b Type
}

var errNotEquivalent = errors.New("types are not equivalent")

// typesEquivalent checks if two types are Equivalent.
func (d *deduper) typesEquivalent(ta, tb Type, visited []Type) error {
	// Fast path: if Types are Identical, they are also Equivalent.
	if ta == tb {
		return nil
	}

	switch a := ta.(type) {
	case *Void:
		if _, ok := tb.(*Void); ok {
			return nil
		}
		return errNotEquivalent

	case *Int:
		b, ok := tb.(*Int)
		if !ok {
			return errNotEquivalent
		}
		if a.Name != b.Name || a.Size != b.Size || a.Encoding != b.Encoding {
			return errNotEquivalent
		}
		return nil

	case *Enum:
		b, ok := tb.(*Enum)
		if !ok {
			return errNotEquivalent
		}
		if a.Name != b.Name || len(a.Values) != len(b.Values) {
			return errNotEquivalent
		}
		for i := range a.Values {
			if a.Values[i].Name != b.Values[i].Name || a.Values[i].Value != b.Values[i].Value {
				return errNotEquivalent
			}
		}
		return nil

	case *Fwd:
		b, ok := tb.(*Fwd)
		if !ok {
			return errNotEquivalent
		}
		if a.Name != b.Name || a.Kind != b.Kind {
			return errNotEquivalent
		}
		return nil

	case *Float:
		b, ok := tb.(*Float)
		if !ok {
			return errNotEquivalent
		}
		if a.Name != b.Name || a.Size != b.Size {
			return errNotEquivalent
		}
		return nil

	case *Array:
		b, ok := tb.(*Array)
		if !ok {
			return errNotEquivalent
		}

		if a.Nelems != b.Nelems {
			return errNotEquivalent
		}
		if err := d.typesEquivalent(a.Index, b.Index, visited); err != nil {
			return err
		}
		if err := d.typesEquivalent(a.Type, b.Type, visited); err != nil {
			return err
		}
		return nil

	case *Pointer:
		b, ok := tb.(*Pointer)
		if !ok {
			return errNotEquivalent
		}

		// Detect cycles by tracking visited types. Assume types are Equivalent if
		// we have already visited this type in the current Equivalence check.
		if slices.Contains(visited, ta) {
			return nil
		}
		visited = append(visited, ta)

		return d.typesEquivalent(a.Target, b.Target, visited)

	case *Struct, *Union:
		// Use a cache to avoid recomputation. We only do this for composite types
		// since they are where types fan out the most. For other types, the
		// overhead of the lookup and update outweighs performance benefits.
		cacheKey := typKey{a: ta, b: tb}
		if equal, ok := d.eqCache[cacheKey]; ok {
			if equal {
				return nil
			}
			return errNotEquivalent
		}

		compErr := d.compositeEquivalent(ta, tb, visited)
		d.eqCache[cacheKey] = compErr == nil

		return compErr

	case *Typedef:
		b, ok := tb.(*Typedef)
		if !ok {
			return errNotEquivalent
		}
		if a.Name != b.Name {
			return errNotEquivalent
		}
		return d.typesEquivalent(a.Type, b.Type, visited)

	case *Volatile:
		b, ok := tb.(*Volatile)
		if !ok {
			return errNotEquivalent
		}
		return d.typesEquivalent(a.Type, b.Type, visited)

	case *Const:
		b, ok := tb.(*Const)
		if !ok {
			return errNotEquivalent
		}
		return d.typesEquivalent(a.Type, b.Type, visited)

	case *Restrict:
		b, ok := tb.(*Restrict)
		if !ok {
			return errNotEquivalent
		}
		return d.typesEquivalent(a.Type, b.Type, visited)

	case *Func:
		b, ok := tb.(*Func)
		if !ok {
			return errNotEquivalent
		}
		if a.Name != b.Name {
			return errNotEquivalent
		}
		return d.typesEquivalent(a.Type, b.Type, visited)

	case *FuncProto:
		b, ok := tb.(*FuncProto)
		if !ok {
			return errNotEquivalent
		}

		if err := d.typesEquivalent(a.Return, b.Return, visited); err != nil {
			return err
		}
		if len(a.Params) != len(b.Params) {
			return errNotEquivalent
		}
		for i := range a.Params {
			if a.Params[i].Name != b.Params[i].Name {
				return errNotEquivalent
			}
			if err := d.typesEquivalent(a.Params[i].Type, b.Params[i].Type, visited); err != nil {
				return err
			}
		}
		return nil

	case *Var:
		b, ok := tb.(*Var)
		if !ok {
			return errNotEquivalent
		}
		if a.Name != b.Name {
			return errNotEquivalent
		}
		if err := d.typesEquivalent(a.Type, b.Type, visited); err != nil {
			return err
		}
		if a.Linkage != b.Linkage {
			return errNotEquivalent
		}
		return nil

	case *Datasec:
		b, ok := tb.(*Datasec)
		if !ok {
			return errNotEquivalent
		}
		if a.Name != b.Name || len(a.Vars) != len(b.Vars) {
			return errNotEquivalent
		}
		for i := range a.Vars {
			if a.Vars[i].Offset != b.Vars[i].Offset ||
				a.Vars[i].Size != b.Vars[i].Size {
				return errNotEquivalent
			}

			if err := d.typesEquivalent(a.Vars[i].Type, b.Vars[i].Type, visited); err != nil {
				return err
			}
		}
		return nil

	case *declTag:
		b, ok := tb.(*declTag)
		if !ok {
			return errNotEquivalent
		}
		if a.Value != b.Value || a.Index != b.Index {
			return errNotEquivalent
		}
		return d.typesEquivalent(a.Type, b.Type, visited)

	case *TypeTag:
		b, ok := tb.(*TypeTag)
		if !ok {
			return errNotEquivalent
		}
		if a.Value != b.Value {
			return errNotEquivalent
		}
		if err := d.typesEquivalent(a.Type, b.Type, visited); err != nil {
			return err
		}
		return nil

	default:
		return fmt.Errorf("unsupported type for equivalence: %T", a)
	}
}

// compositeEquivalent checks if two composite types (Struct or Union) are
// Equivalent.
func (d *deduper) compositeEquivalent(at, bt Type, visited []Type) error {
	var ma, mb []Member
	switch a := at.(type) {
	case *Struct:
		b, ok := bt.(*Struct)
		if !ok {
			return errNotEquivalent
		}

		if a.Name != b.Name || a.Size != b.Size || len(a.Members) != len(b.Members) {
			return errNotEquivalent
		}
		ma = a.Members
		mb = b.Members

	case *Union:
		b, ok := bt.(*Union)
		if !ok {
			return errNotEquivalent
		}

		if a.Name != b.Name || a.Size != b.Size || len(a.Members) != len(b.Members) {
			return errNotEquivalent
		}
		ma = a.Members
		mb = b.Members
	}

	for i := range ma {
		if ma[i].Name != mb[i].Name || ma[i].Offset != mb[i].Offset {
			return errNotEquivalent
		}

		if err := d.typesEquivalent(ma[i].Type, mb[i].Type, visited); err != nil {
			return err
		}
	}

	return nil
}
