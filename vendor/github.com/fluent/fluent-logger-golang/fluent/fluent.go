package fluent

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"reflect"
	"strconv"
	"sync"
	"time"

	"bytes"
	"encoding/base64"
	"encoding/binary"

	"github.com/tinylib/msgp/msgp"
)

const (
	defaultHost                   = "127.0.0.1"
	defaultNetwork                = "tcp"
	defaultSocketPath             = ""
	defaultPort                   = 24224
	defaultTimeout                = 3 * time.Second
	defaultWriteTimeout           = time.Duration(0) // Write() will not time out
	defaultBufferLimit            = 8 * 1024
	defaultRetryWait              = 500
	defaultMaxRetryWait           = 60000
	defaultMaxRetry               = 13
	defaultReconnectWaitIncreRate = 1.5
	// Default sub-second precision value to false since it is only compatible
	// with fluentd versions v0.14 and above.
	defaultSubSecondPrecision = false

	// Default value whether to skip checking insecure certs on TLS connections.
	defaultTlsInsecureSkipVerify = false
)

// randomGenerator is used by getUniqueId to generate ack hashes. Its value is replaced
// during tests with a deterministic function.
var randomGenerator = rand.Uint64

type Config struct {
	FluentPort          int           `json:"fluent_port"`
	FluentHost          string        `json:"fluent_host"`
	FluentNetwork       string        `json:"fluent_network"`
	FluentSocketPath    string        `json:"fluent_socket_path"`
	Timeout             time.Duration `json:"timeout"`
	WriteTimeout        time.Duration `json:"write_timeout"`
	BufferLimit         int           `json:"buffer_limit"`
	RetryWait           int           `json:"retry_wait"`
	MaxRetry            int           `json:"max_retry"`
	MaxRetryWait        int           `json:"max_retry_wait"`
	TagPrefix           string        `json:"tag_prefix"`
	Async               bool          `json:"async"`
	ForceStopAsyncSend  bool          `json:"force_stop_async_send"`
	AsyncResultCallback func(data []byte, err error)
	// Deprecated: Use Async instead
	AsyncConnect  bool `json:"async_connect"`
	MarshalAsJSON bool `json:"marshal_as_json"`

	// AsyncReconnectInterval defines the interval (ms) at which the connection
	// to the fluentd-address is re-established. This option is useful if the address
	// may resolve to one or more IP addresses, e.g. a Consul service address.
	AsyncReconnectInterval int `json:"async_reconnect_interval"`

	// Sub-second precision timestamps are only possible for those using fluentd
	// v0.14+ and serializing their messages with msgpack.
	SubSecondPrecision bool `json:"sub_second_precision"`

	// RequestAck sends the chunk option with a unique ID. The server will
	// respond with an acknowledgement. This option improves the reliability
	// of the message transmission.
	RequestAck bool `json:"request_ack"`

	// Flag to skip verifying insecure certs on TLS connections
	TlsInsecureSkipVerify bool `json: "tls_insecure_skip_verify"`
}

type ErrUnknownNetwork struct {
	network string
}

func (e *ErrUnknownNetwork) Error() string {
	return "unknown network " + e.network
}

func NewErrUnknownNetwork(network string) error {
	return &ErrUnknownNetwork{network}
}

type msgToSend struct {
	data []byte
	ack  string
}

type Fluent struct {
	Config

	dialer dialer
	// stopRunning is used in async mode to signal to run() it should abort.
	stopRunning chan struct{}
	// cancelDialings is used by Close() to stop any in-progress dialing.
	cancelDialings context.CancelFunc
	pending        chan *msgToSend
	pendingMutex   sync.RWMutex
	closed         bool
	wg             sync.WaitGroup

	// time at which the most recent connection to fluentd-address was established.
	latestReconnectTime time.Time

	muconn sync.RWMutex
	conn   net.Conn
}

type dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// New creates a new Logger.
func New(config Config) (*Fluent, error) {
	if config.Timeout == 0 {
		config.Timeout = defaultTimeout
	}
	return newWithDialer(config, &net.Dialer{
		Timeout: config.Timeout,
	})
}

func newWithDialer(config Config, d dialer) (f *Fluent, err error) {
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
	if config.MaxRetryWait == 0 {
		config.MaxRetryWait = defaultMaxRetryWait
	}
	if !config.TlsInsecureSkipVerify {
		config.TlsInsecureSkipVerify = defaultTlsInsecureSkipVerify
	}
	if config.AsyncConnect {
		fmt.Fprintf(os.Stderr, "fluent#New: AsyncConnect is now deprecated, please use Async instead")
		config.Async = config.Async || config.AsyncConnect
	}

	if config.Async {
		ctx, cancel := context.WithCancel(context.Background())

		f = &Fluent{
			Config:         config,
			dialer:         d,
			stopRunning:    make(chan struct{}),
			cancelDialings: cancel,
			pending:        make(chan *msgToSend, config.BufferLimit),
			pendingMutex:   sync.RWMutex{},
			muconn:         sync.RWMutex{},
		}

		f.wg.Add(1)
		go f.run(ctx)
	} else {
		f = &Fluent{
			Config: config,
			dialer: d,
			muconn: sync.RWMutex{},
		}
		err = f.connect(context.Background())
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
			value := msg.FieldByIndex(field.Index)
			// ignore unexported fields
			if !value.CanInterface() {
				continue
			}
			name := field.Name
			if n1 := field.Tag.Get("msg"); n1 != "" {
				name = n1
			} else if n2 := field.Tag.Get("codec"); n2 != "" {
				name = n2
			}
			kv[name] = value.Interface()
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
	var msg *msgToSend
	var err error
	if msg, err = f.EncodeData(tag, tm, message); err != nil {
		return fmt.Errorf("fluent#EncodeAndPostData: can't convert '%#v' to msgpack:%v", message, err)
	}
	return f.postRawData(msg)
}

// Deprecated: Use EncodeAndPostData instead
func (f *Fluent) PostRawData(msg *msgToSend) {
	f.postRawData(msg)
}

func (f *Fluent) postRawData(msg *msgToSend) error {
	if f.Config.Async {
		return f.appendBuffer(msg)
	}

	// Synchronous write
	if f.closed {
		return fmt.Errorf("fluent#postRawData: Logger already closed")
	}
	return f.writeWithRetry(context.Background(), msg)
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
	if err != nil {
		return nil, err
	}
	option, err := json.Marshal(chunk.message.Option)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("[\"%s\",%d,%s,%s]", chunk.message.Tag,
		chunk.message.Time, data, option)), err
}

// getUniqueID returns a base64 encoded unique ID that can be used for chunk/ack
// mechanism, see
// https://github.com/fluent/fluentd/wiki/Forward-Protocol-Specification-v1#option
func getUniqueID(timeUnix int64) (string, error) {
	buf := bytes.NewBuffer(nil)
	enc := base64.NewEncoder(base64.StdEncoding, buf)
	if err := binary.Write(enc, binary.LittleEndian, timeUnix); err != nil {
		enc.Close()
		return "", err
	}
	if err := binary.Write(enc, binary.LittleEndian, randomGenerator()); err != nil {
		enc.Close()
		return "", err
	}
	// encoder needs to be closed before buf.String(), defer does not work
	// here
	enc.Close()
	return buf.String(), nil
}

func (f *Fluent) EncodeData(tag string, tm time.Time, message interface{}) (msg *msgToSend, err error) {
	option := make(map[string]string)
	msg = &msgToSend{}
	timeUnix := tm.Unix()
	if f.Config.RequestAck {
		var err error
		msg.ack, err = getUniqueID(timeUnix)
		if err != nil {
			return nil, err
		}
		option["chunk"] = msg.ack
	}
	if f.Config.MarshalAsJSON {
		m := Message{Tag: tag, Time: timeUnix, Record: message, Option: option}
		chunk := &MessageChunk{message: m}
		msg.data, err = json.Marshal(chunk)
	} else if f.Config.SubSecondPrecision {
		m := &MessageExt{Tag: tag, Time: EventTime(tm), Record: message, Option: option}
		msg.data, err = m.MarshalMsg(nil)
	} else {
		m := &Message{Tag: tag, Time: timeUnix, Record: message, Option: option}
		msg.data, err = m.MarshalMsg(nil)
	}
	return
}

// Close closes the connection, waiting for pending logs to be sent. If the client is
// running in async mode, the run() goroutine exits before Close() returns.
func (f *Fluent) Close() (err error) {
	if f.Config.Async {
		f.pendingMutex.Lock()
		if f.closed {
			f.pendingMutex.Unlock()
			return nil
		}
		f.closed = true
		f.pendingMutex.Unlock()

		if f.Config.ForceStopAsyncSend {
			close(f.stopRunning)
			f.cancelDialings()
		}

		close(f.pending)
		// If ForceStopAsyncSend is false, all logs in the channel have to be sent
		// before closing the connection. At this point closed is true so no more
		// logs are written to the channel and f.pending has been closed, so run()
		// goroutine will exit as soon as all logs in the channel are sent.
		if !f.Config.ForceStopAsyncSend {
			f.wg.Wait()
		}
	}

	f.muconn.Lock()
	f.close()
	f.closed = true
	f.muconn.Unlock()

	// If ForceStopAsyncSend is true, we shall close the connection before waiting for
	// run() goroutine to exit to be sure we aren't waiting on ack message that might
	// never come (eg. because fluentd server is down). However we want to be sure the
	// run() goroutine stops before returning from Close().
	if f.Config.ForceStopAsyncSend {
		f.wg.Wait()
	}
	return
}

// appendBuffer appends data to buffer with lock.
func (f *Fluent) appendBuffer(msg *msgToSend) error {
	f.pendingMutex.RLock()
	defer f.pendingMutex.RUnlock()
	if f.closed {
		return fmt.Errorf("fluent#appendBuffer: Logger already closed")
	}
	select {
	case f.pending <- msg:
	default:
		return fmt.Errorf("fluent#appendBuffer: Buffer full, limit %v", f.Config.BufferLimit)
	}
	return nil
}

// close closes the connection. Callers should take care of locking muconn first.
func (f *Fluent) close() {
	if f.conn != nil {
		f.conn.Close()
		f.conn = nil
	}
}

// connect establishes a new connection using the specified transport. Caller should
// take care of locking muconn first.
func (f *Fluent) connect(ctx context.Context) (err error) {
	switch f.Config.FluentNetwork {
	case "tcp":
		f.conn, err = f.dialer.DialContext(ctx,
			f.Config.FluentNetwork,
			f.Config.FluentHost+":"+strconv.Itoa(f.Config.FluentPort))
	case "tls":
		tlsConfig := &tls.Config{InsecureSkipVerify: f.Config.TlsInsecureSkipVerify}
		f.conn, err = tls.DialWithDialer(
			&net.Dialer{Timeout: f.Config.Timeout},
			"tcp",
			f.Config.FluentHost+":"+strconv.Itoa(f.Config.FluentPort), tlsConfig,
		)
	case "unix":
		f.conn, err = f.dialer.DialContext(ctx,
			f.Config.FluentNetwork,
			f.Config.FluentSocketPath)
	default:
		err = NewErrUnknownNetwork(f.Config.FluentNetwork)
	}

	if err == nil {
		f.latestReconnectTime = time.Now()
	}

	return err
}

var errIsClosing = errors.New("fluent logger is closing")

// Caller should take care of locking muconn first.
func (f *Fluent) connectWithRetry(ctx context.Context) error {
	// A Time channel is used instead of time.Sleep() to avoid blocking this
	// goroutine during way too much time (because of the exponential back-off
	// retry).
	// time.NewTimer() is used instead of time.After() to avoid leaking the
	// timer channel (cf. https://pkg.go.dev/time#After).
	timeout := time.NewTimer(time.Duration(0))
	defer func() {
		// timeout.Stop() is called in a function literal instead of being
		// defered directly as it's re-assigned below when the retry loop spins.
		timeout.Stop()
	}()

	for i := 0; i < f.Config.MaxRetry; i++ {
		select {
		case <-timeout.C:
			err := f.connect(ctx)
			if err == nil {
				return nil
			}

			if _, ok := err.(*ErrUnknownNetwork); ok {
				return err
			}
			if err == context.Canceled {
				return errIsClosing
			}

			waitTime := f.Config.RetryWait * e(defaultReconnectWaitIncreRate, float64(i-1))
			if waitTime > f.Config.MaxRetryWait {
				waitTime = f.Config.MaxRetryWait
			}

			timeout = time.NewTimer(time.Duration(waitTime) * time.Millisecond)
		case <-ctx.Done():
			return errIsClosing
		}
	}

	return fmt.Errorf("could not connect to fluentd after %d retries", f.Config.MaxRetry)
}

// run is the goroutine used to unqueue and write logs in async mode. That
// goroutine is meant to run during the whole life of the Fluent logger.
func (f *Fluent) run(ctx context.Context) {
	for {
		select {
		case entry, ok := <-f.pending:
			// f.stopRunning is closed before f.pending only when ForceStopAsyncSend
			// is enabled. Otherwise, f.pending is closed when Close() is called.
			if !ok {
				f.wg.Done()
				return
			}

			if f.AsyncReconnectInterval > 0 {
				if time.Since(f.latestReconnectTime) > time.Duration(f.AsyncReconnectInterval)*time.Millisecond {
					f.muconn.Lock()
					f.close()
					f.connectWithRetry(ctx)
					f.muconn.Unlock()
				}
			}

			err := f.writeWithRetry(ctx, entry)
			if err != nil && err != errIsClosing {
				fmt.Fprintf(os.Stderr, "[%s] Unable to send logs to fluentd, reconnecting...\n", time.Now().Format(time.RFC3339))
			}
			if f.AsyncResultCallback != nil {
				var data []byte
				if entry != nil {
					data = entry.data
				}
				f.AsyncResultCallback(data, err)
			}
		case <-f.stopRunning:
			fmt.Fprintf(os.Stderr, "[%s] Discarding queued events...\n", time.Now().Format(time.RFC3339))

			f.wg.Done()
			return
		}
	}
}

func e(x, y float64) int {
	return int(math.Pow(x, y))
}

func (f *Fluent) writeWithRetry(ctx context.Context, msg *msgToSend) error {
	for i := 0; i < f.Config.MaxRetry; i++ {
		if retry, err := f.write(ctx, msg); !retry {
			return err
		}
	}

	return fmt.Errorf("fluent#write: failed to write after %d attempts", f.Config.MaxRetry)
}

// write writes the provided msg to fluentd server. Its first return values is
// a bool indicating whether the write should be retried.
// This method relies on function literals to execute muconn.Unlock or
// muconn.RUnlock in deferred calls to ensure the mutex is unlocked even in
// the case of panic recovering.
func (f *Fluent) write(ctx context.Context, msg *msgToSend) (bool, error) {
	closer := func() {
		f.muconn.Lock()
		defer f.muconn.Unlock()

		f.close()
	}

	if err := func() (err error) {
		f.muconn.Lock()
		defer f.muconn.Unlock()

		if f.conn == nil {
			err = f.connectWithRetry(ctx)
		}

		return err
	}(); err != nil {
		// Here, we don't want to retry the write since connectWithRetry already
		// retries Config.MaxRetry times to connect.
		return false, fmt.Errorf("fluent#write: %v", err)
	}

	if err := func() (err error) {
		f.muconn.RLock()
		defer f.muconn.RUnlock()

		if f.conn == nil {
			return fmt.Errorf("connection has been closed before writing to it")
		}

		t := f.Config.WriteTimeout
		if time.Duration(0) < t {
			f.conn.SetWriteDeadline(time.Now().Add(t))
		} else {
			f.conn.SetWriteDeadline(time.Time{})
		}

		_, err = f.conn.Write(msg.data)
		return err
	}(); err != nil {
		closer()
		return true, fmt.Errorf("fluent#write: %v", err)
	}

	// Acknowledgment check
	if msg.ack != "" {
		resp := &AckResp{}
		var err error
		if f.Config.MarshalAsJSON {
			dec := json.NewDecoder(f.conn)
			err = dec.Decode(resp)
		} else {
			r := msgp.NewReader(f.conn)
			err = resp.DecodeMsg(r)
		}

		if err != nil || resp.Ack != msg.ack {
			fmt.Fprintf(os.Stderr, "fluent#write: message ack (%s) doesn't match expected one (%s). Closing connection...", resp.Ack, msg.ack)

			closer()
			return true, err
		}
	}

	return false, nil
}
