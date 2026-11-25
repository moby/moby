// Copyright (c) 2021 PlanetScale Inc. All rights reserved.
// Copyright (c) 2013, The GoGo Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unmarshal

import (
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/planetscale/vtprotobuf/generator"
)

func init() {
	generator.RegisterFeature("unmarshal", func(gen *generator.GeneratedFile) generator.FeatureGenerator {
		return &unmarshal{GeneratedFile: gen}
	})

	generator.RegisterFeature("unmarshal_unsafe", func(gen *generator.GeneratedFile) generator.FeatureGenerator {
		return &unmarshal{GeneratedFile: gen, unsafe: true}
	})
}

type unmarshal struct {
	*generator.GeneratedFile
	unsafe bool
	once   bool
}

var _ generator.FeatureGenerator = (*unmarshal)(nil)

func (p *unmarshal) GenerateFile(file *protogen.File) bool {
	proto3 := file.Desc.Syntax() == protoreflect.Proto3
	for _, message := range file.Messages {
		p.message(proto3, message)
	}

	return p.once
}

func (p *unmarshal) methodUnmarshal() string {
	if p.unsafe {
		return "UnmarshalVTUnsafe"
	}
	return "UnmarshalVT"
}

func (p *unmarshal) decodeMessage(varName, buf string, message *protogen.Message) {
	switch {
	case p.IsWellKnownType(message):
		p.P(`if err := (*`, p.WellKnownTypeMap(message), `)(`, varName, `).`, p.methodUnmarshal(), `(`, buf, `); err != nil {`)
		p.P(`return err`)
		p.P(`}`)

	case p.IsLocalMessage(message):
		p.P(`if err := `, varName, `.`, p.methodUnmarshal(), `(`, buf, `); err != nil {`)
		p.P(`return err`)
		p.P(`}`)

	default:
		p.P(`if unmarshal, ok := interface{}(`, varName, `).(interface{`)
		p.P(p.methodUnmarshal(), `([]byte) error`)
		p.P(`}); ok{`)
		p.P(`if err := unmarshal.`, p.methodUnmarshal(), `(`, buf, `); err != nil {`)
		p.P(`return err`)
		p.P(`}`)
		p.P(`} else {`)
		p.P(`if err := `, p.Ident(generator.ProtoPkg, "Unmarshal"), `(`, buf, `, `, varName, `); err != nil {`)
		p.P(`return err`)
		p.P(`}`)
		p.P(`}`)
	}
}

func (p *unmarshal) decodeVarint(varName string, typName string) {
	p.P(`for shift := uint(0); ; shift += 7 {`)
	p.P(`if shift >= 64 {`)
	p.P(`return `, p.Helper("ErrIntOverflow"))
	p.P(`}`)
	p.P(`if iNdEx >= l {`)
	p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
	p.P(`}`)
	p.P(`b := dAtA[iNdEx]`)
	p.P(`iNdEx++`)
	p.P(varName, ` |= `, typName, `(b&0x7F) << shift`)
	p.P(`if b < 0x80 {`)
	p.P(`break`)
	p.P(`}`)
	p.P(`}`)
}

func (p *unmarshal) decodeFixed32(varName string, typeName string) {
	p.P(`if (iNdEx+4) > l {`)
	p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
	p.P(`}`)
	p.P(varName, ` = `, typeName, `(`, p.Ident("encoding/binary", "LittleEndian"), `.Uint32(dAtA[iNdEx:]))`)
	p.P(`iNdEx += 4`)
}

func (p *unmarshal) decodeFixed64(varName string, typeName string) {
	p.P(`if (iNdEx+8) > l {`)
	p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
	p.P(`}`)
	p.P(varName, ` = `, typeName, `(`, p.Ident("encoding/binary", "LittleEndian"), `.Uint64(dAtA[iNdEx:]))`)
	p.P(`iNdEx += 8`)
}

func (p *unmarshal) declareMapField(varName string, nullable bool, field *protogen.Field) {
	switch field.Desc.Kind() {
	case protoreflect.DoubleKind:
		p.P(`var `, varName, ` float64`)
	case protoreflect.FloatKind:
		p.P(`var `, varName, ` float32`)
	case protoreflect.Int64Kind:
		p.P(`var `, varName, ` int64`)
	case protoreflect.Uint64Kind:
		p.P(`var `, varName, ` uint64`)
	case protoreflect.Int32Kind:
		p.P(`var `, varName, ` int32`)
	case protoreflect.Fixed64Kind:
		p.P(`var `, varName, ` uint64`)
	case protoreflect.Fixed32Kind:
		p.P(`var `, varName, ` uint32`)
	case protoreflect.BoolKind:
		p.P(`var `, varName, ` bool`)
	case protoreflect.StringKind:
		p.P(`var `, varName, ` `, field.GoIdent)
	case protoreflect.MessageKind:
		msgname := field.GoIdent
		if nullable {
			p.P(`var `, varName, ` *`, msgname)
		} else {
			p.P(varName, ` := &`, msgname, `{}`)
		}
	case protoreflect.BytesKind:
		p.P(varName, ` := []byte{}`)
	case protoreflect.Uint32Kind:
		p.P(`var `, varName, ` uint32`)
	case protoreflect.EnumKind:
		p.P(`var `, varName, ` `, field.GoIdent)
	case protoreflect.Sfixed32Kind:
		p.P(`var `, varName, ` int32`)
	case protoreflect.Sfixed64Kind:
		p.P(`var `, varName, ` int64`)
	case protoreflect.Sint32Kind:
		p.P(`var `, varName, ` int32`)
	case protoreflect.Sint64Kind:
		p.P(`var `, varName, ` int64`)
	}
}

func (p *unmarshal) mapField(varName string, field *protogen.Field) {
	switch field.Desc.Kind() {
	case protoreflect.DoubleKind:
		p.P(`var `, varName, `temp uint64`)
		p.decodeFixed64(varName+"temp", "uint64")
		p.P(varName, ` = `, p.Ident("math", "Float64frombits"), `(`, varName, `temp)`)
	case protoreflect.FloatKind:
		p.P(`var `, varName, `temp uint32`)
		p.decodeFixed32(varName+"temp", "uint32")
		p.P(varName, ` = `, p.Ident("math", "Float32frombits"), `(`, varName, `temp)`)
	case protoreflect.Int64Kind:
		p.decodeVarint(varName, "int64")
	case protoreflect.Uint64Kind:
		p.decodeVarint(varName, "uint64")
	case protoreflect.Int32Kind:
		p.decodeVarint(varName, "int32")
	case protoreflect.Fixed64Kind:
		p.decodeFixed64(varName, "uint64")
	case protoreflect.Fixed32Kind:
		p.decodeFixed32(varName, "uint32")
	case protoreflect.BoolKind:
		p.P(`var `, varName, `temp int`)
		p.decodeVarint(varName+"temp", "int")
		p.P(varName, ` = bool(`, varName, `temp != 0)`)
	case protoreflect.StringKind:
		p.P(`var stringLen`, varName, ` uint64`)
		p.decodeVarint("stringLen"+varName, "uint64")
		p.P(`intStringLen`, varName, ` := int(stringLen`, varName, `)`)
		p.P(`if intStringLen`, varName, ` < 0 {`)
		p.P(`return `, p.Helper("ErrInvalidLength"))
		p.P(`}`)
		p.P(`postStringIndex`, varName, ` := iNdEx + intStringLen`, varName)
		p.P(`if postStringIndex`, varName, ` < 0 {`)
		p.P(`return `, p.Helper("ErrInvalidLength"))
		p.P(`}`)
		p.P(`if postStringIndex`, varName, ` > l {`)
		p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
		p.P(`}`)
		if p.unsafe {
			p.P(`if intStringLen`, varName, ` == 0 {`)
			p.P(varName, ` = ""`)
			p.P(`} else {`)
			p.P(varName, ` = `, p.Ident("unsafe", `String`), `(&dAtA[iNdEx], intStringLen`, varName, `)`)
			p.P(`}`)
		} else {
			p.P(varName, ` = `, "string", `(dAtA[iNdEx:postStringIndex`, varName, `])`)
		}
		p.P(`iNdEx = postStringIndex`, varName)
	case protoreflect.MessageKind:
		p.P(`var mapmsglen int`)
		p.decodeVarint("mapmsglen", "int")
		p.P(`if mapmsglen < 0 {`)
		p.P(`return `, p.Helper("ErrInvalidLength"))
		p.P(`}`)
		p.P(`postmsgIndex := iNdEx + mapmsglen`)
		p.P(`if postmsgIndex < 0 {`)
		p.P(`return `, p.Helper("ErrInvalidLength"))
		p.P(`}`)
		p.P(`if postmsgIndex > l {`)
		p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
		p.P(`}`)
		buf := `dAtA[iNdEx:postmsgIndex]`
		p.P(varName, ` = &`, p.noStarOrSliceType(field), `{}`)
		p.decodeMessage(varName, buf, field.Message)
		p.P(`iNdEx = postmsgIndex`)
	case protoreflect.BytesKind:
		p.P(`var mapbyteLen uint64`)
		p.decodeVarint("mapbyteLen", "uint64")
		p.P(`intMapbyteLen := int(mapbyteLen)`)
		p.P(`if intMapbyteLen < 0 {`)
		p.P(`return `, p.Helper("ErrInvalidLength"))
		p.P(`}`)
		p.P(`postbytesIndex := iNdEx + intMapbyteLen`)
		p.P(`if postbytesIndex < 0 {`)
		p.P(`return `, p.Helper("ErrInvalidLength"))
		p.P(`}`)
		p.P(`if postbytesIndex > l {`)
		p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
		p.P(`}`)
		if p.unsafe {
			p.P(varName, ` = dAtA[iNdEx:postbytesIndex]`)
		} else {
			p.P(varName, ` = make([]byte, mapbyteLen)`)
			p.P(`copy(`, varName, `, dAtA[iNdEx:postbytesIndex])`)
		}
		p.P(`iNdEx = postbytesIndex`)
	case protoreflect.Uint32Kind:
		p.decodeVarint(varName, "uint32")
	case protoreflect.EnumKind:
		goTypV, _ := p.FieldGoType(field)
		p.decodeVarint(varName, goTypV)
	case protoreflect.Sfixed32Kind:
		p.decodeFixed32(varName, "int32")
	case protoreflect.Sfixed64Kind:
		p.decodeFixed64(varName, "int64")
	case protoreflect.Sint32Kind:
		p.P(`var `, varName, `temp int32`)
		p.decodeVarint(varName+"temp", "int32")
		p.P(varName, `temp = int32((uint32(`, varName, `temp) >> 1) ^ uint32(((`, varName, `temp&1)<<31)>>31))`)
		p.P(varName, ` = int32(`, varName, `temp)`)
	case protoreflect.Sint64Kind:
		p.P(`var `, varName, `temp uint64`)
		p.decodeVarint(varName+"temp", "uint64")
		p.P(varName, `temp = (`, varName, `temp >> 1) ^ uint64((int64(`, varName, `temp&1)<<63)>>63)`)
		p.P(varName, ` = int64(`, varName, `temp)`)
	}
}

func (p *unmarshal) noStarOrSliceType(field *protogen.Field) string {
	typ, _ := p.FieldGoType(field)
	if typ[0] == '[' && typ[1] == ']' {
		typ = typ[2:]
	}
	if typ[0] == '*' {
		typ = typ[1:]
	}
	return typ
}

func (p *unmarshal) fieldItem(field *protogen.Field, fieldname string, message *protogen.Message, proto3 bool) {
	repeated := field.Desc.Cardinality() == protoreflect.Repeated
	typ := p.noStarOrSliceType(field)
	oneof := field.Oneof != nil && !field.Oneof.Desc.IsSynthetic()
	nullable := field.Oneof != nil && field.Oneof.Desc.IsSynthetic()

	switch field.Desc.Kind() {
	case protoreflect.DoubleKind:
		p.P(`var v uint64`)
		p.decodeFixed64("v", "uint64")
		if oneof {
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{`, field.GoName, ": ", typ, "(", p.Ident("math", `Float64frombits`), `(v))}`)
		} else if repeated {
			p.P(`v2 := `, typ, "(", p.Ident("math", "Float64frombits"), `(v))`)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v2)`)
		} else if proto3 && !nullable {
			p.P(`m.`, fieldname, ` = `, typ, "(", p.Ident("math", "Float64frombits"), `(v))`)
		} else {
			p.P(`v2 := `, typ, "(", p.Ident("math", "Float64frombits"), `(v))`)
			p.P(`m.`, fieldname, ` = &v2`)
		}
	case protoreflect.FloatKind:
		p.P(`var v uint32`)
		p.decodeFixed32("v", "uint32")
		if oneof {
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{`, field.GoName, ": ", typ, "(", p.Ident("math", "Float32frombits"), `(v))}`)
		} else if repeated {
			p.P(`v2 := `, typ, "(", p.Ident("math", "Float32frombits"), `(v))`)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v2)`)
		} else if proto3 && !nullable {
			p.P(`m.`, fieldname, ` = `, typ, "(", p.Ident("math", "Float32frombits"), `(v))`)
		} else {
			p.P(`v2 := `, typ, "(", p.Ident("math", "Float32frombits"), `(v))`)
			p.P(`m.`, fieldname, ` = &v2`)
		}
	case protoreflect.Int64Kind:
		if oneof {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, "{", field.GoName, `: v}`)
		} else if repeated {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else if proto3 && !nullable {
			p.P(`m.`, fieldname, ` = 0`)
			p.decodeVarint("m."+fieldname, typ)
		} else {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = &v`)
		}
	case protoreflect.Uint64Kind:
		if oneof {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, "{", field.GoName, `: v}`)
		} else if repeated {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else if proto3 && !nullable {
			p.P(`m.`, fieldname, ` = 0`)
			p.decodeVarint("m."+fieldname, typ)
		} else {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = &v`)
		}
	case protoreflect.Int32Kind:
		if oneof {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, "{", field.GoName, `: v}`)
		} else if repeated {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else if proto3 && !nullable {
			p.P(`m.`, fieldname, ` = 0`)
			p.decodeVarint("m."+fieldname, typ)
		} else {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = &v`)
		}
	case protoreflect.Fixed64Kind:
		if oneof {
			p.P(`var v `, typ)
			p.decodeFixed64("v", typ)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, "{", field.GoName, `: v}`)
		} else if repeated {
			p.P(`var v `, typ)
			p.decodeFixed64("v", typ)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else if proto3 && !nullable {
			p.P(`m.`, fieldname, ` = 0`)
			p.decodeFixed64("m."+fieldname, typ)
		} else {
			p.P(`var v `, typ)
			p.decodeFixed64("v", typ)
			p.P(`m.`, fieldname, ` = &v`)
		}
	case protoreflect.Fixed32Kind:
		if oneof {
			p.P(`var v `, typ)
			p.decodeFixed32("v", typ)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, "{", field.GoName, `: v}`)
		} else if repeated {
			p.P(`var v `, typ)
			p.decodeFixed32("v", typ)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else if proto3 && !nullable {
			p.P(`m.`, fieldname, ` = 0`)
			p.decodeFixed32("m."+fieldname, typ)
		} else {
			p.P(`var v `, typ)
			p.decodeFixed32("v", typ)
			p.P(`m.`, fieldname, ` = &v`)
		}
	case protoreflect.BoolKind:
		p.P(`var v int`)
		p.decodeVarint("v", "int")
		if oneof {
			p.P(`b := `, typ, `(v != 0)`)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, "{", field.GoName, `: b}`)
		} else if repeated {
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, `, typ, `(v != 0))`)
		} else if proto3 && !nullable {
			p.P(`m.`, fieldname, ` = `, typ, `(v != 0)`)
		} else {
			p.P(`b := `, typ, `(v != 0)`)
			p.P(`m.`, fieldname, ` = &b`)
		}
	case protoreflect.StringKind:
		p.P(`var stringLen uint64`)
		p.decodeVarint("stringLen", "uint64")
		p.P(`intStringLen := int(stringLen)`)
		p.P(`if intStringLen < 0 {`)
		p.P(`return `, p.Helper("ErrInvalidLength"))
		p.P(`}`)
		p.P(`postIndex := iNdEx + intStringLen`)
		p.P(`if postIndex < 0 {`)
		p.P(`return `, p.Helper("ErrInvalidLength"))
		p.P(`}`)
		p.P(`if postIndex > l {`)
		p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
		p.P(`}`)
		str := "string(dAtA[iNdEx:postIndex])"
		if p.unsafe {
			str = "stringValue"
			p.P(`var stringValue string`)
			p.P(`if intStringLen > 0 {`)
			p.P(`stringValue = `, p.Ident("unsafe", `String`), `(&dAtA[iNdEx], intStringLen)`)
			p.P(`}`)
		}
		if oneof {
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{`, field.GoName, ": ", str, `}`)
		} else if repeated {
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, `, str, `)`)
		} else if proto3 && !nullable {
			p.P(`m.`, fieldname, ` = `, str)
		} else {
			p.P(`s := `, str)
			p.P(`m.`, fieldname, ` = &s`)
		}
		p.P(`iNdEx = postIndex`)
	case protoreflect.GroupKind:
		p.P(`groupStart := iNdEx`)
		p.P(`for {`)
		p.P(`maybeGroupEnd := iNdEx`)
		p.P(`var groupFieldWire uint64`)
		p.decodeVarint("groupFieldWire", "uint64")
		p.P(`groupWireType := int(wire & 0x7)`)
		p.P(`if groupWireType == `, strconv.Itoa(int(protowire.EndGroupType)), `{`)
		p.decodeMessage("m."+fieldname, "dAtA[groupStart:maybeGroupEnd]", field.Message)
		p.P(`break`)
		p.P(`}`)
		p.P(`skippy, err := `, p.Helper("Skip"), `(dAtA[iNdEx:])`)
		p.P(`if err != nil {`)
		p.P(`return err`)
		p.P(`}`)
		p.P(`if (skippy < 0) || (iNdEx + skippy) < 0 {`)
		p.P(`return `, p.Helper("ErrInvalidLength"))
		p.P(`}`)
		p.P(`iNdEx += skippy`)
		p.P(`}`)
	case protoreflect.MessageKind:
		p.P(`var msglen int`)
		p.decodeVarint("msglen", "int")
		p.P(`if msglen < 0 {`)
		p.P(`return `, p.Helper("ErrInvalidLength"))
		p.P(`}`)
		p.P(`postIndex := iNdEx + msglen`)
		p.P(`if postIndex < 0 {`)
		p.P(`return `, p.Helper("ErrInvalidLength"))
		p.P(`}`)
		p.P(`if postIndex > l {`)
		p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
		p.P(`}`)
		if oneof {
			buf := `dAtA[iNdEx:postIndex]`
			msgname := p.noStarOrSliceType(field)
			p.P(`if oneof, ok := m.`, fieldname, `.(*`, field.GoIdent, `); ok {`)
			p.decodeMessage("oneof."+field.GoName, buf, field.Message)
			p.P(`} else {`)
			if p.ShouldPool(message) && p.ShouldPool(field.Message) {
				p.P(`v := `, msgname, `FromVTPool()`)
			} else {
				p.P(`v := &`, msgname, `{}`)
			}
			p.decodeMessage("v", buf, field.Message)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, "{", field.GoName, `: v}`)
			p.P(`}`)
		} else if field.Desc.IsMap() {
			goTyp, _ := p.FieldGoType(field)
			goTypK, _ := p.FieldGoType(field.Message.Fields[0])
			goTypV, _ := p.FieldGoType(field.Message.Fields[1])

			p.P(`if m.`, fieldname, ` == nil {`)
			p.P(`m.`, fieldname, ` = make(`, goTyp, `)`)
			p.P(`}`)

			p.P("var mapkey ", goTypK)
			p.P("var mapvalue ", goTypV)
			p.P(`for iNdEx < postIndex {`)

			p.P(`entryPreIndex := iNdEx`)
			p.P(`var wire uint64`)
			p.decodeVarint("wire", "uint64")
			p.P(`fieldNum := int32(wire >> 3)`)

			p.P(`if fieldNum == 1 {`)
			p.mapField("mapkey", field.Message.Fields[0])
			p.P(`} else if fieldNum == 2 {`)
			p.mapField("mapvalue", field.Message.Fields[1])
			p.P(`} else {`)
			p.P(`iNdEx = entryPreIndex`)
			p.P(`skippy, err := `, p.Helper("Skip"), `(dAtA[iNdEx:])`)
			p.P(`if err != nil {`)
			p.P(`return err`)
			p.P(`}`)
			p.P(`if (skippy < 0) || (iNdEx + skippy) < 0 {`)
			p.P(`return `, p.Helper("ErrInvalidLength"))
			p.P(`}`)
			p.P(`if (iNdEx + skippy) > postIndex {`)
			p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
			p.P(`}`)
			p.P(`iNdEx += skippy`)
			p.P(`}`)
			p.P(`}`)
			p.P(`m.`, fieldname, `[mapkey] = mapvalue`)
		} else if repeated {
			if p.ShouldPool(message) {
				p.P(`if len(m.`, fieldname, `) == cap(m.`, fieldname, `) {`)
				p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, &`, field.Message.GoIdent, `{})`)
				p.P(`} else {`)
				p.P(`m.`, fieldname, ` = m.`, fieldname, `[:len(m.`, fieldname, `) + 1]`)
				p.P(`if m.`, fieldname, `[len(m.`, fieldname, `) - 1] == nil {`)
				p.P(`m.`, fieldname, `[len(m.`, fieldname, `) - 1] = &`, field.Message.GoIdent, `{}`)
				p.P(`}`)
				p.P(`}`)
			} else {
				p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, &`, field.Message.GoIdent, `{})`)
			}
			varname := fmt.Sprintf("m.%s[len(m.%s) - 1]", fieldname, fieldname)
			buf := `dAtA[iNdEx:postIndex]`
			p.decodeMessage(varname, buf, field.Message)
		} else {
			p.P(`if m.`, fieldname, ` == nil {`)
			if p.ShouldPool(message) && p.ShouldPool(field.Message) {
				p.P(`m.`, fieldname, ` = `, field.Message.GoIdent, `FromVTPool()`)
			} else {
				p.P(`m.`, fieldname, ` = &`, field.Message.GoIdent, `{}`)
			}
			p.P(`}`)
			p.decodeMessage("m."+fieldname, "dAtA[iNdEx:postIndex]", field.Message)
		}
		p.P(`iNdEx = postIndex`)

	case protoreflect.BytesKind:
		p.P(`var byteLen int`)
		p.decodeVarint("byteLen", "int")
		p.P(`if byteLen < 0 {`)
		p.P(`return `, p.Helper("ErrInvalidLength"))
		p.P(`}`)
		p.P(`postIndex := iNdEx + byteLen`)
		p.P(`if postIndex < 0 {`)
		p.P(`return `, p.Helper("ErrInvalidLength"))
		p.P(`}`)
		p.P(`if postIndex > l {`)
		p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
		p.P(`}`)
		if oneof {
			if p.unsafe {
				p.P(`v := dAtA[iNdEx:postIndex]`)
			} else {
				p.P(`v := make([]byte, postIndex-iNdEx)`)
				p.P(`copy(v, dAtA[iNdEx:postIndex])`)
			}
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, "{", field.GoName, `: v}`)
		} else if repeated {
			if p.unsafe {
				p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, dAtA[iNdEx:postIndex])`)
			} else {
				p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, make([]byte, postIndex-iNdEx))`)
				p.P(`copy(m.`, fieldname, `[len(m.`, fieldname, `)-1], dAtA[iNdEx:postIndex])`)
			}
		} else {
			if p.unsafe {
				p.P(`m.`, fieldname, ` = dAtA[iNdEx:postIndex]`)
			} else {
				p.P(`m.`, fieldname, ` = append(m.`, fieldname, `[:0] , dAtA[iNdEx:postIndex]...)`)
				p.P(`if m.`, fieldname, ` == nil {`)
				p.P(`m.`, fieldname, ` = []byte{}`)
				p.P(`}`)
			}
		}
		p.P(`iNdEx = postIndex`)
	case protoreflect.Uint32Kind:
		if oneof {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, "{", field.GoName, `: v}`)
		} else if repeated {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else if proto3 && !nullable {
			p.P(`m.`, fieldname, ` = 0`)
			p.decodeVarint("m."+fieldname, typ)
		} else {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = &v`)
		}
	case protoreflect.EnumKind:
		if oneof {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, "{", field.GoName, `: v}`)
		} else if repeated {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else if proto3 && !nullable {
			p.P(`m.`, fieldname, ` = 0`)
			p.decodeVarint("m."+fieldname, typ)
		} else {
			p.P(`var v `, typ)
			p.decodeVarint("v", typ)
			p.P(`m.`, fieldname, ` = &v`)
		}
	case protoreflect.Sfixed32Kind:
		if oneof {
			p.P(`var v `, typ)
			p.decodeFixed32("v", typ)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, "{", field.GoName, `: v}`)
		} else if repeated {
			p.P(`var v `, typ)
			p.decodeFixed32("v", typ)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else if proto3 && !nullable {
			p.P(`m.`, fieldname, ` = 0`)
			p.decodeFixed32("m."+fieldname, typ)
		} else {
			p.P(`var v `, typ)
			p.decodeFixed32("v", typ)
			p.P(`m.`, fieldname, ` = &v`)
		}
	case protoreflect.Sfixed64Kind:
		if oneof {
			p.P(`var v `, typ)
			p.decodeFixed64("v", typ)
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, "{", field.GoName, `: v}`)
		} else if repeated {
			p.P(`var v `, typ)
			p.decodeFixed64("v", typ)
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else if proto3 && !nullable {
			p.P(`m.`, fieldname, ` = 0`)
			p.decodeFixed64("m."+fieldname, typ)
		} else {
			p.P(`var v `, typ)
			p.decodeFixed64("v", typ)
			p.P(`m.`, fieldname, ` = &v`)
		}
	case protoreflect.Sint32Kind:
		p.P(`var v `, typ)
		p.decodeVarint("v", typ)
		p.P(`v = `, typ, `((uint32(v) >> 1) ^ uint32(((v&1)<<31)>>31))`)
		if oneof {
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, "{", field.GoName, `: v}`)
		} else if repeated {
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, v)`)
		} else if proto3 && !nullable {
			p.P(`m.`, fieldname, ` = v`)
		} else {
			p.P(`m.`, fieldname, ` = &v`)
		}
	case protoreflect.Sint64Kind:
		p.P(`var v uint64`)
		p.decodeVarint("v", "uint64")
		p.P(`v = (v >> 1) ^ uint64((int64(v&1)<<63)>>63)`)
		if oneof {
			p.P(`m.`, fieldname, ` = &`, field.GoIdent, `{`, field.GoName, ": ", typ, `(v)}`)
		} else if repeated {
			p.P(`m.`, fieldname, ` = append(m.`, fieldname, `, `, typ, `(v))`)
		} else if proto3 && !nullable {
			p.P(`m.`, fieldname, ` = `, typ, `(v)`)
		} else {
			p.P(`v2 := `, typ, `(v)`)
			p.P(`m.`, fieldname, ` = &v2`)
		}
	default:
		panic("not implemented")
	}
}

func (p *unmarshal) field(proto3, oneof bool, field *protogen.Field, message *protogen.Message, required protoreflect.FieldNumbers) {
	fieldname := field.GoName
	errFieldname := fieldname
	if field.Oneof != nil && !field.Oneof.Desc.IsSynthetic() {
		fieldname = field.Oneof.GoName
	}

	p.P(`case `, strconv.Itoa(int(field.Desc.Number())), `:`)
	wireType := generator.ProtoWireType(field.Desc.Kind())
	if field.Desc.IsList() && wireType != protowire.BytesType {
		p.P(`if wireType == `, strconv.Itoa(int(wireType)), `{`)
		p.fieldItem(field, fieldname, message, false)
		p.P(`} else if wireType == `, strconv.Itoa(int(protowire.BytesType)), `{`)
		p.P(`var packedLen int`)
		p.decodeVarint("packedLen", "int")
		p.P(`if packedLen < 0 {`)
		p.P(`return `, p.Helper("ErrInvalidLength"))
		p.P(`}`)
		p.P(`postIndex := iNdEx + packedLen`)
		p.P(`if postIndex < 0 {`)
		p.P(`return `, p.Helper("ErrInvalidLength"))
		p.P(`}`)
		p.P(`if postIndex > l {`)
		p.P(`return `, p.Ident("io", "ErrUnexpectedEOF"))
		p.P(`}`)

		p.P(`var elementCount int`)
		switch field.Desc.Kind() {
		case protoreflect.DoubleKind, protoreflect.Fixed64Kind, protoreflect.Sfixed64Kind:
			p.P(`elementCount = packedLen/`, 8)
		case protoreflect.FloatKind, protoreflect.Fixed32Kind, protoreflect.Sfixed32Kind:
			p.P(`elementCount = packedLen/`, 4)
		case protoreflect.Int64Kind, protoreflect.Uint64Kind, protoreflect.Int32Kind, protoreflect.Uint32Kind, protoreflect.Sint32Kind, protoreflect.Sint64Kind:
			p.P(`var count int`)
			p.P(`for _, integer := range dAtA[iNdEx:postIndex] {`)
			p.P(`if integer < 128 {`)
			p.P(`count++`)
			p.P(`}`)
			p.P(`}`)
			p.P(`elementCount = count`)
		case protoreflect.BoolKind:
			p.P(`elementCount = packedLen`)
		}

		if p.ShouldPool(message) {
			p.P(`if elementCount != 0 && len(m.`, fieldname, `) == 0 && cap(m.`, fieldname, `) < elementCount {`)
		} else {
			p.P(`if elementCount != 0 && len(m.`, fieldname, `) == 0 {`)
		}

		fieldtyp, _ := p.FieldGoType(field)
		p.P(`m.`, fieldname, ` = make(`, fieldtyp, `, 0, elementCount)`)
		p.P(`}`)

		p.P(`for iNdEx < postIndex {`)
		p.fieldItem(field, fieldname, message, false)
		p.P(`}`)
		p.P(`} else {`)
		p.P(`return `, p.Ident("fmt", "Errorf"), `("proto: wrong wireType = %d for field `, errFieldname, `", wireType)`)
		p.P(`}`)
	} else {
		p.P(`if wireType != `, strconv.Itoa(int(wireType)), `{`)
		p.P(`return `, p.Ident("fmt", "Errorf"), `("proto: wrong wireType = %d for field `, errFieldname, `", wireType)`)
		p.P(`}`)
		p.fieldItem(field, fieldname, message, proto3)
	}

	if field.Desc.Cardinality() == protoreflect.Required {
		var fieldBit int
		for fieldBit = 0; fieldBit < required.Len(); fieldBit++ {
			if required.Get(fieldBit) == field.Desc.Number() {
				break
			}
		}
		if fieldBit == required.Len() {
			panic("missing required field")
		}
		p.P(`hasFields[`, strconv.Itoa(fieldBit/64), `] |= uint64(`, fmt.Sprintf("0x%08x", uint64(1)<<(fieldBit%64)), `)`)
	}
}

func (p *unmarshal) message(proto3 bool, message *protogen.Message) {
	for _, nested := range message.Messages {
		p.message(proto3, nested)
	}

	if message.Desc.IsMapEntry() {
		return
	}

	p.once = true
	ccTypeName := message.GoIdent.GoName
	required := message.Desc.RequiredNumbers()

	p.P(`func (m *`, ccTypeName, `) `, p.methodUnmarshal(), `(dAtA []byte) error {`)
	if required.Len() > 0 {
		p.P(`var hasFields [`, strconv.Itoa(1+(required.Len()-1)/64), `]uint64`)
	}
	p.P(`l := len(dAtA)`)
	p.P(`iNdEx := 0`)
	p.P(`for iNdEx < l {`)
	p.P(`preIndex := iNdEx`)
	p.P(`var wire uint64`)
	p.decodeVarint("wire", "uint64")
	p.P(`fieldNum := int32(wire >> 3)`)
	p.P(`wireType := int(wire & 0x7)`)
	p.P(`if wireType == `, strconv.Itoa(int(protowire.EndGroupType)), ` {`)
	p.P(`return `, p.Ident("fmt", "Errorf"), `("proto: `, message.GoIdent.GoName, `: wiretype end group for non-group")`)
	p.P(`}`)
	p.P(`if fieldNum <= 0 {`)
	p.P(`return `, p.Ident("fmt", "Errorf"), `("proto: `, message.GoIdent.GoName, `: illegal tag %d (wire type %d)", fieldNum, wire)`)
	p.P(`}`)
	p.P(`switch fieldNum {`)
	for _, field := range message.Fields {
		p.field(proto3, false, field, message, required)
	}
	p.P(`default:`)
	p.P(`iNdEx=preIndex`)
	p.P(`skippy, err := `, p.Helper("Skip"), `(dAtA[iNdEx:])`)
	p.P(`if err != nil {`)
	p.P(`return err`)
	p.P(`}`)
	p.P(`if (skippy < 0) || (iNdEx + skippy) < 0 {`)
	p.P(`return `, p.Helper("ErrInvalidLength"))
	p.P(`}`)
	p.P(`if (iNdEx + skippy) > l {`)
	p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
	p.P(`}`)
	if message.Desc.ExtensionRanges().Len() > 0 {
		c := []string{}
		eranges := message.Desc.ExtensionRanges()
		for e := 0; e < eranges.Len(); e++ {
			erange := eranges.Get(e)
			c = append(c, `((fieldNum >= `+strconv.Itoa(int(erange[0]))+`) && (fieldNum < `+strconv.Itoa(int(erange[1]))+`))`)
		}
		p.P(`if `, strings.Join(c, "||"), `{`)
		p.P(`err = `, p.Ident(generator.ProtoPkg, "UnmarshalOptions"), `{AllowPartial: true}.Unmarshal(dAtA[iNdEx:iNdEx+skippy], m)`)
		p.P(`if err != nil {`)
		p.P(`return err`)
		p.P(`}`)
		p.P(`iNdEx += skippy`)
		p.P(`} else {`)
	}
	if !p.Wrapper() {
		p.P(`m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)`)
	}
	p.P(`iNdEx += skippy`)
	if message.Desc.ExtensionRanges().Len() > 0 {
		p.P(`}`)
	}
	p.P(`}`)
	p.P(`}`)

	for _, field := range message.Fields {
		if field.Desc.Cardinality() != protoreflect.Required {
			continue
		}
		var fieldBit int
		for fieldBit = 0; fieldBit < required.Len(); fieldBit++ {
			if required.Get(fieldBit) == field.Desc.Number() {
				break
			}
		}
		if fieldBit == required.Len() {
			panic("missing required field")
		}
		p.P(`if hasFields[`, strconv.Itoa(int(fieldBit/64)), `] & uint64(`, fmt.Sprintf("0x%08x", uint64(1)<<(fieldBit%64)), `) == 0 {`)
		p.P(`return `, p.Ident("fmt", "Errorf"), `("proto: required field `, field.Desc.Name(), ` not set")`)
		p.P(`}`)
	}
	p.P()
	p.P(`if iNdEx > l {`)
	p.P(`return `, p.Ident("io", `ErrUnexpectedEOF`))
	p.P(`}`)
	p.P(`return nil`)
	p.P(`}`)
}
