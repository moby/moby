// Copyright (c) 2022 PlanetScale Inc. All rights reserved.

package equal

import (
	"fmt"
	"sort"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/planetscale/vtprotobuf/generator"
)

func init() {
	generator.RegisterFeature("equal", func(gen *generator.GeneratedFile) generator.FeatureGenerator {
		return &equal{GeneratedFile: gen}
	})
}

var (
	protoPkg = protogen.GoImportPath("google.golang.org/protobuf/proto")
)

type equal struct {
	*generator.GeneratedFile
	once bool
}

var _ generator.FeatureGenerator = (*equal)(nil)

func (p *equal) Name() string { return "equal" }

func (p *equal) GenerateFile(file *protogen.File) bool {
	proto3 := file.Desc.Syntax() == protoreflect.Proto3
	for _, message := range file.Messages {
		p.message(proto3, message)
	}
	return p.once
}

const equalName = "EqualVT"
const equalMessageName = "EqualMessageVT"

func (p *equal) message(proto3 bool, message *protogen.Message) {
	for _, nested := range message.Messages {
		p.message(proto3, nested)
	}

	if message.Desc.IsMapEntry() {
		return
	}

	p.once = true

	ccTypeName := message.GoIdent.GoName
	p.P(`func (this *`, ccTypeName, `) `, equalName, `(that *`, ccTypeName, `) bool {`)

	p.P(`if this == that {`)
	p.P(`	return true`)
	p.P(`} else if this == nil || that == nil {`)
	p.P(`	return false`)
	p.P(`}`)

	sort.Slice(message.Fields, func(i, j int) bool {
		return message.Fields[i].Desc.Number() < message.Fields[j].Desc.Number()
	})

	{
		oneofs := make(map[string]struct{}, len(message.Fields))
		for _, field := range message.Fields {
			oneof := field.Oneof != nil && !field.Oneof.Desc.IsSynthetic()
			if !oneof {
				continue
			}

			fieldname := field.Oneof.GoName
			if _, ok := oneofs[fieldname]; ok {
				continue
			}
			oneofs[fieldname] = struct{}{}

			p.P(`if this.`, fieldname, ` == nil && that.`, fieldname, ` != nil {`)
			p.P(`	return false`)
			p.P(`} else if this.`, fieldname, ` != nil {`)
			p.P(`	if that.`, fieldname, ` == nil {`)
			p.P(`		return false`)
			p.P(`	}`)
			ccInterfaceName := fmt.Sprintf("is%s", field.Oneof.GoIdent.GoName)
			if p.IsWellKnownType(message) {
				p.P(`switch c := this.`, fieldname, `.(type) {`)
				for _, f := range field.Oneof.Fields {
					p.P(`case *`, f.GoIdent, `:`)
					p.P(`if !(*`, p.WellKnownFieldMap(f), `)(c).`, equalName, `(that.`, fieldname, `) {`)
					p.P(`return false`)
					p.P(`}`)
				}
				p.P(`}`)
			} else {
				p.P(`if !this.`, fieldname, `.(interface{ `, equalName, `(`, ccInterfaceName, `) bool }).`, equalName, `(that.`, fieldname, `) {`)
				p.P(`return false`)
				p.P(`}`)
			}
			p.P(`}`)
		}
	}

	for _, field := range message.Fields {
		oneof := field.Oneof != nil && !field.Oneof.Desc.IsSynthetic()
		nullable := field.Message != nil || (field.Oneof != nil && field.Oneof.Desc.IsSynthetic()) || (!proto3 && !oneof)
		if !oneof {
			p.field(field, nullable)
		}
	}

	if p.Wrapper() {
		p.P(`return true`)
	} else {
		p.P(`return string(this.unknownFields) == string(that.unknownFields)`)
	}
	p.P(`}`)
	p.P()

	if !p.Wrapper() {
		p.P(`func (this *`, ccTypeName, `) `, equalMessageName, `(thatMsg `, protoPkg.Ident("Message"), `) bool {`)
		p.P(`that, ok := thatMsg.(*`, ccTypeName, `)`)
		p.P(`if !ok {`)
		p.P(`return false`)
		p.P(`}`)
		p.P(`return this.`, equalName, `(that)`)
		p.P(`}`)
	}

	for _, field := range message.Fields {
		oneof := field.Oneof != nil && !field.Oneof.Desc.IsSynthetic()
		if !oneof {
			continue
		}
		p.oneof(field)
	}
}

func (p *equal) oneof(field *protogen.Field) {
	ccTypeName := field.GoIdent.GoName
	ccInterfaceName := fmt.Sprintf("is%s", field.Oneof.GoIdent.GoName)
	fieldname := field.GoName

	if p.IsWellKnownType(field.Parent) {
		p.P(`func (this *`, ccTypeName, `) `, equalName, `(thatIface any) bool {`)
	} else {
		p.P(`func (this *`, ccTypeName, `) `, equalName, `(thatIface `, ccInterfaceName, `) bool {`)
	}
	p.P(`that, ok := thatIface.(*`, ccTypeName, `)`)
	p.P(`if !ok {`)
	if p.IsWellKnownType(field.Parent) {
		p.P(`if ot, ok := thatIface.(*`, field.GoIdent, `); ok {`)
		p.P(`that = (*`, ccTypeName, `)(ot)`)
		p.P("} else {")
		p.P("return false")
		p.P("}")
	} else {
		p.P(`return false`)
	}
	p.P(`}`)
	p.P(`if this == that {`)
	p.P(`return true`)
	p.P(`}`)
	p.P(`if this == nil && that != nil || this != nil && that == nil {`)
	p.P(`return false`)
	p.P(`}`)

	lhs := fmt.Sprintf("this.%s", fieldname)
	rhs := fmt.Sprintf("that.%s", fieldname)
	kind := field.Desc.Kind()
	switch {
	case isScalar(kind):
		p.compareScalar(lhs, rhs, false)
	case kind == protoreflect.BytesKind:
		p.compareBytes(lhs, rhs, false)
	case kind == protoreflect.MessageKind || kind == protoreflect.GroupKind:
		p.compareCall(lhs, rhs, field.Message, false)
	default:
		panic("not implemented")
	}
	p.P(`return true`)
	p.P(`}`)
	p.P()
}

func (p *equal) field(field *protogen.Field, nullable bool) {
	fieldname := field.GoName

	repeated := field.Desc.Cardinality() == protoreflect.Repeated
	lhs := fmt.Sprintf("this.%s", fieldname)
	rhs := fmt.Sprintf("that.%s", fieldname)

	if repeated {
		p.P(`if len(`, lhs, `) != len(`, rhs, `) {`)
		p.P(`	return false`)
		p.P(`}`)
		p.P(`for i, vx := range `, lhs, ` {`)
		if field.Desc.IsMap() {
			p.P(`vy, ok := `, rhs, `[i]`)
			p.P(`if !ok {`)
			p.P(`return false`)
			p.P(`}`)

			field = field.Message.Fields[1]
		} else {
			p.P(`vy := `, rhs, `[i]`)
		}
		lhs, rhs = "vx", "vy"
		nullable = false
	}

	kind := field.Desc.Kind()
	switch {
	case isScalar(kind):
		p.compareScalar(lhs, rhs, nullable)

	case kind == protoreflect.BytesKind:
		p.compareBytes(lhs, rhs, nullable)

	case kind == protoreflect.MessageKind || kind == protoreflect.GroupKind:
		p.compareCall(lhs, rhs, field.Message, nullable)

	default:
		panic("not implemented")
	}

	if repeated {
		// close for loop
		p.P(`}`)
	}
}

func (p *equal) compareScalar(lhs, rhs string, nullable bool) {
	if nullable {
		p.P(`if p, q := `, lhs, `, `, rhs, `; (p == nil && q != nil) || (p != nil && (q == nil || *p != *q)) {`)
	} else {
		p.P(`if `, lhs, ` != `, rhs, ` {`)
	}
	p.P(`	return false`)
	p.P(`}`)
}

func (p *equal) compareBytes(lhs, rhs string, nullable bool) {
	if nullable {
		p.P(`if p, q := `, lhs, `, `, rhs, `; (p == nil && q != nil) || (p != nil && q == nil) || string(p) != string(q) {`)
	} else {
		// Inlined call to bytes.Equal()
		p.P(`if string(`, lhs, `) != string(`, rhs, `) {`)
	}
	p.P(`	return false`)
	p.P(`}`)
}

func (p *equal) compareCall(lhs, rhs string, msg *protogen.Message, nullable bool) {
	if !nullable {
		// The p != q check is mostly intended to catch the lhs = nil, rhs = nil case in which we would pointlessly
		// allocate not just one but two empty values. However, it also provides us with an extra scope to establish
		// our p and q variables.
		p.P(`if p, q := `, lhs, `, `, rhs, `; p != q {`)
		defer p.P(`}`)

		p.P(`if p == nil {`)
		p.P(`p = &`, p.QualifiedGoIdent(msg.GoIdent), `{}`)
		p.P(`}`)
		p.P(`if q == nil {`)
		p.P(`q = &`, p.QualifiedGoIdent(msg.GoIdent), `{}`)
		p.P(`}`)
		lhs, rhs = "p", "q"
	}
	switch {
	case p.IsWellKnownType(msg):
		wkt := p.WellKnownTypeMap(msg)
		p.P(`if !(*`, wkt, `)(`, lhs, `).`, equalName, `((*`, wkt, `)(`, rhs, `)) {`)
		p.P(`	return false`)
		p.P(`}`)
	case p.IsLocalMessage(msg):
		p.P(`if !`, lhs, `.`, equalName, `(`, rhs, `) {`)
		p.P(`	return false`)
		p.P(`}`)
	default:
		p.P(`if equal, ok := interface{}(`, lhs, `).(interface { `, equalName, `(*`, p.QualifiedGoIdent(msg.GoIdent), `) bool }); ok {`)
		p.P(`	if !equal.`, equalName, `(`, rhs, `) {`)
		p.P(`		return false`)
		p.P(`	}`)
		p.P(`} else if !`, p.Ident("google.golang.org/protobuf/proto", "Equal"), `(`, lhs, `, `, rhs, `) {`)
		p.P(`	return false`)
		p.P(`}`)
	}
}

func isScalar(kind protoreflect.Kind) bool {
	switch kind {
	case
		protoreflect.BoolKind,
		protoreflect.StringKind,
		protoreflect.DoubleKind, protoreflect.Fixed64Kind, protoreflect.Sfixed64Kind,
		protoreflect.FloatKind, protoreflect.Fixed32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Uint64Kind, protoreflect.Sint64Kind,
		protoreflect.Int32Kind, protoreflect.Uint32Kind, protoreflect.Sint32Kind,
		protoreflect.EnumKind:
		return true
	}
	return false
}
