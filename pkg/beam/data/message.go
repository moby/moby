package data

import (
)

type Message string

func Empty() Message {
	return Message(Encode(nil))
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

func (m Message) String() string {
	return string(m)
}

func (m Message) Bytes() []byte {
	return []byte(m)
}
