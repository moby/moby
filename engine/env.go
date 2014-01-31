package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

type Env []string

func (env *Env) Get(key string) (value string) {
	// FIXME: use Map()
	for _, kv := range *env {
		if strings.Index(kv, "=") == -1 {
			continue
		}
		parts := strings.SplitN(kv, "=", 2)
		if parts[0] != key {
			continue
		}
		if len(parts) < 2 {
			value = ""
		} else {
			value = parts[1]
		}
	}
	return
}

func (env *Env) Exists(key string) bool {
	_, exists := env.Map()[key]
	return exists
}

func (env *Env) GetBool(key string) (value bool) {
	s := strings.ToLower(strings.Trim(env.Get(key), " \t"))
	if s == "" || s == "0" || s == "no" || s == "false" || s == "none" {
		return false
	}
	return true
}

func (env *Env) SetBool(key string, value bool) {
	if value {
		env.Set(key, "1")
	} else {
		env.Set(key, "0")
	}
}

func (env *Env) GetInt(key string) int {
	return int(env.GetInt64(key))
}

func (env *Env) GetInt64(key string) int64 {
	s := strings.Trim(env.Get(key), " \t")
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return val
}

func (env *Env) SetInt(key string, value int) {
	env.Set(key, fmt.Sprintf("%d", value))
}

func (env *Env) SetInt64(key string, value int64) {
	env.Set(key, fmt.Sprintf("%d", value))
}

// Returns nil if key not found
func (env *Env) GetList(key string) []string {
	sval := env.Get(key)
	if sval == "" {
		return nil
	}
	l := make([]string, 0, 1)
	if err := json.Unmarshal([]byte(sval), &l); err != nil {
		l = append(l, sval)
	}
	return l
}

func (env *Env) GetSubEnv(key string) *Env {
	sval := env.Get(key)
	if sval == "" {
		return nil
	}
	buf := bytes.NewBufferString(sval)
	var sub Env
	if err := sub.Decode(buf); err != nil {
		return nil
	}
	return &sub
}

func (env *Env) SetSubEnv(key string, sub *Env) error {
	var buf bytes.Buffer
	if err := sub.Encode(&buf); err != nil {
		return err
	}
	env.Set(key, string(buf.Bytes()))
	return nil
}

func (env *Env) GetJson(key string, iface interface{}) error {
	sval := env.Get(key)
	if sval == "" {
		return nil
	}
	return json.Unmarshal([]byte(sval), iface)
}

func (env *Env) SetJson(key string, value interface{}) error {
	sval, err := json.Marshal(value)
	if err != nil {
		return err
	}
	env.Set(key, string(sval))
	return nil
}

func (env *Env) SetList(key string, value []string) error {
	return env.SetJson(key, value)
}

func (env *Env) Set(key, value string) {
	*env = append(*env, key+"="+value)
}

func NewDecoder(src io.Reader) *Decoder {
	return &Decoder{
		json.NewDecoder(src),
	}
}

type Decoder struct {
	*json.Decoder
}

func (decoder *Decoder) Decode() (*Env, error) {
	m := make(map[string]interface{})
	if err := decoder.Decoder.Decode(&m); err != nil {
		return nil, err
	}
	env := &Env{}
	for key, value := range m {
		env.SetAuto(key, value)
	}
	return env, nil
}

// DecodeEnv decodes `src` as a json dictionary, and adds
// each decoded key-value pair to the environment.
//
// If `src` cannot be decoded as a json dictionary, an error
// is returned.
func (env *Env) Decode(src io.Reader) error {
	m := make(map[string]interface{})
	if err := json.NewDecoder(src).Decode(&m); err != nil {
		return err
	}
	for k, v := range m {
		env.SetAuto(k, v)
	}
	return nil
}

func (env *Env) SetAuto(k string, v interface{}) {
	// FIXME: we fix-convert float values to int, because
	// encoding/json decodes integers to float64, but cannot encode them back.
	// (See http://golang.org/src/pkg/encoding/json/decode.go#L46)
	if fval, ok := v.(float64); ok {
		env.SetInt64(k, int64(fval))
	} else if sval, ok := v.(string); ok {
		env.Set(k, sval)
	} else if val, err := json.Marshal(v); err == nil {
		env.Set(k, string(val))
	} else {
		env.Set(k, fmt.Sprintf("%v", v))
	}
}

func (env *Env) Encode(dst io.Writer) error {
	m := make(map[string]interface{})
	for k, v := range env.Map() {
		var val interface{}
		if err := json.Unmarshal([]byte(v), &val); err == nil {
			// FIXME: we fix-convert float values to int, because
			// encoding/json decodes integers to float64, but cannot encode them back.
			// (See http://golang.org/src/pkg/encoding/json/decode.go#L46)
			if fval, isFloat := val.(float64); isFloat {
				val = int(fval)
			}
			m[k] = val
		} else {
			m[k] = v
		}
	}
	if err := json.NewEncoder(dst).Encode(&m); err != nil {
		return err
	}
	return nil
}

func (env *Env) WriteTo(dst io.Writer) (n int64, err error) {
	// FIXME: return the number of bytes written to respect io.WriterTo
	return 0, env.Encode(dst)
}

func (env *Env) Import(src interface{}) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("ImportEnv: %s", err)
		}
	}()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(src); err != nil {
		return err
	}
	if err := env.Decode(&buf); err != nil {
		return err
	}
	return nil
}

func (env *Env) Map() map[string]string {
	m := make(map[string]string)
	for _, kv := range *env {
		parts := strings.SplitN(kv, "=", 2)
		m[parts[0]] = parts[1]
	}
	return m
}

type Table struct {
	Data    []*Env
	sortKey string
	Chan    chan *Env
}

func NewTable(sortKey string, sizeHint int) *Table {
	return &Table{
		make([]*Env, 0, sizeHint),
		sortKey,
		make(chan *Env),
	}
}

func (t *Table) SetKey(sortKey string) {
	t.sortKey = sortKey
}

func (t *Table) Add(env *Env) {
	t.Data = append(t.Data, env)
}

func (t *Table) Len() int {
	return len(t.Data)
}

func (t *Table) Less(a, b int) bool {
	return t.lessBy(a, b, t.sortKey)
}

func (t *Table) lessBy(a, b int, by string) bool {
	keyA := t.Data[a].Get(by)
	keyB := t.Data[b].Get(by)
	intA, errA := strconv.ParseInt(keyA, 10, 64)
	intB, errB := strconv.ParseInt(keyB, 10, 64)
	if errA == nil && errB == nil {
		return intA < intB
	}
	return keyA < keyB
}

func (t *Table) Swap(a, b int) {
	tmp := t.Data[a]
	t.Data[a] = t.Data[b]
	t.Data[b] = tmp
}

func (t *Table) Sort() {
	sort.Sort(t)
}

func (t *Table) ReverseSort() {
	sort.Sort(sort.Reverse(t))
}

func (t *Table) WriteListTo(dst io.Writer) (n int64, err error) {
	if _, err := dst.Write([]byte{'['}); err != nil {
		return -1, err
	}
	n = 1
	for i, env := range t.Data {
		bytes, err := env.WriteTo(dst)
		if err != nil {
			return -1, err
		}
		n += bytes
		if i != len(t.Data)-1 {
			if _, err := dst.Write([]byte{','}); err != nil {
				return -1, err
			}
			n += 1
		}
	}
	if _, err := dst.Write([]byte{']'}); err != nil {
		return -1, err
	}
	return n + 1, nil
}

func (t *Table) ToListString() (string, error) {
	buffer := bytes.NewBuffer(nil)
	if _, err := t.WriteListTo(buffer); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func (t *Table) WriteTo(dst io.Writer) (n int64, err error) {
	for _, env := range t.Data {
		bytes, err := env.WriteTo(dst)
		if err != nil {
			return -1, err
		}
		n += bytes
	}
	return n, nil
}

func (t *Table) ReadListFrom(src []byte) (n int64, err error) {
	var array []interface{}

	if err := json.Unmarshal(src, &array); err != nil {
		return -1, err
	}

	for _, item := range array {
		if m, ok := item.(map[string]interface{}); ok {
			env := &Env{}
			for key, value := range m {
				env.SetAuto(key, value)
			}
			t.Add(env)
		}
	}

	return int64(len(src)), nil
}

func (t *Table) ReadFrom(src io.Reader) (n int64, err error) {
	decoder := NewDecoder(src)
	for {
		env, err := decoder.Decode()
		if err == io.EOF {
			return 0, nil
		} else if err != nil {
			return -1, err
		}
		t.Add(env)
	}
	return 0, nil
}
