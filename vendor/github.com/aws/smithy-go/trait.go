package smithy

// Trait represents a trait applied to a shape in a Smithy model. Traits
// related to (de)serialization are included in code-generated Schemas for the
// client.
type Trait interface {
	TraitID() ShapeID
}

// IndexableTrait is optionally implemented by Trait values that have a
// reserved index in Schema's indexed trait slice. All traits defined in the
// traits package implement this interface.
//
// You SHOULD NOT implement this outside of a smithy-go trait unless you know
// what you are doing. If you implement this and return a value that collides
// with one of the primary serde-based indexed traits (see index.go) you will
// probably break something.
type IndexableTrait interface {
	Trait
	TraitIndex() int
}
