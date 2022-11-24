package testing

import (
	"fmt"
	fuzz "github.com/AdaLogics/go-fuzz-headers"
	"os"
	"reflect"
)

type F struct {
	Data     []byte
	T        *T
	FuzzFunc func(*T, any)
}

func (f *F) CleanupTempDirs() {
	f.T.CleanupTempDirs()
}

func (f *F) Add(args ...any)                   {}
func (c *F) Cleanup(f func())                  {}
func (c *F) Error(args ...any)                 {}
func (c *F) Errorf(format string, args ...any) {}
func (f *F) Fail()                             {}
func (c *F) FailNow()                          {}
func (c *F) Failed() bool                      { return false }
func (c *F) Fatal(args ...any)                 {}
func (c *F) Fatalf(format string, args ...any) {}
func (f *F) Fuzz(ff any) {
	// we are assuming that ff is a func.
	// TODO: Add a check for UX purposes

	fn := reflect.ValueOf(ff)
	fnType := fn.Type()
	var types []reflect.Type
	for i := 1; i < fnType.NumIn(); i++ {
		t := fnType.In(i)

		types = append(types, t)
	}
	args := []reflect.Value{reflect.ValueOf(f.T)}
	fuzzConsumer := fuzz.NewConsumer(f.Data)
	for _, v := range types {
		switch v.String() {
		case "[]uint8":
			b, err := fuzzConsumer.GetBytes()
			if err != nil {
				return
			}
			newBytes := reflect.New(v)
			newBytes.Elem().SetBytes(b)
			args = append(args, newBytes.Elem())
		case "string":
			s, err := fuzzConsumer.GetString()
			if err != nil {
				return
			}
			newString := reflect.New(v)
			newString.Elem().SetString(s)
			args = append(args, newString.Elem())
		case "int":
			randInt, err := fuzzConsumer.GetInt()
			if err != nil {
				return
			}
			newInt := reflect.New(v)
			newInt.Elem().SetInt(int64(randInt))
			args = append(args, newInt.Elem())
		case "int8":
			randInt, err := fuzzConsumer.GetInt()
			if err != nil {
				return
			}
			newInt := reflect.New(v)
			newInt.Elem().SetInt(int64(randInt))
			args = append(args, newInt.Elem())
		case "int16":
			randInt, err := fuzzConsumer.GetInt()
			if err != nil {
				return
			}
			newInt := reflect.New(v)
			newInt.Elem().SetInt(int64(randInt))
			args = append(args, newInt.Elem())
		case "int32":
			randInt, err := fuzzConsumer.GetInt()
			if err != nil {
				return
			}
			newInt := reflect.New(v)
			newInt.Elem().SetInt(int64(randInt))
			args = append(args, newInt.Elem())
		case "int64":
			randInt, err := fuzzConsumer.GetInt()
			if err != nil {
				return
			}
			newInt := reflect.New(v)
			newInt.Elem().SetInt(int64(randInt))
			args = append(args, newInt.Elem())
		case "uint":
			randInt, err := fuzzConsumer.GetInt()
			if err != nil {
				return
			}
			newUint := reflect.New(v)
			newUint.Elem().SetUint(uint64(randInt))
			args = append(args, newUint.Elem())
		case "uint8":
			randInt, err := fuzzConsumer.GetInt()
			if err != nil {
				return
			}
			newUint := reflect.New(v)
			newUint.Elem().SetUint(uint64(randInt))
			args = append(args, newUint.Elem())
		case "uint16":
			randInt, err := fuzzConsumer.GetUint16()
			if err != nil {
				return
			}
			newUint16 := reflect.New(v)
			newUint16.Elem().SetUint(uint64(randInt))
			args = append(args, newUint16.Elem())
		case "uint32":
			randInt, err := fuzzConsumer.GetUint32()
			if err != nil {
				return
			}
			newUint32 := reflect.New(v)
			newUint32.Elem().SetUint(uint64(randInt))
			args = append(args, newUint32.Elem())
		case "uint64":
			randInt, err := fuzzConsumer.GetUint64()
			if err != nil {
				return
			}
			newUint64 := reflect.New(v)
			newUint64.Elem().SetUint(uint64(randInt))
			args = append(args, newUint64.Elem())
		case "rune":
			randRune, err := fuzzConsumer.GetRune()
			if err != nil {
				return
			}
			newRune := reflect.New(v)
			newRune.Elem().Set(reflect.ValueOf(randRune))
			args = append(args, newRune.Elem())
		case "float32":
			randFloat, err := fuzzConsumer.GetFloat32()
			if err != nil {
				return
			}
			newFloat := reflect.New(v)
			newFloat.Elem().Set(reflect.ValueOf(randFloat))
			args = append(args, newFloat.Elem())
		case "float64":
			randFloat, err := fuzzConsumer.GetFloat64()
			if err != nil {
				return
			}
			newFloat := reflect.New(v)
			newFloat.Elem().Set(reflect.ValueOf(randFloat))
			args = append(args, newFloat.Elem())
		case "bool":
			randBool, err := fuzzConsumer.GetBool()
			if err != nil {
				return
			}
			newBool := reflect.New(v)
			newBool.Elem().Set(reflect.ValueOf(randBool))
			args = append(args, newBool.Elem())
		default:
			fmt.Println(v.String())
		}
	}
	fn.Call(args)
}
func (f *F) Helper() {}
func (c *F) Log(args ...any) {
	fmt.Println(args...)
}
func (c *F) Logf(format string, args ...any) {
	fmt.Println(format, args)
}
func (c *F) Name() string             { return "libFuzzer" }
func (c *F) Setenv(key, value string) {}
func (c *F) Skip(args ...any) {
	panic("GO-FUZZ-BUILD-PANIC")
}
func (c *F) SkipNow() {
	panic("GO-FUZZ-BUILD-PANIC")
}
func (c *F) Skipf(format string, args ...any) {
	panic("GO-FUZZ-BUILD-PANIC")
}
func (f *F) Skipped() bool { return false }

func (f *F) TempDir() string {
	dir, err := os.MkdirTemp("", "fuzzdir-")
	if err != nil {
		panic(err)
	}
	f.T.TempDirs = append(f.T.TempDirs, dir)

	return dir
}
