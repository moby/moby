package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Env []string

// Get returns the last value associated with the given key. If there are no
// values associated with the key, Get returns the empty string.
func (env *Env) Get(key string) (value string) {
	// not using Map() because of the extra allocations https://github.com/docker/docker/pull/7488#issuecomment-51638315
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

// Len returns the number of keys in the environment.
// Note that len(env) might be different from env.Len(),
// because the same key might be set multiple times.
func (env *Env) Len() int {
	return len(env.Map())
}

func (env *Env) Init(src *Env) {
	(*env) = make([]string, 0, len(*src))
	for _, val := range *src {
		(*env) = append((*env), val)
	}
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
	// Issue 7941 - if the value in the incoming JSON is null then treat it
	// as if they never specified the property at all.
	if v == nil {
		return
	}

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

func changeFloats(v interface{}) interface{} {
	switch v := v.(type) {
	case float64:
		return int(v)
	case map[string]interface{}:
		for key, val := range v {
			v[key] = changeFloats(val)
		}
	case []interface{}:
		for idx, val := range v {
			v[idx] = changeFloats(val)
		}
	}
	return v
}

func (env *Env) Encode(dst io.Writer) error {
	m := make(map[string]interface{})
	for k, v := range env.Map() {
		var val interface{}
		if err := json.Unmarshal([]byte(v), &val); err == nil {
			// FIXME: we fix-convert float values to int, because
			// encoding/json decodes integers to float64, but cannot encode them back.
			// (See http://golang.org/src/pkg/encoding/json/decode.go#L46)
			m[k] = changeFloats(val)
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

// MultiMap returns a representation of env as a
// map of string arrays, keyed by string.
// This is the same structure as http headers for example,
// which allow each key to have multiple values.
func (env *Env) MultiMap() map[string][]string {
	m := make(map[string][]string)
	for _, kv := range *env {
		parts := strings.SplitN(kv, "=", 2)
		m[parts[0]] = append(m[parts[0]], parts[1])
	}
	return m
}

// InitMultiMap removes all values in env, then initializes
// new values from the contents of m.
func (env *Env) InitMultiMap(m map[string][]string) {
	(*env) = make([]string, 0, len(m))
	for k, vals := range m {
		for _, v := range vals {
			env.Set(k, v)
		}
	}
}
