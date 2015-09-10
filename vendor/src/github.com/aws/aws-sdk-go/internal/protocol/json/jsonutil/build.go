// Package jsonutil provides JSON serialisation of AWS requests and responses.
package jsonutil

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// BuildJSON builds a JSON string for a given object v.
func BuildJSON(v interface{}) ([]byte, error) {
	var buf bytes.Buffer

	err := buildAny(reflect.ValueOf(v), &buf, "")
	return buf.Bytes(), err
}

func buildAny(value reflect.Value, buf *bytes.Buffer, tag reflect.StructTag) error {
	value = reflect.Indirect(value)
	if !value.IsValid() {
		return nil
	}

	vtype := value.Type()

	t := tag.Get("type")
	if t == "" {
		switch vtype.Kind() {
		case reflect.Struct:
			// also it can't be a time object
			if _, ok := value.Interface().(time.Time); !ok {
				t = "structure"
			}
		case reflect.Slice:
			// also it can't be a byte slice
			if _, ok := value.Interface().([]byte); !ok {
				t = "list"
			}
		case reflect.Map:
			t = "map"
		}
	}

	switch t {
	case "structure":
		if field, ok := vtype.FieldByName("SDKShapeTraits"); ok {
			tag = field.Tag
		}
		return buildStruct(value, buf, tag)
	case "list":
		return buildList(value, buf, tag)
	case "map":
		return buildMap(value, buf, tag)
	default:
		return buildScalar(value, buf, tag)
	}
}

func buildStruct(value reflect.Value, buf *bytes.Buffer, tag reflect.StructTag) error {
	if !value.IsValid() {
		return nil
	}

	buf.WriteString("{")

	t, fields := value.Type(), []*reflect.StructField{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		member := value.FieldByName(field.Name)
		if (member.Kind() == reflect.Ptr || member.Kind() == reflect.Slice || member.Kind() == reflect.Map) && member.IsNil() {
			continue // ignore unset fields
		}
		if c := field.Name[0:1]; strings.ToLower(c) == c {
			continue // ignore unexported fields
		}
		if field.Tag.Get("location") != "" {
			continue // ignore non-body elements
		}

		fields = append(fields, &field)
	}

	for i, field := range fields {
		member := value.FieldByName(field.Name)

		// figure out what this field is called
		name := field.Name
		if locName := field.Tag.Get("locationName"); locName != "" {
			name = locName
		}

		buf.WriteString(fmt.Sprintf("%q:", name))

		err := buildAny(member, buf, field.Tag)
		if err != nil {
			return err
		}

		if i < len(fields)-1 {
			buf.WriteString(",")
		}
	}

	buf.WriteString("}")

	return nil
}

func buildList(value reflect.Value, buf *bytes.Buffer, tag reflect.StructTag) error {
	buf.WriteString("[")

	for i := 0; i < value.Len(); i++ {
		buildAny(value.Index(i), buf, "")

		if i < value.Len()-1 {
			buf.WriteString(",")
		}
	}

	buf.WriteString("]")

	return nil
}

func buildMap(value reflect.Value, buf *bytes.Buffer, tag reflect.StructTag) error {
	buf.WriteString("{")

	keys := make([]string, value.Len())
	for i, n := range value.MapKeys() {
		keys[i] = n.String()
	}
	sort.Strings(keys)

	for i, k := range keys {
		buf.WriteString(fmt.Sprintf("%q:", k))
		buildAny(value.MapIndex(reflect.ValueOf(k)), buf, "")

		if i < len(keys)-1 {
			buf.WriteString(",")
		}
	}

	buf.WriteString("}")

	return nil
}

func buildScalar(value reflect.Value, buf *bytes.Buffer, tag reflect.StructTag) error {
	switch converted := value.Interface().(type) {
	case string:
		writeString(converted, buf)
	case []byte:
		if !value.IsNil() {
			buf.WriteString(fmt.Sprintf("%q", base64.StdEncoding.EncodeToString(converted)))
		}
	case bool:
		buf.WriteString(strconv.FormatBool(converted))
	case int64:
		buf.WriteString(strconv.FormatInt(converted, 10))
	case float64:
		buf.WriteString(strconv.FormatFloat(converted, 'f', -1, 64))
	case time.Time:
		buf.WriteString(strconv.FormatInt(converted.UTC().Unix(), 10))
	default:
		return fmt.Errorf("unsupported JSON value %v (%s)", value.Interface(), value.Type())
	}
	return nil
}

func writeString(s string, buf *bytes.Buffer) {
	buf.WriteByte('"')
	for _, r := range s {
		if r == '"' {
			buf.WriteString(`\"`)
		} else if r == '\\' {
			buf.WriteString(`\\`)
		} else if r == '\b' {
			buf.WriteString(`\b`)
		} else if r == '\f' {
			buf.WriteString(`\f`)
		} else if r == '\r' {
			buf.WriteString(`\r`)
		} else if r == '\t' {
			buf.WriteString(`\t`)
		} else if r == '\n' {
			buf.WriteString(`\n`)
		} else if r < 32 {
			fmt.Fprintf(buf, "\\u%0.4x", r)
		} else {
			buf.WriteRune(r)
		}
	}
	buf.WriteByte('"')
}
