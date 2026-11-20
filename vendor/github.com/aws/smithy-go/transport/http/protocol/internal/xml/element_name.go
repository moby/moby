package xml

import (
	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/traits"
)

// Unlike json which has a sane recursive data model, the xml serializer at its
// core operates by writing <ename>...</ename> except what ename is supposed to
// be depends on a handful of different factors based on serialization context
// and traits. The helpers here attempt to solve that in the most
// straightforward way possible.

// base resolver for element names
func (s *ShapeSerializer) ename(schema *smithy.Schema) string {
	// DIRECT only, if a member does not have an @xmlName but its target does,
	// we don't use the one on the target
	if t, ok := smithy.SchemaDirectTrait[*traits.XMLName](schema); ok {
		return t.Name
	} else if s.stack.Len() == 0 { // the root
		return schema.ID().Name
	}

	return schema.MemberName()
}

// wraps ename to check for the context of "am i in a list or map"
func (s *ShapeSerializer) ctxEname(schema *smithy.Schema) string {
	if top := s.stack.Top(); top != nil {
		switch top.kind {
		case ctxKindList:
			return top.itemName
		case ctxKindMap:
			if top.inMapEntry {
				return s.ename(top.schema.MapValue())
			}
		}
	}

	return s.ename(schema)
}

// structs with @httpPayload are special, preferring in order:
//   - member/target @xmlName
//   - target shape name
//
// otherwise just use context-based name
func (s *ShapeSerializer) structEname(schema *smithy.Schema) string {
	if isPayload(schema) {
		if t, ok := smithy.SchemaTrait[*traits.XMLName](schema); ok {
			return t.Name
		}
		return schema.TargetID().Name
	}

	return s.ctxEname(schema)
}

// resolution for @xmlNamespace, which is generally sane
func (s *ShapeSerializer) xmlns(schema *smithy.Schema) *traits.XMLNamespace {
	if top := s.stack.Top(); top != nil {
		switch {
		case top.flat && (top.kind == ctxKindList || top.kind == ctxKindMap):
			// flattened lists/maps have no wrapper, inherit the
			// collection's namespace if the member doesn't have its own
			if _, ok := smithy.SchemaDirectTrait[*traits.XMLNamespace](schema); !ok {
				ns, _ := smithy.SchemaDirectTrait[*traits.XMLNamespace](top.schema)
				return ns
			}
		case top.kind == ctxKindMap && !top.inMapEntry:
			return nil
		}
	}

	ns, _ := smithy.SchemaDirectTrait[*traits.XMLNamespace](schema)
	return ns
}
