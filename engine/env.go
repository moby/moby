package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type Env map[string]interface{}

func (env *Env) get(key string) interface{} {
	if value, exists := (*env)[key]; exists {
		return value
	}
	return nil
}

func (env *Env) set(key string, value interface{}) {
	(*env)[key] = value
}

// If value doesn't exists: return ""
// If value isn't a string: return ""
// Otherwise: return value
func (env *Env) Get(key string) string {
	value := env.get(key)
	if value == nil {
		return ""
	}
	if sval, ok := value.(string); ok {
		return sval
	}
	return ""
}

func (env *Env) Set(key string, value string) {
	env.set(key, value)
}

func (env *Env) Exists(key string) bool {
	_, exists := (*env)[key]
	return exists
}

// If value doesn't exists: return false
// If value isn't a bool: return false
// Otherwise: return value
func (env *Env) GetBool(key string) bool {
	value := env.get(key)
	if value == nil {
		return false
	}
	if bval, ok := value.(bool); ok {
		return bval
	}
	return false
}

func (env *Env) SetBool(key string, value bool) {
	env.set(key, value)
}

// If value doesn't exists: return -1
// If value isn't an int: return -1
// Otherwise: return value
func (env *Env) GetInt(key string) int {
	return int(env.GetInt64(key))
}

func (env *Env) SetInt(key string, value int) {
	env.set(key, value)
}

// If value doesn't exists: return -1
// If value isn't an int: return -1
// Otherwise: return value
func (env *Env) GetInt64(key string) int64 {
	value := env.get(key)
	if value == nil {
		return -1
	}
	if nval, ok := value.(json.Number); ok {
		ival, _ := nval.Int64()
		return ival
	}
	if ival, ok := value.(int64); ok {
		return ival
	}
	if ival, ok := value.(int); ok {
		return int64(ival)
	}
	if fval, ok := value.(float64); ok {
		return int64(fval)
	}
	return -1
}

func (env *Env) SetInt64(key string, value int64) {
	env.set(key, value)
}

// If value doesn't exists: return nil
// If value is a []string : return value
// If value is a string: return []string{value}
// Otherwise: return value
func (env *Env) GetList(key string) []string {
	value := env.get(key)
	if value == nil {
		return nil
	}
	if aval, ok := value.([]string); ok {
		return aval
	}
	if sval, ok := value.(string); ok {
		return []string{sval}
	}
	return nil
}

func (env *Env) SetList(key string, value []string) {
	env.set(key, value)
}

// If value doesn't exists: return nil
// Otherwise: return unmarshalled value
func (env *Env) GetJson(key string) interface{} {
	return env.get(key)
}

func (env *Env) SetJson(key string, value interface{}) {
	env.set(key, value)
}

func NewDecoder(src io.Reader) *Decoder {
	dec := json.NewDecoder(src)
	dec.UseNumber()
	return &Decoder{
		dec,
	}
}

type Decoder struct {
	*json.Decoder
}

func (decoder *Decoder) Decode() (*Env, error) {
	env := make(Env)
	if err := decoder.Decoder.Decode(&env); err != nil {
		return nil, err
	}
	return &env, nil
}

// DecodeEnv decodes `src` as a json dictionary, and adds
// each decoded key-value pair to the environment.
//
// If `src` cannot be decoded as a json dictionary, an error
// is returned.
func (env *Env) Decode(src io.Reader) error {
	dec := json.NewDecoder(src)
	dec.UseNumber()
	if err := dec.Decode(env); err != nil {
		return err
	}
	return nil
}

func (env *Env) Encode(dst io.Writer) error {
	if err := json.NewEncoder(dst).Encode(&env); err != nil {
		return err
	}
	return nil
}

func (env *Env) WriteTo(dst io.Writer) (n int64, err error) {
	// FIXME: return the number of bytes written to respect io.WriterTo
	return 0, env.Encode(dst)
}

func (env *Env) Export(dst interface{}) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("ExportEnv %s", err)
		}
	}()
	var buf bytes.Buffer
	// step 1: encode/marshal the env to an intermediary json representation
	if err := env.Encode(&buf); err != nil {
		return err
	}
	// step 2: decode/unmarshal the intermediary json into the destination object
	dec := json.NewDecoder(&buf)
	dec.UseNumber()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return nil
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
	for k, v := range *env {
		m[k] = fmt.Sprintf("%v", v)
	}
	return m
}
