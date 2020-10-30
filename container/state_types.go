package container // import "github.com/docker/docker/container"

import (
	"encoding/json"
	"sync/atomic"
	"time"
)

// sbool marshals to true/false booleans
type sbool struct {
	v int64
}

func (b *sbool) Set(v bool) {
	if v {
		atomic.StoreInt64(&b.v, 1)
	} else {
		atomic.StoreInt64(&b.v, 0)
	}
}

func (b *sbool) Get() bool {
	return atomic.LoadInt64(&b.v) == 1
}

func (b *sbool) UnmarshalJSON(buf []byte) error {
	var tmp bool
	if err := json.Unmarshal(buf, &tmp); err != nil {
		return err
	}
	if tmp {
		b.Set(true)
	} else {
		b.Set(false)
	}
	return nil
}

func (b sbool) MarshalJSON() ([]byte, error) {
	switch atomic.LoadInt64(&b.v) {
	case 0:
		return []byte("false"), nil
	case 1:
		return []byte("true"), nil
	}
	return []byte("false"), nil
}

type sstring struct {
	v atomic.Value
}

func (b *sstring) Set(v string) {
	b.v.Store(v)
}

func (b *sstring) Get() string {
	s := b.v.Load()
	if s == nil {
		return ""
	}
	return s.(string)
}

func (b *sstring) UnmarshalJSON(buf []byte) error {
	var s string
	if err := json.Unmarshal(buf, &s); err != nil {
		return err
	}
	b.Set(s)
	return nil
}

func (b sstring) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.v.Load())
}

type stime struct {
	v atomic.Value
}

func (t *stime) Set(v time.Time) {
	t.v.Store(v)
}

func (t *stime) Get() time.Time {
	s := t.v.Load()
	if s == nil {
		return time.Time{}
	}
	return s.(time.Time)
}

func (t *stime) UnmarshalJSON(buf []byte) error {
	var ts time.Time
	if err := json.Unmarshal(buf, &ts); err != nil {
		return err
	}
	t.Set(ts)
	return nil
}

func (t stime) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.v.Load())
}
