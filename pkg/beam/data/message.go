package data

import (
	"fmt"
	"strings"
)

type Message string

func Empty() Message {
	return Message(Encode(nil))
}

func Parse(args []string) Message {
	data := make(map[string][]string)
	for _, word := range args {
		if strings.Contains(word, "=") {
			kv := strings.SplitN(word, "=", 2)
			key := kv[0]
			var val string
			if len(kv) == 2 {
				val = kv[1]
			}
			data[key] = []string{val}
		}
	}
	return Message(Encode(data))
}

func (m Message) Add(k, v string) Message {
	data, err := Decode(string(m))
	if err != nil {
		return m
	}
	if values, exists := data[k]; exists {
		data[k] = append(values, v)
	} else {
		data[k] = []string{v}
	}
	return Message(Encode(data))
}

func (m Message) Set(k string, v ...string) Message {
	data, err := Decode(string(m))
	if err != nil {
		panic(err)
		return m
	}
	data[k] = v
	return Message(Encode(data))
}

func (m Message) Del(k string) Message {
	data, err := Decode(string(m))
	if err != nil {
		panic(err)
		return m
	}
	delete(data, k)
	return Message(Encode(data))
}

func (m Message) Get(k string) []string {
	data, err := Decode(string(m))
	if err != nil {
		return nil
	}
	v, exists := data[k]
	if !exists {
		return nil
	}
	return v
}

func (m Message) Pretty() string {
	data, err := Decode(string(m))
	if err != nil {
		return ""
	}
	entries := make([]string, 0, len(data))
	for k, values := range data {
		entries = append(entries, fmt.Sprintf("%s=%s", k, strings.Join(values, ",")))
	}
	return strings.Join(entries, " ")
}

func (m Message) String() string {
	return string(m)
}

func (m Message) Bytes() []byte {
	return []byte(m)
}
