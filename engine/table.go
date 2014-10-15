package engine

import (
	"bytes"
	"encoding/json"
	"io"
	"sort"
	"strconv"
)

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
			n++
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
}
