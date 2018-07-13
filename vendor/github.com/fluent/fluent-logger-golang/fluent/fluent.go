package fluent

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/tinylib/msgp/msgp"
)

const (
	defaultHost                   = "127.0.0.1"
	defaultNetwork                = "tcp"
	defaultSocketPath             = ""
	defaultPort                   = 24224
	defaultTimeout                = 3 * time.Second
	defaultWriteTimeout           = time.Duration(0) // Write() will not time out
	defaultBufferLimit            = 8 * 1024 * 1024
	defaultRetryWait              = 500
	defaultMaxRetry               = 13
	defaultReconnectWaitIncreRate = 1.5
	// Default sub-second precision value to false since it is only compatible
	// with fluentd versions v0.14 and above.
	defaultSubSecondPrecision = false
)

type Config struct {
	FluentPort       int           `json:"fluent_port"`
	FluentHost       string        `json:"fluent_host"`
	FluentNetwork    string        `json:"fluent_network"`
	FluentSocketPath string        `json:"fluent_socket_path"`
	Timeout          time.Duration `json:"timeout"`
	WriteTimeout     time.Duration `json:"write_timeout"`
	BufferLimit      int           `json:"buffer_limit"`
	RetryWait        int           `json:"retry_wait"`
	MaxRetry         int           `json:"max_retry"`
	TagPrefix        string        `json:"tag_prefix"`
	AsyncConnect     bool          `json:"async_connect"`
	MarshalAsJSON    bool          `json:"marshal_as_json"`

	// Sub-second precision timestamps are only possible for those using fluentd
	// v0.14+ and serializing their messages with msgpack.
	SubSecondPrecision bool `json:"sub_second_precision"`
}

type Fluent struct {
	Config

	mubuff  sync.Mutex
	pending []byte

	muconn       sync.Mutex
	conn         net.Conn
	reconnecting bool
}

// New creates a new Logger.
func New(config Config) (f *Fluent, err error) {
	if config.FluentNetwork == "" {
		config.FluentNetwork = defaultNetwork
	}
	if config.FluentHost == "" {
		config.FluentHost = defaultHost
	}
	if config.FluentPort == 0 {
		config.FluentPort = defaultPort
	}
	if config.FluentSocketPath == "" {
		config.FluentSocketPath = defaultSocketPath
	}
	if config.Timeout == 0 {
		config.Timeout = defaultTimeout
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = defaultWriteTimeout
	}
	if config.BufferLimit == 0 {
		config.BufferLimit = defaultBufferLimit
	}
	if config.RetryWait == 0 {
		config.RetryWait = defaultRetryWait
	}
	if config.MaxRetry == 0 {
		config.MaxRetry = defaultMaxRetry
	}
	if config.AsyncConnect {
		f = &Fluent{Config: config, reconnecting: true}
		go f.reconnect()
	} else {
		f = &Fluent{Config: config, reconnecting: false}
		err = f.connect()
	}
	return
}

// Post writes the output for a logging event.
//
// Examples:
//
//  // send map[string]
//  mapStringData := map[string]string{
//  	"foo":  "bar",
//  }
//  f.Post("tag_name", mapStringData)
//
//  // send message with specified time
//  mapStringData := map[string]string{
//  	"foo":  "bar",
//  }
//  tm := time.Now()
//  f.PostWithTime("tag_name", tm, mapStringData)
//
//  // send struct
//  structData := struct {
//  		Name string `msg:"name"`
//  } {
//  		"john smith",
//  }
//  f.Post("tag_name", structData)
//
func (f *Fluent) Post(tag string, message interface{}) error {
	timeNow := time.Now()
	return f.PostWithTime(tag, timeNow, message)
}

func (f *Fluent) PostWithTime(tag string, tm time.Time, message interface{}) error {
	if len(f.TagPrefix) > 0 {
		tag = f.TagPrefix + "." + tag
	}

	if m, ok := message.(msgp.Marshaler); ok {
		return f.EncodeAndPostData(tag, tm, m)
	}

	msg := reflect.ValueOf(message)
	msgtype := msg.Type()

	if msgtype.Kind() == reflect.Struct {
		// message should be tagged by "codec" or "msg"
		kv := make(map[string]interface{})
		fields := msgtype.NumField()
		for i := 0; i < fields; i++ {
			field := msgtype.Field(i)
			name := field.Name
			if n1 := field.Tag.Get("msg"); n1 != "" {
				name = n1
			} else if n2 := field.Tag.Get("codec"); n2 != "" {
				name = n2
			}
			kv[name] = msg.FieldByIndex(field.Index).Interface()
		}
		return f.EncodeAndPostData(tag, tm, kv)
	}

	if msgtype.Kind() != reflect.Map {
		return errors.New("fluent#PostWithTime: message must be a map")
	} else if msgtype.Key().Kind() != reflect.String {
		return errors.New("fluent#PostWithTime: map keys must be strings")
	}

	kv := make(map[string]interface{})
	for _, k := range msg.MapKeys() {
		kv[k.String()] = msg.MapIndex(k).Interface()
	}

	return f.EncodeAndPostData(tag, tm, kv)
}

func (f *Fluent) EncodeAndPostData(tag string, tm time.Time, message interface{}) error {
	var data []byte
	var err error
	if data, err = f.EncodeData(tag, tm, message); err != nil {
		return fmt.Errorf("fluent#EncodeAndPostData: can't convert '%#v' to msgpack:%v", message, err)
	}
	return f.postRawData(data)
}

// Deprecated: Use EncodeAndPostData instead
func (f *Fluent) PostRawData(data []byte) {
	f.postRawData(data)
}

func (f *Fluent) postRawData(data []byte) error {
	if err := f.appendBuffer(data); err != nil {
		return err
	}
	if err := f.send(); err != nil {
		f.close()
		return err
	}
	return nil
}

// For sending forward protocol adopted JSON
type MessageChunk struct {
	message Message
}

// Golang default marshaler does not support
// ["value", "value2", {"key":"value"}] style marshaling.
// So, it should write JSON marshaler by hand.
func (chunk *MessageChunk) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(chunk.message.Record)
	return []byte(fmt.Sprintf("[\"%s\",%d,%s,null]", chunk.message.Tag,
		chunk.message.Time, data)), err
}

func (f *Fluent) EncodeData(tag string, tm time.Time, message interface{}) (data []byte, err error) {
	timeUnix := tm.Unix()
	if f.Config.MarshalAsJSON {
		msg := Message{Tag: tag, Time: timeUnix, Record: message}
		chunk := &MessageChunk{message: msg}
		data, err = json.Marshal(chunk)
	} else if f.Config.SubSecondPrecision {
		msg := &MessageExt{Tag: tag, Time: EventTime(tm), Record: message}
		data, err = msg.MarshalMsg(nil)
	} else {
		msg := &Message{Tag: tag, Time: timeUnix, Record: message}
		data, err = msg.MarshalMsg(nil)
	}
	return
}

// Close closes the connection.
func (f *Fluent) Close() (err error) {
	if len(f.pending) > 0 {
		err = f.send()
	}
	f.close()
	return
}

// appendBuffer appends data to buffer with lock.
func (f *Fluent) appendBuffer(data []byte) error {
	f.mubuff.Lock()
	defer f.mubuff.Unlock()
	if len(f.pending)+len(data) > f.Config.BufferLimit {
		return errors.New(fmt.Sprintf("fluent#appendBuffer: Buffer full, limit %v", f.Config.BufferLimit))
	}
	f.pending = append(f.pending, data...)
	return nil
}

// close closes the connection.
func (f *Fluent) close() {
	f.muconn.Lock()
	if f.conn != nil {
		f.conn.Close()
		f.conn = nil
	}
	f.muconn.Unlock()
}

// connect establishes a new connection using the specified transport.
func (f *Fluent) connect() (err error) {
	f.muconn.Lock()
	defer f.muconn.Unlock()

	switch f.Config.FluentNetwork {
	case "tcp":
		f.conn, err = net.DialTimeout(f.Config.FluentNetwork, f.Config.FluentHost+":"+strconv.Itoa(f.Config.FluentPort), f.Config.Timeout)
	case "unix":
		f.conn, err = net.DialTimeout(f.Config.FluentNetwork, f.Config.FluentSocketPath, f.Config.Timeout)
	default:
		err = net.UnknownNetworkError(f.Config.FluentNetwork)
	}

	if err == nil {
		f.reconnecting = false
	}
	return
}

func e(x, y float64) int {
	return int(math.Pow(x, y))
}

func (f *Fluent) reconnect() {
	for i := 0; ; i++ {
		err := f.connect()
		if err == nil {
			f.send()
			return
		}
		if i == f.Config.MaxRetry {
			// TODO: What we can do when connection failed MaxRetry times?
			panic("fluent#reconnect: failed to reconnect!")
		}
		waitTime := f.Config.RetryWait * e(defaultReconnectWaitIncreRate, float64(i-1))
		time.Sleep(time.Duration(waitTime) * time.Millisecond)
	}
}

func (f *Fluent) send() error {
	f.muconn.Lock()
	defer f.muconn.Unlock()

	if f.conn == nil {
		if f.reconnecting == false {
			f.reconnecting = true
			go f.reconnect()
		}
		return errors.New("fluent#send: can't send logs, client is reconnecting")
	}

	f.mubuff.Lock()
	defer f.mubuff.Unlock()

	var err error
	if len(f.pending) > 0 {
		t := f.Config.WriteTimeout
		if time.Duration(0) < t {
			f.conn.SetWriteDeadline(time.Now().Add(t))
		} else {
			f.conn.SetWriteDeadline(time.Time{})
		}
		_, err = f.conn.Write(f.pending)
		if err != nil {
			f.conn.Close()
			f.conn = nil
		} else {
			f.pending = f.pending[:0]
		}
	}
	return err
}
