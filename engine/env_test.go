package engine

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/docker/docker/pkg/testutils"
)

func TestEnvLenZero(t *testing.T) {
	env := &Env{}
	if env.Len() != 0 {
		t.Fatalf("%d", env.Len())
	}
}

func TestEnvLenNotZero(t *testing.T) {
	env := &Env{}
	env.Set("foo", "bar")
	env.Set("ga", "bu")
	if env.Len() != 2 {
		t.Fatalf("%d", env.Len())
	}
}

func TestEnvLenDup(t *testing.T) {
	env := &Env{
		"foo=bar",
		"foo=baz",
		"a=b",
	}
	// len(env) != env.Len()
	if env.Len() != 2 {
		t.Fatalf("%d", env.Len())
	}
}

func TestEnvGetDup(t *testing.T) {
	env := &Env{
		"foo=bar",
		"foo=baz",
		"foo=bif",
	}
	expected := "bif"
	if v := env.Get("foo"); v != expected {
		t.Fatalf("expect %q, got %q", expected, v)
	}
}

func TestNewJob(t *testing.T) {
	job := mkJob(t, "dummy", "--level=awesome")
	if job.Name != "dummy" {
		t.Fatalf("Wrong job name: %s", job.Name)
	}
	if len(job.Args) != 1 {
		t.Fatalf("Wrong number of job arguments: %d", len(job.Args))
	}
	if job.Args[0] != "--level=awesome" {
		t.Fatalf("Wrong job arguments: %s", job.Args[0])
	}
}

func TestSetenv(t *testing.T) {
	job := mkJob(t, "dummy")
	job.Setenv("foo", "bar")
	if val := job.Getenv("foo"); val != "bar" {
		t.Fatalf("Getenv returns incorrect value: %s", val)
	}

	job.Setenv("bar", "")
	if val := job.Getenv("bar"); val != "" {
		t.Fatalf("Getenv returns incorrect value: %s", val)
	}
	if val := job.Getenv("nonexistent"); val != "" {
		t.Fatalf("Getenv returns incorrect value: %s", val)
	}
}

func TestSetenvBool(t *testing.T) {
	job := mkJob(t, "dummy")
	job.SetenvBool("foo", true)
	if val := job.GetenvBool("foo"); !val {
		t.Fatalf("GetenvBool returns incorrect value: %t", val)
	}

	job.SetenvBool("bar", false)
	if val := job.GetenvBool("bar"); val {
		t.Fatalf("GetenvBool returns incorrect value: %t", val)
	}

	if val := job.GetenvBool("nonexistent"); val {
		t.Fatalf("GetenvBool returns incorrect value: %t", val)
	}
}

func TestSetenvTime(t *testing.T) {
	job := mkJob(t, "dummy")

	now := time.Now()
	job.SetenvTime("foo", now)
	if val, err := job.GetenvTime("foo"); err != nil {
		t.Fatalf("GetenvTime failed to parse: %v", err)
	} else {
		nowStr := now.Format(time.RFC3339)
		valStr := val.Format(time.RFC3339)
		if nowStr != valStr {
			t.Fatalf("GetenvTime returns incorrect value: %s, Expected: %s", valStr, nowStr)
		}
	}

	job.Setenv("bar", "Obviously I'm not a date")
	if val, err := job.GetenvTime("bar"); err == nil {
		t.Fatalf("GetenvTime was supposed to fail, instead returned: %s", val)
	}
}

func TestSetenvInt(t *testing.T) {
	job := mkJob(t, "dummy")

	job.SetenvInt("foo", -42)
	if val := job.GetenvInt("foo"); val != -42 {
		t.Fatalf("GetenvInt returns incorrect value: %d", val)
	}

	job.SetenvInt("bar", 42)
	if val := job.GetenvInt("bar"); val != 42 {
		t.Fatalf("GetenvInt returns incorrect value: %d", val)
	}
	if val := job.GetenvInt("nonexistent"); val != 0 {
		t.Fatalf("GetenvInt returns incorrect value: %d", val)
	}
}

func TestSetenvList(t *testing.T) {
	job := mkJob(t, "dummy")

	job.SetenvList("foo", []string{"bar"})
	if val := job.GetenvList("foo"); len(val) != 1 || val[0] != "bar" {
		t.Fatalf("GetenvList returns incorrect value: %v", val)
	}

	job.SetenvList("bar", nil)
	if val := job.GetenvList("bar"); val != nil {
		t.Fatalf("GetenvList returns incorrect value: %v", val)
	}
	if val := job.GetenvList("nonexistent"); val != nil {
		t.Fatalf("GetenvList returns incorrect value: %v", val)
	}
}

func TestEnviron(t *testing.T) {
	job := mkJob(t, "dummy")
	job.Setenv("foo", "bar")
	val, exists := job.Environ()["foo"]
	if !exists {
		t.Fatalf("foo not found in the environ")
	}
	if val != "bar" {
		t.Fatalf("bar not found in the environ")
	}
}

func TestMultiMap(t *testing.T) {
	e := &Env{}
	e.Set("foo", "bar")
	e.Set("bar", "baz")
	e.Set("hello", "world")
	m := e.MultiMap()
	e2 := &Env{}
	e2.Set("old_key", "something something something")
	e2.InitMultiMap(m)
	if v := e2.Get("old_key"); v != "" {
		t.Fatalf("%#v", v)
	}
	if v := e2.Get("bar"); v != "baz" {
		t.Fatalf("%#v", v)
	}
	if v := e2.Get("hello"); v != "world" {
		t.Fatalf("%#v", v)
	}
}

func testMap(l int) [][2]string {
	res := make([][2]string, l)
	for i := 0; i < l; i++ {
		t := [2]string{testutils.RandomString(5), testutils.RandomString(20)}
		res[i] = t
	}
	return res
}

func BenchmarkSet(b *testing.B) {
	fix := testMap(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env := &Env{}
		for _, kv := range fix {
			env.Set(kv[0], kv[1])
		}
	}
}

func BenchmarkSetJson(b *testing.B) {
	fix := testMap(100)
	type X struct {
		f string
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env := &Env{}
		for _, kv := range fix {
			if err := env.SetJson(kv[0], X{kv[1]}); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkGet(b *testing.B) {
	fix := testMap(100)
	env := &Env{}
	for _, kv := range fix {
		env.Set(kv[0], kv[1])
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, kv := range fix {
			env.Get(kv[0])
		}
	}
}

func BenchmarkGetJson(b *testing.B) {
	fix := testMap(100)
	env := &Env{}
	type X struct {
		f string
	}
	for _, kv := range fix {
		env.SetJson(kv[0], X{kv[1]})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, kv := range fix {
			if err := env.GetJson(kv[0], &X{}); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkEncode(b *testing.B) {
	fix := testMap(100)
	env := &Env{}
	type X struct {
		f string
	}
	// half a json
	for i, kv := range fix {
		if i%2 != 0 {
			if err := env.SetJson(kv[0], X{kv[1]}); err != nil {
				b.Fatal(err)
			}
			continue
		}
		env.Set(kv[0], kv[1])
	}
	var writer bytes.Buffer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env.Encode(&writer)
		writer.Reset()
	}
}

func BenchmarkDecode(b *testing.B) {
	fix := testMap(100)
	env := &Env{}
	type X struct {
		f string
	}
	// half a json
	for i, kv := range fix {
		if i%2 != 0 {
			if err := env.SetJson(kv[0], X{kv[1]}); err != nil {
				b.Fatal(err)
			}
			continue
		}
		env.Set(kv[0], kv[1])
	}
	var writer bytes.Buffer
	env.Encode(&writer)
	denv := &Env{}
	reader := bytes.NewReader(writer.Bytes())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := denv.Decode(reader)
		if err != nil {
			b.Fatal(err)
		}
		reader.Seek(0, 0)
	}
}

func TestLongNumbers(t *testing.T) {
	type T struct {
		TestNum int64
	}
	v := T{67108864}
	var buf bytes.Buffer
	e := &Env{}
	e.SetJson("Test", v)
	if err := e.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	res := make(map[string]T)
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res["Test"].TestNum != v.TestNum {
		t.Fatalf("TestNum %d, expected %d", res["Test"].TestNum, v.TestNum)
	}
}

func TestLongNumbersArray(t *testing.T) {
	type T struct {
		TestNum []int64
	}
	v := T{[]int64{67108864}}
	var buf bytes.Buffer
	e := &Env{}
	e.SetJson("Test", v)
	if err := e.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	res := make(map[string]T)
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res["Test"].TestNum[0] != v.TestNum[0] {
		t.Fatalf("TestNum %d, expected %d", res["Test"].TestNum, v.TestNum)
	}
}
