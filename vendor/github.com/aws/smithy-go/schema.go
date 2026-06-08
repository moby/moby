package smithy

import (
	"fmt"
	"strings"
	"sync/atomic"
	"unsafe"
)

// ShapeType is a type of Smithy shape.
// See https://smithy.io/2.0/spec/idl.html#defining-shapes.
type ShapeType int

// Enumerates ShapeType per the Smithy IDL.
const (
	ShapeTypeBlob ShapeType = iota
	ShapeTypeBoolean
	ShapeTypeString
	ShapeTypeTimestamp
	ShapeTypeByte
	ShapeTypeShort
	ShapeTypeInteger
	ShapeTypeLong
	ShapeTypeFloat
	ShapeTypeDocument
	ShapeTypeDouble
	ShapeTypeBigDecimal
	ShapeTypeBigInteger
	ShapeTypeEnum
	ShapeTypeIntEnum
	ShapeTypeList
	ShapeTypeSet
	ShapeTypeMap
	ShapeTypeStructure
	ShapeTypeUnion
	ShapeTypeMember
	ShapeTypeService
	ShapeTypeResource
	ShapeTypeOperation
)

// ShapeID fields of a Smithy shape ID.
type ShapeID struct {
	Namespace, Name, Member string
}

// String returns the IDL microformat for the shape ID.
func (s ShapeID) String() string {
	if s.Member == "" {
		return fmt.Sprintf("%s#%s", s.Namespace, s.Name)
	}
	return fmt.Sprintf("%s#%s$%s", s.Namespace, s.Name, s.Member)
}

func stoid(s string) ShapeID {
	ns, n, _ := strings.Cut(s, "#")
	n, m, _ := strings.Cut(n, "$")
	return ShapeID{ns, n, m}
}

// Schema encodes information about a shape from a Smithy model.
//
// Generated clients use schemas at runtime to dynamically (de)serialize
// request/responses.
type Schema struct {
	id         ShapeID
	typ        ShapeType
	members    map[string]*Schema // member name -> schema
	traits     map[ShapeID]Trait  // trait ID -> non-indexed traits only
	indexed    []Trait            // indexed trait slots, sized to max index present
	directMask uint64             // bitmask: bit i set means indexed[i] was declared directly on this schema
	targetID   ShapeID            // for member schemas, the target's shape ID

	listMember       *Schema
	mapKey, mapValue *Schema

	ext [numExtensionSlots]unsafe.Pointer // lazily-computed codec extensions, accessed atomically
}

// NewSchema creates a new Schema with the given shape ID and traits.
func NewSchema(id ShapeID, typ ShapeType, numMembers int, ts ...Trait) *Schema {
	s := &Schema{
		id:      id,
		typ:     typ,
		members: make(map[string]*Schema, numMembers),
	}
	for _, t := range ts {
		s.addTrait(t, true)
	}
	return s
}

func (s *Schema) addTrait(t Trait, direct bool) {
	if it, ok := t.(IndexableTrait); ok {
		idx := it.TraitIndex()
		if idx >= len(s.indexed) {
			s.indexed = append(s.indexed, make([]Trait, idx-len(s.indexed)+1)...)
		}
		s.indexed[idx] = t
		if direct {
			s.directMask |= 1 << uint(idx)
		}
		return
	}

	if s.traits == nil {
		s.traits = map[ShapeID]Trait{}
	}
	s.traits[t.TraitID()] = t
}

// AddMember adds a member to the schema derived from the target, with
// optional trait overrides. The member schema is returned for caller
// reference.
//
// The member schema's effective trait view (accessed via [SchemaTrait])
// inherits all of the target's traits, then applies the overrides. The
// member's direct trait view (accessed via [SchemaDirectTrait]) contains
// only the overrides, i.e. the traits declared directly on the member.
func (s *Schema) AddMember(name string, target *Schema, ts ...Trait) *Schema {
	m := &Schema{
		id:         ShapeID{Member: name},
		typ:        target.typ,
		members:    target.members,
		indexed:    cloneIndexed(target.indexed),
		traits:     cloneTraits(target.traits),
		directMask: 0, // inherited traits are not direct
		targetID:   target.id,
		listMember: target.listMember,
		mapKey:     target.mapKey,
		mapValue:   target.mapValue,
	}

	// member-declared traits override and are direct
	for _, t := range ts {
		m.addTrait(t, true)
	}

	s.members[name] = m

	// Invalidate cached extensions, schema structure changed.
	for i := range s.ext {
		atomic.StorePointer(&s.ext[i], nil)
	}

	switch name {
	case "member":
		s.listMember = m
	case "key":
		s.mapKey = m
	case "value":
		s.mapValue = m
	}
	return m
}

func cloneIndexed(src []Trait) []Trait {
	if src == nil {
		return nil
	}
	dst := make([]Trait, len(src))
	copy(dst, src)
	return dst
}

func cloneTraits(src map[ShapeID]Trait) map[ShapeID]Trait {
	if src == nil {
		return nil
	}
	dst := make(map[ShapeID]Trait, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// ListMember returns the "member" schema for list types.
func (s *Schema) ListMember() *Schema {
	return s.listMember
}

// MapKey returns the "key" schema for map types.
func (s *Schema) MapKey() *Schema {
	return s.mapKey
}

// MapValue returns the "value" schema for map types.
func (s *Schema) MapValue() *Schema {
	return s.mapValue
}

// MemberName returns the member component of the schema's shape ID.
func (s *Schema) MemberName() string {
	return s.id.Member
}

// ID returns the shape ID of the schema.
func (s *Schema) ID() ShapeID {
	return s.id
}

// TargetID returns the shape ID of the member's target shape.
func (s *Schema) TargetID() ShapeID {
	return s.targetID
}

// Type returns the shape type of the schema.
func (s *Schema) Type() ShapeType {
	return s.typ
}

// Member returns the member schema for the given name, or nil.
func (s *Schema) Member(name string) *Schema {
	return s.members[name]
}

// Members returns the schema's members as a map of name to schema.
func (s *Schema) Members() map[string]*Schema {
	return s.members
}

// OperationSchema describes an operation, which is essentially its own schema
// with additional pointers to its input and output.
type OperationSchema struct {
	*Schema
	Input, Output *Schema

	inputStream, outputStream bool
}

// NewOperationSchema returns an OperationSchema for (input, output).
func NewOperationSchema(op, input, output *Schema) *OperationSchema {
	return &OperationSchema{
		Schema:       op,
		Input:        input,
		Output:       output,
		inputStream:  isEventStream(input),
		outputStream: isEventStream(output),
	}
}

// IsInputEventStream reports whether this is an input event stream.
func (s *OperationSchema) IsInputEventStream() bool {
	return s.inputStream
}

// IsOutputEventStream reports whether this is an output event stream.
func (s *OperationSchema) IsOutputEventStream() bool {
	return s.outputStream
}

// ServiceSchema describes a service shape.
type ServiceSchema struct {
	*Schema
	Version string
}

// NewServiceSchema returns a ServiceSchema for the given service shape.
func NewServiceSchema(schema *Schema, version string) *ServiceSchema {
	return &ServiceSchema{Schema: schema, Version: version}
}

// SchemaTrait returns the target trait on the schema if it exists.
//
// For member schemas this returns the effective trait, which is the trait
// declared directly on the member if present, else the trait inherited from
// the target shape.
func SchemaTrait[T Trait](s *Schema) (T, bool) {
	return schemaTrait[T](s, false)
}

// SchemaDirectTrait returns the target trait on the schema if it was
// declared directly on the schema.
//
// For member schemas this returns the trait only if it was declared on the
// member itself, ignoring any trait inherited from the target shape. For
// non-member schemas this is equivalent to [SchemaTrait].
func SchemaDirectTrait[T Trait](s *Schema) (T, bool) {
	return schemaTrait[T](s, true)
}

func schemaTrait[T Trait](s *Schema, directOnly bool) (T, bool) {
	var zero T

	if s == nil {
		return zero, false
	}

	if it, ok := Trait(zero).(IndexableTrait); ok {
		idx := it.TraitIndex()
		if idx >= len(s.indexed) {
			return zero, false
		}
		if directOnly && s.directMask&(1<<uint(idx)) == 0 {
			return zero, false
		}
		tt, ok := s.indexed[idx].(T)
		return tt, ok
	}

	opaque, ok := s.traits[zero.TraitID()]
	if !ok {
		return zero, false
	}

	tt, ok := opaque.(T)
	return tt, ok
}

// indexStreaming is the indexed trait slot for @streaming, mirrored from
// traits.indexStreaming. We can't import the traits package from here due to a
// circular dependency.
const indexStreaming = 17

func isEventStream(s *Schema) bool {
	if s == nil {
		return false
	}
	for _, m := range s.members {
		if m.typ != ShapeTypeUnion {
			continue
		}
		if len(m.indexed) > indexStreaming && m.indexed[indexStreaming] != nil {
			return true
		}
	}
	return false
}
