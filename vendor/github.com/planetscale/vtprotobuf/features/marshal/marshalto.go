// Copyright (c) 2021 PlanetScale Inc. All rights reserved.
// Copyright (c) 2013, The GoGo Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package marshal

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/planetscale/vtprotobuf/generator"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func init() {
	generator.RegisterFeature("marshal", func(gen *generator.GeneratedFile) generator.FeatureGenerator {
		return &marshal{GeneratedFile: gen, Stable: false, strict: false}
	})
	generator.RegisterFeature("marshal_strict", func(gen *generator.GeneratedFile) generator.FeatureGenerator {
		return &marshal{GeneratedFile: gen, Stable: false, strict: true}
	})
}

type counter int

func (cnt *counter) Next() string {
	*cnt++
	return cnt.Current()
}

func (cnt *counter) Current() string {
	return strconv.Itoa(int(*cnt))
}

type marshal struct {
	*generator.GeneratedFile
	Stable, once, strict bool
}

var _ generator.FeatureGenerator = (*marshal)(nil)

func (p *marshal) GenerateFile(file *protogen.File) bool {
	for _, message := range file.Messages {
		p.message(message)
	}
	return p.once
}

func (p *marshal) encodeFixed64(varName ...string) {
	p.P(`i -= 8`)
	p.P(p.Ident("encoding/binary", "LittleEndian"), `.PutUint64(dAtA[i:], uint64(`, strings.Join(varName, ""), `))`)
}

func (p *marshal) encodeFixed32(varName ...string) {
	p.P(`i -= 4`)
	p.P(p.Ident("encoding/binary", "LittleEndian"), `.PutUint32(dAtA[i:], uint32(`, strings.Join(varName, ""), `))`)
}

func (p *marshal) encodeVarint(varName ...string) {
	p.P(`i = `, p.Helper("EncodeVarint"), `(dAtA, i, uint64(`, strings.Join(varName, ""), `))`)
}

func (p *marshal) encodeKey(fieldNumber protoreflect.FieldNumber, wireType protowire.Type) {
	x := uint32(fieldNumber)<<3 | uint32(wireType)
	i := 0
	keybuf := make([]byte, 0)
	for i = 0; x > 127; i++ {
		keybuf = append(keybuf, 0x80|uint8(x&0x7F))
		x >>= 7
	}
	keybuf = append(keybuf, uint8(x))
	for i = len(keybuf) - 1; i >= 0; i-- {
		p.P(`i--`)
		p.P(`dAtA[i] = `, fmt.Sprintf("%#v", keybuf[i]))
	}
}

func (p *marshal) mapField(kvField *protogen.Field, varName string) {
	switch kvField.Desc.Kind() {
	case protoreflect.DoubleKind:
		p.encodeFixed64(p.Ident("math", "Float64bits"), `(float64(`, varName, `))`)
	case protoreflect.FloatKind:
		p.encodeFixed32(p.Ident("math", "Float32bits"), `(float32(`, varName, `))`)
	case protoreflect.Int64Kind, protoreflect.Uint64Kind, protoreflect.Int32Kind, protoreflect.Uint32Kind, protoreflect.EnumKind:
		p.encodeVarint(varName)
	case protoreflect.Fixed64Kind, protoreflect.Sfixed64Kind:
		p.encodeFixed64(varName)
	case protoreflect.Fixed32Kind, protoreflect.Sfixed32Kind:
		p.encodeFixed32(varName)
	case protoreflect.BoolKind:
		p.P(`i--`)
		p.P(`if `, varName, ` {`)
		p.P(`dAtA[i] = 1`)
		p.P(`} else {`)
		p.P(`dAtA[i] = 0`)
		p.P(`}`)
	case protoreflect.StringKind, protoreflect.BytesKind:
		p.P(`i -= len(`, varName, `)`)
		p.P(`copy(dAtA[i:], `, varName, `)`)
		p.encodeVarint(`len(`, varName, `)`)
	case protoreflect.Sint32Kind:
		p.encodeVarint(`(uint32(`, varName, `) << 1) ^ uint32((`, varName, ` >> 31))`)
	case protoreflect.Sint64Kind:
		p.encodeVarint(`(uint64(`, varName, `) << 1) ^ uint64((`, varName, ` >> 63))`)
	case protoreflect.MessageKind:
		p.marshalBackward(varName, true, kvField.Message)
	}
}

func (p *marshal) field(oneof bool, numGen *counter, field *protogen.Field) {
	fieldname := field.GoName
	nullable := field.Message != nil || (!oneof && field.Desc.HasPresence())
	repeated := field.Desc.Cardinality() == protoreflect.Repeated
	if repeated {
		p.P(`if len(m.`, fieldname, `) > 0 {`)
	} else if nullable {
		if field.Desc.Cardinality() == protoreflect.Required {
			p.P(`if m.`, fieldname, ` == nil {`)
			p.P(`return 0, `, p.Ident("fmt", "Errorf"), `("proto: required field `, field.Desc.Name(), ` not set")`)
			p.P(`} else {`)
		} else {
			p.P(`if m.`, fieldname, ` != nil {`)
		}
	}
	packed := field.Desc.IsPacked()
	wireType := generator.ProtoWireType(field.Desc.Kind())
	fieldNumber := field.Desc.Number()
	if packed {
		wireType = protowire.BytesType
	}
	switch field.Desc.Kind() {
	case protoreflect.DoubleKind:
		if packed {
			val := p.reverseListRange(`m.`, fieldname)
			p.P(`f`, numGen.Next(), ` := `, p.Ident("math", "Float64bits"), `(float64(`, val, `))`)
			p.encodeFixed64("f", numGen.Current())
			p.P(`}`)
			p.encodeVarint(`len(m.`, fieldname, `) * 8`)
			p.encodeKey(fieldNumber, wireType)
		} else if repeated {
			val := p.reverseListRange(`m.`, fieldname)
			p.P(`f`, numGen.Next(), ` := `, p.Ident("math", "Float64bits"), `(float64(`, val, `))`)
			p.encodeFixed64("f", numGen.Current())
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else if nullable {
			p.encodeFixed64(p.Ident("math", "Float64bits"), `(float64(*m.`+fieldname, `))`)
			p.encodeKey(fieldNumber, wireType)
		} else if !oneof {
			p.P(`if m.`, fieldname, ` != 0 {`)
			p.encodeFixed64(p.Ident("math", "Float64bits"), `(float64(m.`, fieldname, `))`)
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else {
			p.encodeFixed64(p.Ident("math", "Float64bits"), `(float64(m.`+fieldname, `))`)
			p.encodeKey(fieldNumber, wireType)
		}
	case protoreflect.FloatKind:
		if packed {
			val := p.reverseListRange(`m.`, fieldname)
			p.P(`f`, numGen.Next(), ` := `, p.Ident("math", "Float32bits"), `(float32(`, val, `))`)
			p.encodeFixed32("f" + numGen.Current())
			p.P(`}`)
			p.encodeVarint(`len(m.`, fieldname, `) * 4`)
			p.encodeKey(fieldNumber, wireType)
		} else if repeated {
			val := p.reverseListRange(`m.`, fieldname)
			p.P(`f`, numGen.Next(), ` := `, p.Ident("math", "Float32bits"), `(float32(`, val, `))`)
			p.encodeFixed32("f" + numGen.Current())
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else if nullable {
			p.encodeFixed32(p.Ident("math", "Float32bits"), `(float32(*m.`+fieldname, `))`)
			p.encodeKey(fieldNumber, wireType)
		} else if !oneof {
			p.P(`if m.`, fieldname, ` != 0 {`)
			p.encodeFixed32(p.Ident("math", "Float32bits"), `(float32(m.`+fieldname, `))`)
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else {
			p.encodeFixed32(p.Ident("math", "Float32bits"), `(float32(m.`+fieldname, `))`)
			p.encodeKey(fieldNumber, wireType)
		}
	case protoreflect.Int64Kind, protoreflect.Uint64Kind, protoreflect.Int32Kind, protoreflect.Uint32Kind, protoreflect.EnumKind:
		if packed {
			jvar := "j" + numGen.Next()
			total := "pksize" + numGen.Next()

			p.P(`var `, total, ` int`)
			p.P(`for _, num := range m.`, fieldname, ` {`)
			p.P(total, ` += `, p.Helper("SizeOfVarint"), `(uint64(num))`)
			p.P(`}`)

			p.P(`i -= `, total)
			p.P(jvar, `:= i`)

			switch field.Desc.Kind() {
			case protoreflect.Int64Kind, protoreflect.Int32Kind, protoreflect.EnumKind:
				p.P(`for _, num1 := range m.`, fieldname, ` {`)
				p.P(`num := uint64(num1)`)
			default:
				p.P(`for _, num := range m.`, fieldname, ` {`)
			}
			p.P(`for num >= 1<<7 {`)
			p.P(`dAtA[`, jvar, `] = uint8(uint64(num)&0x7f|0x80)`)
			p.P(`num >>= 7`)
			p.P(jvar, `++`)
			p.P(`}`)
			p.P(`dAtA[`, jvar, `] = uint8(num)`)
			p.P(jvar, `++`)
			p.P(`}`)

			p.encodeVarint(total)
			p.encodeKey(fieldNumber, wireType)
		} else if repeated {
			val := p.reverseListRange(`m.`, fieldname)
			p.encodeVarint(val)
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else if nullable {
			p.encodeVarint(`*m.`, fieldname)
			p.encodeKey(fieldNumber, wireType)
		} else if !oneof {
			p.P(`if m.`, fieldname, ` != 0 {`)
			p.encodeVarint(`m.`, fieldname)
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else {
			p.encodeVarint(`m.`, fieldname)
			p.encodeKey(fieldNumber, wireType)
		}
	case protoreflect.Fixed64Kind, protoreflect.Sfixed64Kind:
		if packed {
			val := p.reverseListRange(`m.`, fieldname)
			p.encodeFixed64(val)
			p.P(`}`)
			p.encodeVarint(`len(m.`, fieldname, `) * 8`)
			p.encodeKey(fieldNumber, wireType)
		} else if repeated {
			val := p.reverseListRange(`m.`, fieldname)
			p.encodeFixed64(val)
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else if nullable {
			p.encodeFixed64("*m.", fieldname)
			p.encodeKey(fieldNumber, wireType)
		} else if !oneof {
			p.P(`if m.`, fieldname, ` != 0 {`)
			p.encodeFixed64("m.", fieldname)
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else {
			p.encodeFixed64("m.", fieldname)
			p.encodeKey(fieldNumber, wireType)
		}
	case protoreflect.Fixed32Kind, protoreflect.Sfixed32Kind:
		if packed {
			val := p.reverseListRange(`m.`, fieldname)
			p.encodeFixed32(val)
			p.P(`}`)
			p.encodeVarint(`len(m.`, fieldname, `) * 4`)
			p.encodeKey(fieldNumber, wireType)
		} else if repeated {
			val := p.reverseListRange(`m.`, fieldname)
			p.encodeFixed32(val)
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else if nullable {
			p.encodeFixed32("*m." + fieldname)
			p.encodeKey(fieldNumber, wireType)
		} else if !oneof {
			p.P(`if m.`, fieldname, ` != 0 {`)
			p.encodeFixed32("m." + fieldname)
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else {
			p.encodeFixed32("m." + fieldname)
			p.encodeKey(fieldNumber, wireType)
		}
	case protoreflect.BoolKind:
		if packed {
			val := p.reverseListRange(`m.`, fieldname)
			p.P(`i--`)
			p.P(`if `, val, ` {`)
			p.P(`dAtA[i] = 1`)
			p.P(`} else {`)
			p.P(`dAtA[i] = 0`)
			p.P(`}`)
			p.P(`}`)
			p.encodeVarint(`len(m.`, fieldname, `)`)
			p.encodeKey(fieldNumber, wireType)
		} else if repeated {
			val := p.reverseListRange(`m.`, fieldname)
			p.P(`i--`)
			p.P(`if `, val, ` {`)
			p.P(`dAtA[i] = 1`)
			p.P(`} else {`)
			p.P(`dAtA[i] = 0`)
			p.P(`}`)
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else if nullable {
			p.P(`i--`)
			p.P(`if *m.`, fieldname, ` {`)
			p.P(`dAtA[i] = 1`)
			p.P(`} else {`)
			p.P(`dAtA[i] = 0`)
			p.P(`}`)
			p.encodeKey(fieldNumber, wireType)
		} else if !oneof {
			p.P(`if m.`, fieldname, ` {`)
			p.P(`i--`)
			p.P(`if m.`, fieldname, ` {`)
			p.P(`dAtA[i] = 1`)
			p.P(`} else {`)
			p.P(`dAtA[i] = 0`)
			p.P(`}`)
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else {
			p.P(`i--`)
			p.P(`if m.`, fieldname, ` {`)
			p.P(`dAtA[i] = 1`)
			p.P(`} else {`)
			p.P(`dAtA[i] = 0`)
			p.P(`}`)
			p.encodeKey(fieldNumber, wireType)
		}
	case protoreflect.StringKind:
		if repeated {
			val := p.reverseListRange(`m.`, fieldname)
			p.P(`i -= len(`, val, `)`)
			p.P(`copy(dAtA[i:], `, val, `)`)
			p.encodeVarint(`len(`, val, `)`)
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else if nullable {
			p.P(`i -= len(*m.`, fieldname, `)`)
			p.P(`copy(dAtA[i:], *m.`, fieldname, `)`)
			p.encodeVarint(`len(*m.`, fieldname, `)`)
			p.encodeKey(fieldNumber, wireType)
		} else if !oneof {
			p.P(`if len(m.`, fieldname, `) > 0 {`)
			p.P(`i -= len(m.`, fieldname, `)`)
			p.P(`copy(dAtA[i:], m.`, fieldname, `)`)
			p.encodeVarint(`len(m.`, fieldname, `)`)
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else {
			p.P(`i -= len(m.`, fieldname, `)`)
			p.P(`copy(dAtA[i:], m.`, fieldname, `)`)
			p.encodeVarint(`len(m.`, fieldname, `)`)
			p.encodeKey(fieldNumber, wireType)
		}
	case protoreflect.GroupKind:
		p.encodeKey(fieldNumber, protowire.EndGroupType)
		p.marshalBackward(`m.`+fieldname, false, field.Message)
		p.encodeKey(fieldNumber, protowire.StartGroupType)
	case protoreflect.MessageKind:
		if field.Desc.IsMap() {
			goTypK, _ := p.FieldGoType(field.Message.Fields[0])
			keyKind := field.Message.Fields[0].Desc.Kind()
			valKind := field.Message.Fields[1].Desc.Kind()

			var val string
			if p.Stable && keyKind != protoreflect.BoolKind {
				keysName := `keysFor` + fieldname
				p.P(keysName, ` := make([]`, goTypK, `, 0, len(m.`, fieldname, `))`)
				p.P(`for k := range m.`, fieldname, ` {`)
				p.P(keysName, ` = append(`, keysName, `, `, goTypK, `(k))`)
				p.P(`}`)
				p.P(p.Ident("sort", "Slice"), `(`, keysName, `, func(i, j int) bool {`)
				p.P(`return `, keysName, `[i] < `, keysName, `[j]`)
				p.P(`})`)
				val = p.reverseListRange(keysName)
			} else {
				p.P(`for k := range m.`, fieldname, ` {`)
				val = "k"
			}
			if p.Stable {
				p.P(`v := m.`, fieldname, `[`, goTypK, `(`, val, `)]`)
			} else {
				p.P(`v := m.`, fieldname, `[`, val, `]`)
			}
			p.P(`baseI := i`)

			accessor := `v`
			p.mapField(field.Message.Fields[1], accessor)
			p.encodeKey(2, generator.ProtoWireType(valKind))

			p.mapField(field.Message.Fields[0], val)
			p.encodeKey(1, generator.ProtoWireType(keyKind))
			p.encodeVarint(`baseI - i`)
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else if repeated {
			val := p.reverseListRange(`m.`, fieldname)
			p.marshalBackward(val, true, field.Message)
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else {
			p.marshalBackward(`m.`+fieldname, true, field.Message)
			p.encodeKey(fieldNumber, wireType)
		}
	case protoreflect.BytesKind:
		if repeated {
			val := p.reverseListRange(`m.`, fieldname)
			p.P(`i -= len(`, val, `)`)
			p.P(`copy(dAtA[i:], `, val, `)`)
			p.encodeVarint(`len(`, val, `)`)
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else if !oneof && !field.Desc.HasPresence() {
			p.P(`if len(m.`, fieldname, `) > 0 {`)
			p.P(`i -= len(m.`, fieldname, `)`)
			p.P(`copy(dAtA[i:], m.`, fieldname, `)`)
			p.encodeVarint(`len(m.`, fieldname, `)`)
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else {
			p.P(`i -= len(m.`, fieldname, `)`)
			p.P(`copy(dAtA[i:], m.`, fieldname, `)`)
			p.encodeVarint(`len(m.`, fieldname, `)`)
			p.encodeKey(fieldNumber, wireType)
		}
	case protoreflect.Sint32Kind:
		if packed {
			jvar := "j" + numGen.Next()
			total := "pksize" + numGen.Next()

			p.P(`var `, total, ` int`)
			p.P(`for _, num := range m.`, fieldname, ` {`)
			p.P(total, ` += `, p.Helper("SizeOfZigzag"), `(uint64(num))`)
			p.P(`}`)
			p.P(`i -= `, total)
			p.P(jvar, `:= i`)

			p.P(`for _, num := range m.`, fieldname, ` {`)
			xvar := "x" + numGen.Next()
			p.P(xvar, ` := (uint32(num) << 1) ^ uint32((num >> 31))`)
			p.P(`for `, xvar, ` >= 1<<7 {`)
			p.P(`dAtA[`, jvar, `] = uint8(uint64(`, xvar, `)&0x7f|0x80)`)
			p.P(jvar, `++`)
			p.P(xvar, ` >>= 7`)
			p.P(`}`)
			p.P(`dAtA[`, jvar, `] = uint8(`, xvar, `)`)
			p.P(jvar, `++`)
			p.P(`}`)

			p.encodeVarint(total)
			p.encodeKey(fieldNumber, wireType)
		} else if repeated {
			val := p.reverseListRange(`m.`, fieldname)
			p.P(`x`, numGen.Next(), ` := (uint32(`, val, `) << 1) ^ uint32((`, val, ` >> 31))`)
			p.encodeVarint(`x`, numGen.Current())
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else if nullable {
			p.encodeVarint(`(uint32(*m.`, fieldname, `) << 1) ^ uint32((*m.`, fieldname, ` >> 31))`)
			p.encodeKey(fieldNumber, wireType)
		} else if !oneof {
			p.P(`if m.`, fieldname, ` != 0 {`)
			p.encodeVarint(`(uint32(m.`, fieldname, `) << 1) ^ uint32((m.`, fieldname, ` >> 31))`)
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else {
			p.encodeVarint(`(uint32(m.`, fieldname, `) << 1) ^ uint32((m.`, fieldname, ` >> 31))`)
			p.encodeKey(fieldNumber, wireType)
		}
	case protoreflect.Sint64Kind:
		if packed {
			jvar := "j" + numGen.Next()
			total := "pksize" + numGen.Next()

			p.P(`var `, total, ` int`)
			p.P(`for _, num := range m.`, fieldname, ` {`)
			p.P(total, ` += `, p.Helper("SizeOfZigzag"), `(uint64(num))`)
			p.P(`}`)
			p.P(`i -= `, total)
			p.P(jvar, `:= i`)

			p.P(`for _, num := range m.`, fieldname, ` {`)
			xvar := "x" + numGen.Next()
			p.P(xvar, ` := (uint64(num) << 1) ^ uint64((num >> 63))`)
			p.P(`for `, xvar, ` >= 1<<7 {`)
			p.P(`dAtA[`, jvar, `] = uint8(uint64(`, xvar, `)&0x7f|0x80)`)
			p.P(jvar, `++`)
			p.P(xvar, ` >>= 7`)
			p.P(`}`)
			p.P(`dAtA[`, jvar, `] = uint8(`, xvar, `)`)
			p.P(jvar, `++`)
			p.P(`}`)

			p.encodeVarint(total)
			p.encodeKey(fieldNumber, wireType)
		} else if repeated {
			val := p.reverseListRange(`m.`, fieldname)
			p.P(`x`, numGen.Next(), ` := (uint64(`, val, `) << 1) ^ uint64((`, val, ` >> 63))`)
			p.encodeVarint("x" + numGen.Current())
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else if nullable {
			p.encodeVarint(`(uint64(*m.`, fieldname, `) << 1) ^ uint64((*m.`, fieldname, ` >> 63))`)
			p.encodeKey(fieldNumber, wireType)
		} else if !oneof {
			p.P(`if m.`, fieldname, ` != 0 {`)
			p.encodeVarint(`(uint64(m.`, fieldname, `) << 1) ^ uint64((m.`, fieldname, ` >> 63))`)
			p.encodeKey(fieldNumber, wireType)
			p.P(`}`)
		} else {
			p.encodeVarint(`(uint64(m.`, fieldname, `) << 1) ^ uint64((m.`, fieldname, ` >> 63))`)
			p.encodeKey(fieldNumber, wireType)
		}
	default:
		panic("not implemented")
	}
	// Empty protobufs should emit a message or compatibility with Golang protobuf;
	// See https://github.com/planetscale/vtprotobuf/issues/61
	if oneof && field.Desc.Kind() == protoreflect.MessageKind && !field.Desc.IsMap() && !field.Desc.IsList() {
		p.P("} else {")
		p.P("i = protohelpers.EncodeVarint(dAtA, i, 0)")
		p.encodeKey(fieldNumber, wireType)
		p.P("}")
	} else if repeated || nullable {
		p.P(`}`)
	}
}

func (p *marshal) methodMarshalToSizedBuffer() string {
	switch {
	case p.strict:
		return "MarshalToSizedBufferVTStrict"
	default:
		return "MarshalToSizedBufferVT"
	}
}

func (p *marshal) methodMarshalTo() string {
	switch {
	case p.strict:
		return "MarshalToVTStrict"
	default:
		return "MarshalToVT"
	}
}

func (p *marshal) methodMarshal() string {
	switch {
	case p.strict:
		return "MarshalVTStrict"
	default:
		return "MarshalVT"
	}
}

func (p *marshal) message(message *protogen.Message) {
	for _, nested := range message.Messages {
		p.message(nested)
	}

	if message.Desc.IsMapEntry() {
		return
	}

	p.once = true

	var numGen counter
	ccTypeName := message.GoIdent.GoName

	p.P(`func (m *`, ccTypeName, `) `, p.methodMarshal(), `() (dAtA []byte, err error) {`)
	p.P(`if m == nil {`)
	p.P(`return nil, nil`)
	p.P(`}`)
	p.P(`size := m.SizeVT()`)
	p.P(`dAtA = make([]byte, size)`)
	p.P(`n, err := m.`, p.methodMarshalToSizedBuffer(), `(dAtA[:size])`)
	p.P(`if err != nil {`)
	p.P(`return nil, err`)
	p.P(`}`)
	p.P(`return dAtA[:n], nil`)
	p.P(`}`)
	p.P(``)
	p.P(`func (m *`, ccTypeName, `) `, p.methodMarshalTo(), `(dAtA []byte) (int, error) {`)
	p.P(`size := m.SizeVT()`)
	p.P(`return m.`, p.methodMarshalToSizedBuffer(), `(dAtA[:size])`)
	p.P(`}`)
	p.P(``)
	p.P(`func (m *`, ccTypeName, `) `, p.methodMarshalToSizedBuffer(), `(dAtA []byte) (int, error) {`)
	p.P(`if m == nil {`)
	p.P(`return 0, nil`)
	p.P(`}`)
	p.P(`i := len(dAtA)`)
	p.P(`_ = i`)
	p.P(`var l int`)
	p.P(`_ = l`)

	if !p.Wrapper() {
		p.P(`if m.unknownFields != nil {`)
		p.P(`i -= len(m.unknownFields)`)
		p.P(`copy(dAtA[i:], m.unknownFields)`)
		p.P(`}`)
	}

	sort.Slice(message.Fields, func(i, j int) bool {
		return message.Fields[i].Desc.Number() < message.Fields[j].Desc.Number()
	})

	marshalForwardOneOf := func(varname ...any) {
		l := []any{`size, err := `}
		l = append(l, varname...)
		l = append(l, `.`, p.methodMarshalToSizedBuffer(), `(dAtA[:i])`)
		p.P(l...)
		p.P(`if err != nil {`)
		p.P(`return 0, err`)
		p.P(`}`)
		p.P(`i -= size`)
	}

	if p.strict {
		for i := len(message.Fields) - 1; i >= 0; i-- {
			field := message.Fields[i]
			oneof := field.Oneof != nil && !field.Oneof.Desc.IsSynthetic()
			if !oneof {
				p.field(false, &numGen, field)
			} else {
				if p.IsWellKnownType(message) {
					p.P(`if m, ok := m.`, field.Oneof.GoName, `.(*`, field.GoIdent, `); ok {`)
					p.P(`msg := ((*`, p.WellKnownFieldMap(field), `)(m))`)
				} else {
					p.P(`if msg, ok := m.`, field.Oneof.GoName, `.(*`, field.GoIdent.GoName, `); ok {`)
				}
				marshalForwardOneOf("msg")
				p.P(`}`)
			}
		}
	} else {
		// To match the wire format of proto.Marshal, oneofs have to be marshaled
		// before fields. See https://github.com/planetscale/vtprotobuf/pull/22

		oneofs := make(map[string]struct{}, len(message.Fields))
		for i := len(message.Fields) - 1; i >= 0; i-- {
			field := message.Fields[i]
			oneof := field.Oneof != nil && !field.Oneof.Desc.IsSynthetic()
			if oneof {
				fieldname := field.Oneof.GoName
				if _, ok := oneofs[fieldname]; ok {
					continue
				}
				oneofs[fieldname] = struct{}{}
				if p.IsWellKnownType(message) {
					p.P(`switch c := m.`, fieldname, `.(type) {`)
					for _, f := range field.Oneof.Fields {
						p.P(`case *`, f.GoIdent, `:`)
						marshalForwardOneOf(`(*`, p.WellKnownFieldMap(f), `)(c)`)
					}
					p.P(`}`)
				} else {
					p.P(`if vtmsg, ok := m.`, fieldname, `.(interface{`)
					p.P(p.methodMarshalToSizedBuffer(), ` ([]byte) (int, error)`)
					p.P(`}); ok {`)
					marshalForwardOneOf("vtmsg")
					p.P(`}`)
				}
			}
		}

		for i := len(message.Fields) - 1; i >= 0; i-- {
			field := message.Fields[i]
			oneof := field.Oneof != nil && !field.Oneof.Desc.IsSynthetic()
			if !oneof {
				p.field(false, &numGen, field)
			}
		}
	}

	p.P(`return len(dAtA) - i, nil`)
	p.P(`}`)
	p.P()

	// Generate MarshalToVT methods for oneof fields
	for _, field := range message.Fields {
		if field.Oneof == nil || field.Oneof.Desc.IsSynthetic() {
			continue
		}
		ccTypeName := field.GoIdent.GoName
		p.P(`func (m *`, ccTypeName, `) `, p.methodMarshalTo(), `(dAtA []byte) (int, error) {`)
		p.P(`size := m.SizeVT()`)
		p.P(`return m.`, p.methodMarshalToSizedBuffer(), `(dAtA[:size])`)
		p.P(`}`)
		p.P(``)
		p.P(`func (m *`, ccTypeName, `) `, p.methodMarshalToSizedBuffer(), `(dAtA []byte) (int, error) {`)
		p.P(`i := len(dAtA)`)
		p.field(true, &numGen, field)
		p.P(`return len(dAtA) - i, nil`)
		p.P(`}`)
	}
}

func (p *marshal) reverseListRange(expression ...string) string {
	exp := strings.Join(expression, "")
	p.P(`for iNdEx := len(`, exp, `) - 1; iNdEx >= 0; iNdEx-- {`)
	return exp + `[iNdEx]`
}

func (p *marshal) marshalBackwardSize(varInt bool) {
	p.P(`if err != nil {`)
	p.P(`return 0, err`)
	p.P(`}`)
	p.P(`i -= size`)
	if varInt {
		p.encodeVarint(`size`)
	}
}

func (p *marshal) marshalBackward(varName string, varInt bool, message *protogen.Message) {
	switch {
	case p.IsWellKnownType(message):
		p.P(`size, err := (*`, p.WellKnownTypeMap(message), `)(`, varName, `).`, p.methodMarshalToSizedBuffer(), `(dAtA[:i])`)
		p.marshalBackwardSize(varInt)

	case p.IsLocalMessage(message):
		p.P(`size, err := `, varName, `.`, p.methodMarshalToSizedBuffer(), `(dAtA[:i])`)
		p.marshalBackwardSize(varInt)

	default:
		p.P(`if vtmsg, ok := interface{}(`, varName, `).(interface{`)
		p.P(p.methodMarshalToSizedBuffer(), `([]byte) (int, error)`)
		p.P(`}); ok{`)
		p.P(`size, err := vtmsg.`, p.methodMarshalToSizedBuffer(), `(dAtA[:i])`)
		p.marshalBackwardSize(varInt)
		p.P(`} else {`)
		p.P(`encoded, err := `, p.Ident(generator.ProtoPkg, "Marshal"), `(`, varName, `)`)
		p.P(`if err != nil {`)
		p.P(`return 0, err`)
		p.P(`}`)
		p.P(`i -= len(encoded)`)
		p.P(`copy(dAtA[i:], encoded)`)
		if varInt {
			p.encodeVarint(`len(encoded)`)
		}
		p.P(`}`)
	}
}
