package main

import (
	"fmt"

	gengo "google.golang.org/protobuf/cmd/protoc-gen-go/internal_gengo"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// emitMessages runs the vendored protoc-gen-go generator on a descriptor built
// from the Go contract. This keeps generation independent of protoc and PATH
// tools while preserving protoc-gen-go output.
func emitMessages(pt point) ([]byte, error) {
	fd, err := fileDescriptor(pt)
	if err != nil {
		return nil, err
	}
	req := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{pt.protoPath},
		ProtoFile:      []*descriptorpb.FileDescriptorProto{fd},
	}
	gen, err := protogen.Options{}.New(req)
	if err != nil {
		return nil, fmt.Errorf("build protobuf generator: %w", err)
	}
	gen.SupportedFeatures = gengo.SupportedFeatures
	gen.SupportedEditionsMinimum = gengo.SupportedEditionsMinimum
	gen.SupportedEditionsMaximum = gengo.SupportedEditionsMaximum
	// No protoc runs in this pipeline, but retain the generated version markers so
	// the protobuf runtime compatibility check remains active.
	for _, f := range gen.Files {
		if f.Generate {
			gengo.GenerateFile(gen, f)
		}
	}
	resp := gen.Response()
	if resp.Error != nil {
		return nil, fmt.Errorf("generate protobuf code: %s", resp.GetError())
	}
	if len(resp.File) != 1 {
		return nil, fmt.Errorf("expected 1 generated protobuf file, got %d", len(resp.File))
	}
	return []byte(resp.File[0].GetContent()), nil
}

// fileDescriptor builds the descriptor corresponding to [emitProto].
func fileDescriptor(pt point) (*descriptorpb.FileDescriptorProto, error) {
	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String(pt.protoPath),
		Package: proto.String(pt.id),
		Syntax:  proto.String("proto3"),
		Options: &descriptorpb.FileOptions{GoPackage: proto.String(pt.protogenImport())},
	}

	for _, msg := range pt.messages {
		d, err := messageDescriptor(pt.id, msg)
		if err != nil {
			return nil, err
		}
		fd.MessageType = append(fd.MessageType, d)
	}
	// Bare-error methods still need response messages.
	for _, m := range pt.methods {
		if m.bareError {
			fd.MessageType = append(fd.MessageType, &descriptorpb.DescriptorProto{Name: proto.String(m.response)})
		}
	}

	svc := &descriptorpb.ServiceDescriptorProto{Name: proto.String(pt.service)}
	for _, m := range pt.methods {
		svc.Method = append(svc.Method, &descriptorpb.MethodDescriptorProto{
			Name:       proto.String(m.name),
			InputType:  proto.String("." + pt.id + "." + m.request),
			OutputType: proto.String("." + pt.id + "." + m.response),
		})
	}
	fd.Service = []*descriptorpb.ServiceDescriptorProto{svc}
	return fd, nil
}

func messageDescriptor(pkg string, msg message) (*descriptorpb.DescriptorProto, error) {
	d := &descriptorpb.DescriptorProto{Name: proto.String(msg.name)}
	for _, n := range msg.reserved {
		d.ReservedRange = append(d.ReservedRange, &descriptorpb.DescriptorProto_ReservedRange{
			Start: proto.Int32(int32(n)), End: proto.Int32(int32(n) + 1),
		})
	}
	for _, f := range msg.fields {
		fd := &descriptorpb.FieldDescriptorProto{
			Name:     proto.String(f.protoName),
			Number:   proto.Int32(int32(f.number)),
			JsonName: proto.String(jsonName(f.protoName)),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		}
		switch f.kind {
		case scalarSingle:
			t, err := scalarDescriptorType(f.protoType)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", msg.name, f.protoName, err)
			}
			fd.Type = t.Enum()
		case scalarRepeated:
			t, err := scalarDescriptorType(f.protoType)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", msg.name, f.protoName, err)
			}
			fd.Type = t.Enum()
			fd.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
		case messageSingle:
			fd.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
			fd.TypeName = proto.String("." + pkg + "." + f.protoType)
		case messageRepeated:
			fd.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
			fd.TypeName = proto.String("." + pkg + "." + f.protoType)
			fd.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
		case scalarMap:
			// A proto map is represented as a repeated nested entry message.
			key, err := scalarDescriptorType(f.mapKey)
			if err != nil {
				return nil, fmt.Errorf("%s.%s key: %w", msg.name, f.protoName, err)
			}
			val, err := scalarDescriptorType(f.protoType)
			if err != nil {
				return nil, fmt.Errorf("%s.%s value: %w", msg.name, f.protoName, err)
			}
			entry := goCamelCase(f.protoName) + "Entry"
			d.NestedType = append(d.NestedType, &descriptorpb.DescriptorProto{
				Name:    proto.String(entry),
				Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name: proto.String("key"), Number: proto.Int32(1), JsonName: proto.String("key"),
						Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: key.Enum(),
					},
					{
						Name: proto.String("value"), Number: proto.Int32(2), JsonName: proto.String("value"),
						Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: val.Enum(),
					},
				},
			})
			fd.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
			fd.TypeName = proto.String("." + pkg + "." + msg.name + "." + entry)
			fd.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
		}
		d.Field = append(d.Field, fd)
	}
	return d, nil
}

// scalarDescriptorType maps a proto type token to its descriptor enum.
func scalarDescriptorType(protoType string) (descriptorpb.FieldDescriptorProto_Type, error) {
	switch protoType {
	case "string":
		return descriptorpb.FieldDescriptorProto_TYPE_STRING, nil
	case "bytes":
		return descriptorpb.FieldDescriptorProto_TYPE_BYTES, nil
	case "bool":
		return descriptorpb.FieldDescriptorProto_TYPE_BOOL, nil
	case "int32":
		return descriptorpb.FieldDescriptorProto_TYPE_INT32, nil
	case "int64":
		return descriptorpb.FieldDescriptorProto_TYPE_INT64, nil
	case "uint32":
		return descriptorpb.FieldDescriptorProto_TYPE_UINT32, nil
	case "uint64":
		return descriptorpb.FieldDescriptorProto_TYPE_UINT64, nil
	case "float":
		return descriptorpb.FieldDescriptorProto_TYPE_FLOAT, nil
	case "double":
		return descriptorpb.FieldDescriptorProto_TYPE_DOUBLE, nil
	}
	return 0, fmt.Errorf("unsupported proto type %q", protoType)
}

// jsonName matches protoc's lowerCamelCase json_name conversion.
func jsonName(protoName string) string {
	var b []byte
	upper := false
	for i := 0; i < len(protoName); i++ {
		c := protoName[i]
		if c == '_' {
			upper = true
			continue
		}
		if upper && isASCIILower(c) {
			c -= 'a' - 'A'
		}
		upper = false
		b = append(b, c)
	}
	return string(b)
}
