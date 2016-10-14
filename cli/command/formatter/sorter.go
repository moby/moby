package formatter

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

type genericStructSorter struct {
	data     []interface{}
	bys      []string
	ordering []string
	indexes  [][]int
}

func (s genericStructSorter) Len() int {
	return len(s.data)
}

func (s genericStructSorter) Swap(i, j int) {
	s.data[i], s.data[j] = s.data[j], s.data[i]
}

const (
	ascendantKey  = "asc"
	descendantKey = "dsc"
)

func validateType(i interface{}, j interface{}) error {
	iType := reflect.TypeOf(i)
	jType := reflect.TypeOf(j)
	if iType.Kind() != reflect.Struct {
		return fmt.Errorf("data must be of type struct")
	}
	if iType.PkgPath() != jType.PkgPath() || iType.Name() != jType.Name() || iType.Kind() != jType.Kind() {
		return fmt.Errorf("data are not of the same type: (%v, %v, %v) != (%v, %v, %v)",
			iType.PkgPath(), iType.Name(), iType.Kind(), jType.PkgPath(), jType.Name(), jType.Kind())
	}
	return nil
}

func findFieldIndex(t reflect.Type, name string, index *[]int) bool {
	match := func(s string) bool {
		return strings.EqualFold(s, name)
	}
	f, ok := t.FieldByNameFunc(match)
	if ok {
		*index = append(f.Index, *index...)
		return true
	}

	for i := 0; i < t.NumField(); i++ {
		nt := t.Field(i).Type
		if t.Kind() == reflect.Ptr {
			nt = nt.Elem()
		}
		if nt.Kind() == reflect.Struct {
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
	gs.data = data
	gs.bys = bys
	d := data[0]
	for i := 1; i < len(data); i++ {
		if err := validateType(d, data[i]); err != nil {
			return gs, err
		}
	}
	for idx, by := range bys {
		parts := strings.Split(by, ":")
		by = parts[0]
		dir := ascendantKey
		if len(parts) > 1 && (dir == ascendantKey || dir == descendantKey) {
			dir = parts[1]
		}
		gs.bys[idx] = by
		gs.ordering = append(gs.ordering, dir)

		fieldIndex := []int{}
		ok := findFieldIndex(reflect.TypeOf(d), by, &fieldIndex)
		if !ok {
			return gs, fmt.Errorf("field %s was not found in struct", by)
		}
		gs.indexes = append(gs.indexes, fieldIndex)
	}

	return gs, nil
}

func (s genericStructSorter) Less(i, j int) bool {
	di := s.data[i]
	dj := s.data[j]

	diType := reflect.TypeOf(di)
	diValue := reflect.ValueOf(di)

	djType := reflect.TypeOf(dj)
	djValue := reflect.ValueOf(dj)

	lastByIdx := len(s.bys) - 1
	for byIdx := range s.bys {
		var (
			diField, djField           reflect.StructField
			diFieldValue, djFieldValue reflect.Value
		)

		diCurType := diType
		diCurValue := diValue
		djCurType := djType
		djCurValue := djValue

		for _, idx := range s.indexes[byIdx] {
			diField = diCurType.Field(idx)
			diFieldValue = diCurValue.Field(idx)

			djField = djCurType.Field(idx)
			djFieldValue = djCurValue.Field(idx)

			if diField.Type.Kind() == reflect.Ptr {
				if diFieldValue.IsNil() && djFieldValue.IsNil() {
					return false
				} else if diFieldValue.IsNil() || djFieldValue.IsNil() {
					if diFieldValue.IsNil() {
						return true
					}
					return false
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

		dir := s.ordering[byIdx]
		res := 0
		switch diCurType.Kind() {
		case reflect.String:
			res = lessForString(diCurValue, djCurValue, dir == ascendantKey, byIdx == lastByIdx)
		case reflect.Bool:
			res = lessForBool(diCurValue, djCurValue, dir == ascendantKey, byIdx == lastByIdx)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			res = lessForInt(diCurValue, djCurValue, dir == ascendantKey, byIdx == lastByIdx)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			res = lessForUint(diCurValue, djCurValue, dir == ascendantKey, byIdx == lastByIdx)
		case reflect.Float32, reflect.Float64:
			res = lessForFloat(diCurValue, djCurValue, dir == ascendantKey, byIdx == lastByIdx)
		default:
			switch diCurType {
			case reflect.TypeOf(time.Time{}):
				res = lessForTime(diCurValue, djCurValue, dir == ascendantKey, byIdx == lastByIdx)
			default:
				panic(fmt.Sprintf("genericStructSorter: comparison of %v are not supported", diCurType.Kind()))
			}
		}

		if res != 0 {
			return res == -1
		}
	}
	return true
}

func lessForString(i, j reflect.Value, asc, last bool) int {
	vi, vj := i.String(), j.String()
	if vi == vj && !last {
		return 0
	}
	if asc {
		if vi < vj {
			return -1
		}
		return 1
	}
	if vi > vj {
		return -1
	}
	return 1
}

func lessForBool(i, j reflect.Value, asc, last bool) int {
	vi, vj := i.Bool(), j.Bool()
	if vi == vj && !last {
		return 0
	}
	if asc {
		if !vi && vj {
			return -1
		}
		return 1
	}
	if vi && !vj {
		return -1
	}
	return 1
}

func lessForInt(i, j reflect.Value, asc, last bool) (ret int) {
	vi, vj := i.Int(), j.Int()
	if vi == vj && !last {
		return 0
	}
	if asc {
		if vi < vj {
			return -1
		}
		return 1
	}
	if vi > vj {
		return -1
	}
	return 1
}

func lessForUint(i, j reflect.Value, asc, last bool) int {
	vi, vj := i.Uint(), j.Uint()
	if vi == vj && !last {
		return 0
	}
	if asc {
		if vi < vj {
			return -1
		}
		return 1
	}
	if vi > vj {
		return -1
	}
	return 1
}

func lessForFloat(i, j reflect.Value, asc, last bool) int {
	vi, vj := i.Float(), j.Float()
	if vi == vj && !last {
		return 0
	}
	if asc {
		if vi < vj {
			return -1
		}
		return 1
	}
	if vi > vj {
		return -1
	}
	return 1
}

func lessForTime(i, j reflect.Value, asc, last bool) int {
	vi, vj := i.Interface().(time.Time), j.Interface().(time.Time)
	if vi.Equal(vj) && !last {
		return 0
	}
	if asc {
		if vi.Before(vj) {
			return -1
		}
		return 1
	}
	if vi.After(vj) {
		return -1
	}
	return 1
}
