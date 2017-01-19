package formatter

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

const (
	ascendantKey  = "asc"
	descendantKey = "dsc"
)

type orderBy struct {
	fieldName string // order by field with `fieldName` in struct
	ordering  string // asc or dsc
	index     []int  // index of field in struct
}

type genericStructSorter struct {
	data []interface{}
	bys  []orderBy
}

func (s genericStructSorter) Len() int {
	return len(s.data)
}

func (s genericStructSorter) Swap(i, j int) {
	s.data[i], s.data[j] = s.data[j], s.data[i]
}

// validate all the data are of same time
func validateType(data []interface{}) error {
	if len(data) == 0 {
		return nil
	}
	baseType := reflect.TypeOf(data[0])
	if baseType.Kind() != reflect.Struct {
		return fmt.Errorf("data must be of type struct")
	}
	for i := 1; i < len(data); i++ {
		iType := reflect.TypeOf(data[i])
		if iType.PkgPath() != baseType.PkgPath() || iType.Name() != baseType.Name() || iType.Kind() != baseType.Kind() {
			return fmt.Errorf("data are not of the same type: (%v, %v, %v) != (%v, %v, %v)",
				iType.PkgPath(), iType.Name(), iType.Kind(), baseType.PkgPath(), baseType.Name(), baseType.Kind())
		}
	}
	return nil
}

// findFieldIndex find the first occurence of field with name "name"
// if it's a nested field, return array of index
// e.g. [2, 1, 1] means field[2][1][1]
func findFieldIndex(t reflect.Type, name string, index *[]int) bool {
	// TODO: use FieldByNameFunc to compare both lower and upper case
	// It's weird that current implementation doesn't work!
	// match := func(s string) bool {
	//	 return strings.EqualFold(s, name)
	// }
	// f, ok := t.FieldByNameFunc(match)
	f, ok := t.FieldByName(name)
	if ok {
		*index = append(f.Index, *index...)
		return true
	}

	for i := 0; i < t.NumField(); i++ {
		nt := t.Field(i).Type
		if nt.Kind() == reflect.Ptr {
			nt = nt.Elem()
		}
		// don't peek into time.Time{}
		if nt.Kind() == reflect.Struct && nt != reflect.TypeOf(time.Time{}) {
			ok = findFieldIndex(nt, name, index)
			if ok {
				*index = append(t.Field(i).Index, *index...)
				return true
			}
		}
	}

	return false
}

func newGenericStructSorter(data []interface{}, bys []string) (genericStructSorter, error) {
	gs := genericStructSorter{}
	if len(data) == 0 {
		return gs, nil
	}
	if len(bys) == 0 {
		return gs, fmt.Errorf("bys cannot be empty")
	}
	if err := validateType(data); err != nil {
		return gs, err
	}
	// scan and get the sortable fields
	sortableFields := make(map[string]bool)
	if err := getSortableFields(reflect.TypeOf(data[0]), sortableFields); err != nil {
		return gs, fmt.Errorf("can't get sortable fields: %v", err)
	}

	gs.data = data
	for _, by := range bys {
		by = strings.TrimSpace(by)
		parts := strings.Split(by, ":")
		dir := ascendantKey // ascendant key is default
		switch len(parts) {
		case 2:
			switch parts[1] {
			case descendantKey:
				dir = descendantKey
			case ascendantKey, "": // if default, keep as ascendantKey
			default: // doesn't support key other than "asc","dsc",""
				return gs, fmt.Errorf("sort order %q not supported", parts[1])
			}
		case 1: // sort order not specified, keep default
		default:
			return gs, fmt.Errorf("malformed sort key: %q", by)
		}
		by = parts[0]
		if _, ok := sortableFields[by]; !ok {
			var keys []string
			for k := range sortableFields {
				keys = append(keys, k)
			}
			return gs, fmt.Errorf("can't sort by %q, it must be in [%s]", by, strings.Join(keys, ","))
		}
		fieldIndex := []int{}
		ok := findFieldIndex(reflect.TypeOf(data[0]), by, &fieldIndex)
		if !ok { // if by is in sortableFields, then this should never happen
			var keys []string
			for k := range sortableFields {
				keys = append(keys, k)
			}
			return gs, fmt.Errorf("can't find field index of %q, supported fields are [%s]", by, strings.Join(keys, ","))
		}
		gs.bys = append(gs.bys, orderBy{
			fieldName: by,
			ordering:  dir,
			index:     fieldIndex,
		})

	}

	return gs, nil
}

func (s genericStructSorter) Less(i, j int) bool {
	di := s.data[i]
	dj := s.data[j]

	for _, orderBy := range s.bys {
		var (
			diField, djField           reflect.StructField
			diFieldValue, djFieldValue reflect.Value
			diCurType                  = reflect.TypeOf(di)
			diCurValue                 = reflect.ValueOf(di)
			djCurType                  = reflect.TypeOf(dj)
			djCurValue                 = reflect.ValueOf(dj)
		)

		for _, idx := range orderBy.index {
			diField = diCurType.Field(idx)
			diFieldValue = diCurValue.Field(idx)

			djField = djCurType.Field(idx)
			djFieldValue = djCurValue.Field(idx)

			if diField.Type.Kind() == reflect.Ptr {
				// keep nil value before non-nil value
				// Note: for Less() parameters, i is larger than j, e.g. i=1, j=0
				// if djFieldValue(data[0]) is nil, return false to keep their positions
				// or if djFieldValue(data[0]) isn't nil but diFieldValue(data[1]) is nil, return true to swap
				if djFieldValue.IsNil() {
					return false
				} else if diFieldValue.IsNil() {
					return true
				}

				diCurType = diField.Type.Elem()
				diCurValue = diFieldValue.Elem()

				djCurType = djField.Type.Elem()
				djCurValue = djFieldValue.Elem()
			} else {
				diCurType = diField.Type
				diCurValue = diFieldValue

				djCurType = djField.Type
				djCurValue = djFieldValue
			}
		}

		res := 0
		switch diCurType.Kind() {
		case reflect.String:
			res = lessForString(diCurValue, djCurValue)
		case reflect.Bool:
			res = lessForBool(diCurValue, djCurValue)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			res = lessForInt(diCurValue, djCurValue)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			res = lessForUint(diCurValue, djCurValue)
		case reflect.Float32, reflect.Float64:
			res = lessForFloat(diCurValue, djCurValue)
		default:
			switch diCurType {
			case reflect.TypeOf(time.Time{}):
				res = lessForTime(diCurValue, djCurValue)
			default:
				// panic(fmt.Sprintf("genericStructSorter: comparison of %v are not supported", diCurType.Kind()))
				// TODO: find a reasonable logic
				// if it's unsupported field, keep original positions
				return false
			}
		}

		if res != 0 {
			if orderBy.ordering == ascendantKey {
				return res == -1
			}
			return res == 1
		}
	}

	// equal values, return false to keep their postions
	return false
}

func lessForString(i, j reflect.Value) int {
	vi, vj := i.String(), j.String()
	if vi == vj {
		return 0
	}
	if vi < vj {
		return -1
	}
	return 1
}

func lessForBool(i, j reflect.Value) int {
	vi, vj := i.Bool(), j.Bool()
	if vi == vj {
		return 0
	}
	if !vi && vj {
		return -1
	}
	return 1
}

func lessForInt(i, j reflect.Value) (ret int) {
	vi, vj := i.Int(), j.Int()
	if vi == vj {
		return 0
	}
	if vi < vj {
		return -1
	}
	return 1
}

func lessForUint(i, j reflect.Value) int {
	vi, vj := i.Uint(), j.Uint()
	if vi == vj {
		return 0
	}
	if vi < vj {
		return -1
	}
	return 1
}

func lessForFloat(i, j reflect.Value) int {
	vi, vj := i.Float(), j.Float()
	if vi == vj {
		return 0
	}
	if vi < vj {
		return -1
	}
	return 1
}

func lessForTime(i, j reflect.Value) int {
	vi, vj := i.Interface().(time.Time), j.Interface().(time.Time)
	if vi.Equal(vj) {
		return 0
	}
	if vi.Before(vj) {
		return -1
	}
	return 1
}

func getSortableFields(dataType reflect.Type, fields map[string]bool) error {
	if dataType.Kind() != reflect.Struct {
		return fmt.Errorf("data must be of type struct: %v", dataType)
	}

	for i := 0; i < dataType.NumField(); i++ {
		structField := dataType.Field(i)
		fieldType := structField.Type
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}
		switch fieldType.Kind() {
		case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
			fields[structField.Name] = true
		case reflect.Struct:
			if fieldType == reflect.TypeOf(time.Time{}) {
				fields[structField.Name] = true
				continue
			}
			if err := getSortableFields(fieldType, fields); err != nil {
				return err
			}
		default: //ignore other types
		}
	}

	return nil
}
