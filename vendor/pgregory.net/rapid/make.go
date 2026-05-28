// Copyright 2022 Gregory Petrosyan <gregory.petrosyan@gmail.com>
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package rapid

import (
	"fmt"
	"reflect"
)

// MakeConfig customizes reflection-based generators produced by MakeCustom.
type MakeConfig struct {
	// Types, if specified, provides Generators for concrete types that override
	// the automatic reflection-based generation.
	Types map[reflect.Type]*Generator[any]
	// Kinds, if specified, provides Generators for the specified kind that
	// override the automatic reflection-based generation.
	Kinds map[reflect.Kind]*Generator[any]
	// Fields, if specified, provides Generators for fields on a given type that
	// override the automatic reflection-based generation.
	Fields map[reflect.Type]map[string]*Generator[any]
}

// Make creates a generator of values of type V, using reflection to infer the required structure.
// Currently, Make may be unable to terminate generation of values of some recursive types, thus using
// Make with recursive types requires extra care.
func Make[V any]() *Generator[V] {
	return MakeCustom[V](MakeConfig{})
}

// MakeCustom creates a generator of values of type V, using reflection and
// overrides from MakeConfig to infer the required structure.
// Currently, Make may be unable to terminate generation of values of some recursive types, thus using
// Make with recursive types requires extra care.
func MakeCustom[V any](cfg MakeConfig) *Generator[V] {
	var zero V
	gen := cfg.newMakeGen(reflect.TypeOf(zero))
	return newGenerator[V](&makeGen[V]{
		gen: gen,
	})
}

type makeGen[V any] struct {
	gen *Generator[any]
}

func (g *makeGen[V]) String() string {
	var zero V
	return fmt.Sprintf("Make[%T]()", zero)
}

func (g *makeGen[V]) value(t *T) V {
	return g.gen.value(t).(V)
}

func (c *MakeConfig) newMakeGen(typ reflect.Type) *Generator[any] {
	gen, mayNeedCast := c.newMakeKindGen(typ)
	if !mayNeedCast || typ.String() == typ.Kind().String() {
		return gen // fast path with less reflect
	}
	return newGenerator[any](&castGen{gen, typ})
}

type castGen struct {
	gen *Generator[any]
	typ reflect.Type
}

func (g *castGen) String() string {
	return fmt.Sprintf("cast(%v, %v)", g.gen, g.typ.Name())
}

func (g *castGen) value(t *T) any {
	v := g.gen.value(t)
	if v == nil {
		return nil
	}
	return reflect.ValueOf(v).Convert(g.typ).Interface()
}

func (c *MakeConfig) newMakeKindGen(typ reflect.Type) (gen *Generator[any], mayNeedCast bool) {
	if c.Types != nil {
		if gen, ok := c.Types[typ]; ok {
			return gen, true
		}
	}

	if c.Kinds != nil {
		if gen, ok := c.Kinds[typ.Kind()]; ok {
			return gen, true
		}
	}

	switch typ.Kind() {
	case reflect.Bool:
		return Bool().AsAny(), true
	case reflect.Int:
		return Int().AsAny(), true
	case reflect.Int8:
		return Int8().AsAny(), true
	case reflect.Int16:
		return Int16().AsAny(), true
	case reflect.Int32:
		return Int32().AsAny(), true
	case reflect.Int64:
		return Int64().AsAny(), true
	case reflect.Uint:
		return Uint().AsAny(), true
	case reflect.Uint8:
		return Uint8().AsAny(), true
	case reflect.Uint16:
		return Uint16().AsAny(), true
	case reflect.Uint32:
		return Uint32().AsAny(), true
	case reflect.Uint64:
		return Uint64().AsAny(), true
	case reflect.Uintptr:
		return Uintptr().AsAny(), true
	case reflect.Float32:
		return Float32().AsAny(), true
	case reflect.Float64:
		return Float64().AsAny(), true
	case reflect.Array:
		return c.genAnyArray(typ), false
	case reflect.Map:
		return c.genAnyMap(typ), false
	case reflect.Pointer:
		return Deferred(func() *Generator[any] { return c.genAnyPointer(typ) }), false
	case reflect.Slice:
		return c.genAnySlice(typ), false
	case reflect.String:
		return String().AsAny(), true
	case reflect.Struct:
		return c.genAnyStruct(typ), false
	default:
		panic(fmt.Sprintf("unsupported type kind for Make: %v", typ.Kind()))
	}
}

func (c *MakeConfig) genAnyPointer(typ reflect.Type) *Generator[any] {
	elem := typ.Elem()
	elemGen := c.newMakeGen(elem)
	const pNonNil = 0.5

	return Custom(func(t *T) any {
		if flipBiasedCoin(t.s, pNonNil) {
			val := elemGen.value(t)
			ptr := reflect.New(elem)
			ptr.Elem().Set(reflect.ValueOf(val))
			return ptr.Interface()
		} else {
			return reflect.Zero(typ).Interface()
		}
	})
}

func (c *MakeConfig) genAnyArray(typ reflect.Type) *Generator[any] {
	count := typ.Len()
	elemGen := c.newMakeGen(typ.Elem())

	return Custom(func(t *T) any {
		a := reflect.Indirect(reflect.New(typ))
		if count == 0 {
			t.s.drawBits(0)
		} else {
			for i := 0; i < count; i++ {
				e := reflect.ValueOf(elemGen.value(t))
				a.Index(i).Set(e)
			}
		}
		return a.Interface()
	})
}

func (c *MakeConfig) genAnySlice(typ reflect.Type) *Generator[any] {
	elemGen := c.newMakeGen(typ.Elem())

	return Custom(func(t *T) any {
		repeat := newRepeat(-1, -1, -1, elemGen.String())
		sl := reflect.MakeSlice(typ, 0, repeat.avg())
		for repeat.more(t.s) {
			e := reflect.ValueOf(elemGen.value(t))
			sl = reflect.Append(sl, e)
		}
		return sl.Interface()
	})
}

func (c *MakeConfig) genAnyMap(typ reflect.Type) *Generator[any] {
	keyGen := c.newMakeGen(typ.Key())
	valGen := c.newMakeGen(typ.Elem())

	return Custom(func(t *T) any {
		label := keyGen.String() + "," + valGen.String()
		repeat := newRepeat(-1, -1, -1, label)
		m := reflect.MakeMapWithSize(typ, repeat.avg())
		for repeat.more(t.s) {
			k := reflect.ValueOf(keyGen.value(t))
			v := reflect.ValueOf(valGen.value(t))
			if m.MapIndex(k).IsValid() {
				repeat.reject()
			} else {
				m.SetMapIndex(k, v)
			}
		}
		return m.Interface()
	})
}

func (c *MakeConfig) genAnyStruct(typ reflect.Type) *Generator[any] {
	customFields := map[string]*Generator[any]{}
	if c.Fields != nil {
		if custom, ok := c.Fields[typ]; ok {
			customFields = custom
		}
	}

	numFields := typ.NumField()
	fieldGens := make([]*Generator[any], numFields)
	for i := 0; i < numFields; i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		if gen, ok := customFields[field.Name]; ok {
			fieldGens[i] = gen
		} else {
			fieldGens[i] = c.newMakeGen(field.Type)
		}
	}

	return Custom(func(t *T) any {
		s := reflect.Indirect(reflect.New(typ))

		fieldsSet := 0
		for i := 0; i < numFields; i++ {
			if fieldGens[i] == nil {
				continue
			}

			value := fieldGens[i].value(t)
			if value == nil {
				continue
			}

			s.Field(i).Set(reflect.ValueOf(value))
			fieldsSet++
		}

		if fieldsSet == 0 {
			t.s.drawBits(0)
		}

		return s.Interface()
	})
}
