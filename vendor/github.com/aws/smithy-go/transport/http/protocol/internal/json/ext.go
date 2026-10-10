package json

import (
	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/traits"
)

type jsonExt struct {
	jsonKey     []byte // `,"memberName":` -- use [1:] when no comma needed
	jsonNameKey []byte // `,"jsonName":` -- use [1:] when no comma needed (nil if no @jsonName)
}

func getExt(s *smithy.Schema) *jsonExt {
	return smithy.SchemaExtension(s, smithy.ExtJSON, buildJSONExt)
}

func buildJSONExt(s *smithy.Schema) *jsonExt {
	ext := &jsonExt{}

	if name := s.MemberName(); name != "" {
		ext.jsonKey = encodeJSONKey(name)
		if jn, ok := smithy.SchemaTrait[*traits.JSONName](s); ok {
			ext.jsonNameKey = encodeJSONKey(jn.Name)
		}
	}

	return ext
}

// memberByBytes looks a member up without allocating a string for its name: the
// conversion in a map index expression is optimized away.
func memberByBytes(s *smithy.Schema, name []byte) *smithy.Schema {
	return s.Members()[string(name)]
}

func encodeJSONKey(name string) []byte {
	buf := make([]byte, 0, len(name)+4)
	buf = append(buf, ',', '"')
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch c {
		case '"':
			buf = append(buf, '\\', '"')
		case '\\':
			buf = append(buf, '\\', '\\')
		case '\n':
			buf = append(buf, '\\', 'n')
		case '\r':
			buf = append(buf, '\\', 'r')
		case '\t':
			buf = append(buf, '\\', 't')
		default:
			if c < 0x20 {
				buf = append(buf, '\\', 'u', '0', '0', "0123456789abcdef"[c>>4], "0123456789abcdef"[c&0xF])
			} else {
				buf = append(buf, c)
			}
		}
	}
	buf = append(buf, '"', ':')
	return buf
}
