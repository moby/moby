package traits

import smithy "github.com/aws/smithy-go"

// JSONName represents smithy.api#jsonName.
type JSONName struct {
	Name string
}

// TraitID identifies the trait.
func (*JSONName) TraitID() smithy.ShapeID { return smithy.ShapeID{Namespace: "smithy.api", Name: "jsonName"} }

// MediaType represents smithy.api#mediaType.
type MediaType struct {
	Type string
}

// TraitID identifies the trait.
func (*MediaType) TraitID() smithy.ShapeID { return smithy.ShapeID{Namespace: "smithy.api", Name: "mediaType"} }

// TimestampFormat represents smithy.api#timestampFormat.
type TimestampFormat struct {
	Format string
}

// TraitID identifies the trait.
func (*TimestampFormat) TraitID() smithy.ShapeID { return smithy.ShapeID{Namespace: "smithy.api", Name: "timestampFormat"} }

// XMLAttribute represents smithy.api#xmlAttribute.
type XMLAttribute struct{}

// TraitID identifies the trait.
func (*XMLAttribute) TraitID() smithy.ShapeID { return smithy.ShapeID{Namespace: "smithy.api", Name: "xmlAttribute"} }

// XMLFlattened represents smithy.api#xmlFlattened.
type XMLFlattened struct{}

// TraitID identifies the trait.
func (*XMLFlattened) TraitID() smithy.ShapeID { return smithy.ShapeID{Namespace: "smithy.api", Name: "xmlFlattened"} }

// XMLName represents smithy.api#xmlName.
type XMLName struct {
	Name string
}

// TraitID identifies the trait.
func (*XMLName) TraitID() smithy.ShapeID { return smithy.ShapeID{Namespace: "smithy.api", Name: "xmlName"} }

// XMLNamespace represents smithy.api#xmlNamespace.
type XMLNamespace struct {
	URI    string
	Prefix string
}

// TraitID identifies the trait.
func (*XMLNamespace) TraitID() smithy.ShapeID { return smithy.ShapeID{Namespace: "smithy.api", Name: "xmlNamespace"} }
