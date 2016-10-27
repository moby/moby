// Copyright 2012-2016 Apcera Inc. All rights reserved.

// A Go client for the NATS messaging system (https://nats.io).
package nats

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/url"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats/util"
	"github.com/nats-io/nuid"
)

// Default Constants
const (
	Version                 = "1.2.2"
	DefaultURL              = "nats://localhost:4222"
	DefaultPort             = 4222
	DefaultMaxReconnect     = 60
	DefaultReconnectWait    = 2 * time.Second
	DefaultTimeout          = 2 * time.Second
	DefaultPingInterval     = 2 * time.Minute
	DefaultMaxPingOut       = 2
	DefaultMaxChanLen       = 8192            // 8k
	DefaultReconnectBufSize = 8 * 1024 * 1024 // 8MB
	RequestChanLen          = 8
	LangString              = "go"
)

// STALE_CONNECTION is for detection and proper handling of stale connections.
const STALE_CONNECTION = "stale connection"

// PERMISSIONS_ERR is for when nats server subject authorization has failed.
const PERMISSIONS_ERR = "permissions violation"

// Errors
var (
	ErrConnectionClosed     = errors.New("nats: connection closed")
	ErrSecureConnRequired   = errors.New("nats: secure connection required")
	ErrSecureConnWanted     = errors.New("nats: secure connection not available")
	ErrBadSubscription      = errors.New("nats: invalid subscription")
	ErrTypeSubscription     = errors.New("nats: invalid subscription type")
	ErrBadSubject           = errors.New("nats: invalid subject")
	ErrSlowConsumer         = errors.New("nats: slow consumer, messages dropped")
	ErrTimeout              = errors.New("nats: timeout")
	ErrBadTimeout           = errors.New("nats: timeout invalid")
	ErrAuthorization        = errors.New("nats: authorization violation")
	ErrNoServers            = errors.New("nats: no servers available for connection")
	ErrJsonParse            = errors.New("nats: connect message, json parse error")
	ErrChanArg              = errors.New("nats: argument needs to be a channel type")
	ErrMaxPayload           = errors.New("nats: maximum payload exceeded")
	ErrMaxMessages          = errors.New("nats: maximum messages delivered")
	ErrSyncSubRequired      = errors.New("nats: illegal call on an async subscription")
	ErrMultipleTLSConfigs   = errors.New("nats: multiple tls.Configs not allowed")
	ErrNoInfoReceived       = errors.New("nats: protocol exception, INFO not received")
	ErrReconnectBufExceeded = errors.New("nats: outbound buffer limit exceeded")
	ErrInvalidConnection    = errors.New("nats: invalid connection")
	ErrInvalidMsg           = errors.New("nats: invalid message or message nil")
	ErrInvalidArg           = errors.New("nats: invalid argument")
	ErrStaleConnection      = errors.New("nats: " + STALE_CONNECTION)
)

var DefaultOptions = Options{
	AllowReconnect:   true,
	MaxReconnect:     DefaultMaxReconnect,
	ReconnectWait:    DefaultReconnectWait,
	Timeout:          DefaultTimeout,
	PingInterval:     DefaultPingInterval,
	MaxPingsOut:      DefaultMaxPingOut,
	SubChanLen:       DefaultMaxChanLen,
	ReconnectBufSize: DefaultReconnectBufSize,
	Dialer: &net.Dialer{
		Timeout: DefaultTimeout,
	},
}

// Status represents the state of the connection.
type Status int

const (
	DISCONNECTED = Status(iota)
	CONNECTED
	CLOSED
	RECONNECTING
	CONNECTING
)

// ConnHandler is used for asynchronous events such as
// disconnected and closed connections.
type ConnHandler func(*Conn)

// ErrHandler is used to process asynchronous errors encountered
// while processing inbound messages.
type ErrHandler func(*Conn, *Subscription, error)

// asyncCB is used to preserve order for async callbacks.
type asyncCB func()

// Option is a function on the options for a connection.
type Option func(*Options) error

// Options can be used to create a customized connection.
type Options struct {
	Url            string
	Servers        []string
	NoRandomize    bool
	Name           string
	Verbose        bool
	Pedantic       bool
	Secure         bool
	TLSConfig      *tls.Config
	AllowReconnect bool
	MaxReconnect   int
	ReconnectWait  time.Duration
	Timeout        time.Duration
	PingInterval   time.Duration // disabled if 0 or negative
	MaxPingsOut    int
	ClosedCB       ConnHandler
	DisconnectedCB ConnHandler
	ReconnectedCB  ConnHandler
	AsyncErrorCB   ErrHandler

	// Size of the backing bufio buffer during reconnect. Once this
	// has been exhausted publish operations will error.
	ReconnectBufSize int

	// The size of the buffered channel used between the socket
	// Go routine and the message delivery for SyncSubscriptions.
	// NOTE: This does not affect AsyncSubscriptions which are
	// dictated by PendingLimits()
	SubChanLen int

	User     string
	Password string
	Token    string

	// Dialer allows users setting a custom Dialer
	Dialer *net.Dialer
}

const (
	// Scratch storage for assembling protocol headers
	scratchSize = 512

	// The size of the bufio reader/writer on top of the socket.
	defaultBufSize = 32768

	// The buffered size of the flush "kick" channel
	flushChanSize = 1024

	// Default server pool size
	srvPoolSize = 4

	// Channel size for the async callback handler.
	asyncCBChanSize = 32
)

// A Conn represents a bare connection to a nats-server.
// It can send and receive []byte payloads.
type Conn struct {
	// Keep all members for which we use atomic at the beginning of the
	// struct and make sure they are all 64bits (or use padding if necessary).
	// atomic.* functions crash on 32bit machines if operand is not aligned
	// at 64bit. See https://github.com/golang/go/issues/599
	ssid int64

	Statistics
	mu      sync.Mutex
	Opts    Options
	wg      sync.WaitGroup
	url     *url.URL
	conn    net.Conn
	srvPool []*srv
	urls    map[string]struct{} // Keep track of all known URLs (used by processInfo)
	bw      *bufio.Writer
	pending *bytes.Buffer
	fch     chan bool
	info    serverInfo
	subs    map[int64]*Subscription
	mch     chan *Msg
	ach     chan asyncCB
	pongs   []chan bool
	scratch [scratchSize]byte
	status  Status
	err     error
	ps      *parseState
	ptmr    *time.Timer
	pout    int
}

// A Subscription represents interest in a given subject.
type Subscription struct {
	mu  sync.Mutex
	sid int64

	// Subject that represents this subscription. This can be different
	// than the received subject inside a Msg if this is a wildcard.
	Subject string

	// Optional queue group name. If present, all subscriptions with the
	// same name will form a distributed queue, and each message will
	// only be processed by one member of the group.
	Queue string

	delivered  uint64
	max        uint64
	conn       *Conn
	mcb        MsgHandler
	mch        chan *Msg
	closed     bool
	sc         bool
	connClosed bool

	// Type of Subscription
	typ SubscriptionType

	// Async linked list
	pHead *Msg
	pTail *Msg
	pCond *sync.Cond

	// Pending stats, async subscriptions, high-speed etc.
	pMsgs       int
	pBytes      int
	pMsgsMax    int
	pBytesMax   int
	pMsgsLimit  int
	pBytesLimit int
	dropped     int
}

// Msg is a structure used by Subscribers and PublishMsg().
type Msg struct {
	Subject string
	Reply   string
	Data    []byte
	Sub     *Subscription
	next    *Msg
}

// Tracks various stats received and sent on this connection,
// including counts for messages and bytes.
type Statistics struct {
	InMsgs     uint64
	OutMsgs    uint64
	InBytes    uint64
	OutBytes   uint64
	Reconnects uint64
}

// Tracks individual backend servers.
type srv struct {
	url         *url.URL
	didConnect  bool
	reconnects  int
	lastAttempt time.Time
	isImplicit  bool
}

type serverInfo struct {
	Id           string   `json:"server_id"`
	Host         string   `json:"host"`
	Port         uint     `json:"port"`
	Version      string   `json:"version"`
	AuthRequired bool     `json:"auth_required"`
	TLSRequired  bool     `json:"tls_required"`
	MaxPayload   int64    `json:"max_payload"`
	ConnectURLs  []string `json:"connect_urls,omitempty"`
}

const (
	// clientProtoZero is the original client protocol from 2009.
	// http://nats.io/documentation/internals/nats-protocol/
	clientProtoZero = iota
	// clientProtoInfo signals a client can receive more then the original INFO block.
	// This can be used to update clients on other cluster members, etc.
	clientProtoInfo
)

type connectInfo struct {
	Verbose  bool   `json:"verbose"`
	Pedantic bool   `json:"pedantic"`
	User     string `json:"user,omitempty"`
	Pass     string `json:"pass,omitempty"`
	Token    string `json:"auth_token,omitempty"`
	TLS      bool   `json:"tls_required"`
	Name     string `json:"name"`
	Lang     string `json:"lang"`
	Version  string `json:"version"`
	Protocol int    `json:"protocol"`
}

// MsgHandler is a callback function that processes messages delivered to
// asynchronous subscribers.
type MsgHandler func(msg *Msg)

// Connect will attempt to connect to the NATS system.
// The url can contain username/password semantics. e.g. nats://derek:pass@localhost:4222
// Comma separated arrays are also supported, e.g. urlA, urlB.
// Options start with the defaults but can be overridden.
func Connect(url string, options ...Option) (*Conn, error) {
	opts := DefaultOptions
	opts.Servers = processUrlString(url)
	for _, opt := range options {
		if err := opt(&opts); err != nil {
			return nil, err
		}
	}
	return opts.Connect()
}

// Options that can be passed to Connect.

// Name is an Option to set the client name.
func Name(name string) Option {
	return func(o *Options) error {
		o.Name = name
		return nil
	}
}

// Secure is an Option to enable TLS secure connections that skip server verification by default.
// Pass a TLS Configuration for proper TLS.
func Secure(tls ...*tls.Config) Option {
	return func(o *Options) error {
		o.Secure = true
		// Use of variadic just simplifies testing scenarios. We only take the first one.
		// fixme(DLC) - Could panic if more than one. Could also do TLS option.
		if len(tls) > 1 {
			return ErrMultipleTLSConfigs
		}
		if len(tls) == 1 {
			o.TLSConfig = tls[0]
		}
		return nil
	}
}

// RootCAs is a helper option to provide the RootCAs pool from a list of filenames. If Secure is
// not already set this will set it as well.
func RootCAs(file ...string) Option {
	return func(o *Options) error {
		pool := x509.NewCertPool()
		for _, f := range file {
			rootPEM, err := ioutil.ReadFile(f)
			if err != nil || rootPEM == nil {
				return fmt.Errorf("nats: error loading or parsing rootCA file: %v", err)
			}
			ok := pool.AppendCertsFromPEM([]byte(rootPEM))
			if !ok {
				return fmt.Errorf("nats: failed to parse root certificate from %q", f)
			}
		}
		if o.TLSConfig == nil {
			o.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		o.TLSConfig.RootCAs = pool
		o.Secure = true
		return nil
	}
}

// ClientCert is a helper option to provide the client certificate from a file. If Secure is
// not already set this will set it as well
func ClientCert(certFile, keyFile string) Option {
	return func(o *Options) error {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return fmt.Errorf("nats: error loading client certificate: %v", err)
		}
		cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return fmt.Errorf("nats: error parsing client certificate: %v", err)
		}
		if o.TLSConfig == nil {
			o.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		o.TLSConfig.Certificates = []tls.Certificate{cert}
		o.Secure = true
		return nil
	}
}

// NoReconnect is an Option to turn off reconnect behavior.
func NoReconnect() Option {
	return func(o *Options) error {
		o.AllowReconnect = false
		return nil
	}
}

// DontRandomize is an Option to turn off randomizing the server pool.
func DontRandomize() Option {
	return func(o *Options) error {
		o.NoRandomize = true
		return nil
	}
}

// ReconnectWait is an Option to set the wait time between reconnect attempts.
func ReconnectWait(t time.Duration) Option {
	return func(o *Options) error {
		o.ReconnectWait = t
		return nil
	}
}

// MaxReconnects is an Option to set the maximum number of reconnect attempts.
func MaxReconnects(max int) Option {
	return func(o *Options) error {
		o.MaxReconnect = max
		return nil
	}
}

// Timeout is an Option to set the timeout for Dial on a connection.
func Timeout(t time.Duration) Option {
	return func(o *Options) error {
		o.Timeout = t
		return nil
	}
}

// DisconnectHandler is an Option to set the disconnected handler.
func DisconnectHandler(cb ConnHandler) Option {
	return func(o *Options) error {
		o.DisconnectedCB = cb
		return nil
	}
}

// ReconnectHandler is an Option to set the reconnected handler.
func ReconnectHandler(cb ConnHandler) Option {
	return func(o *Options) error {
		o.ReconnectedCB = cb
		return nil
	}
}

// ClosedHandler is an Option to set the closed handler.
func ClosedHandler(cb ConnHandler) Option {
	return func(o *Options) error {
		o.ClosedCB = cb
		return nil
	}
}

// ErrHandler is an Option to set the async error  handler.
func ErrorHandler(cb ErrHandler) Option {
	return func(o *Options) error {
		o.AsyncErrorCB = cb
		return nil
	}
}

// UserInfo is an Option to set the username and password to
// use when not included directly in the URLs.
func UserInfo(user, password string) Option {
	return func(o *Options) error {
		o.User = user
		o.Password = password
		return nil
	}
}

// Token is an Option to set the token to use when not included
// directly in the URLs.
func Token(token string) Option {
	return func(o *Options) error {
		o.Token = token
		return nil
	}
}

// Dialer is an Option to set the dialer which will be used when
// attempting to establish a connection.
func Dialer(dialer *net.Dialer) Option {
	return func(o *Options) error {
		o.Dialer = dialer
		return nil
	}
}

// Handler processing

// SetDisconnectHandler will set the disconnect event handler.
func (nc *Conn) SetDisconnectHandler(dcb ConnHandler) {
	if nc == nil {
		return
	}
	nc.mu.Lock()
	defer nc.mu.Unlock()
	nc.Opts.DisconnectedCB = dcb
}

// SetReconnectHandler will set the reconnect event handler.
func (nc *Conn) SetReconnectHandler(rcb ConnHandler) {
	if nc == nil {
		return
	}
	nc.mu.Lock()
	defer nc.mu.Unlock()
	nc.Opts.ReconnectedCB = rcb
}

// SetClosedHandler will set the reconnect event handler.
func (nc *Conn) SetClosedHandler(cb ConnHandler) {
	if nc == nil {
		return
	}
	nc.mu.Lock()
	defer nc.mu.Unlock()
	nc.Opts.ClosedCB = cb
}

// SetErrHandler will set the async error handler.
func (nc *Conn) SetErrorHandler(cb ErrHandler) {
	if nc == nil {
		return
	}
	nc.mu.Lock()
	defer nc.mu.Unlock()
	nc.Opts.AsyncErrorCB = cb
}

// Process the url string argument to Connect. Return an array of
// urls, even if only one.
func processUrlString(url string) []string {
	urls := strings.Split(url, ",")
	for i, s := range urls {
		urls[i] = strings.TrimSpace(s)
	}
	return urls
}

// Connect will attempt to connect to a NATS server with multiple options.
func (o Options) Connect() (*Conn, error) {
	nc := &Conn{Opts: o}

	// Some default options processing.
	if nc.Opts.MaxPingsOut == 0 {
		nc.Opts.MaxPingsOut = DefaultMaxPingOut
	}
	// Allow old default for channel length to work correctly.
	if nc.Opts.SubChanLen == 0 {
		nc.Opts.SubChanLen = DefaultMaxChanLen
	}
	// Default ReconnectBufSize
	if nc.Opts.ReconnectBufSize == 0 {
		nc.Opts.ReconnectBufSize = DefaultReconnectBufSize
	}
	// Ensure that Timeout is not 0
	if nc.Opts.Timeout == 0 {
		nc.Opts.Timeout = DefaultTimeout
	}

	// Allow custom Dialer for connecting using DialTimeout by default
	if nc.Opts.Dialer == nil {
		nc.Opts.Dialer = &net.Dialer{
			Timeout: nc.Opts.Timeout,
		}
	}

	if err := nc.setupServerPool(); err != nil {
		return nil, err
	}

	// Create the async callback channel.
	nc.ach = make(chan asyncCB, asyncCBChanSize)

	if err := nc.connect(); err != nil {
		return nil, err
	}

	// Spin up the async cb dispatcher on success
	go nc.asyncDispatch()

	return nc, nil
}

const (
	_CRLF_  = "\r\n"
	_EMPTY_ = ""
	_SPC_   = " "
	_PUB_P_ = "PUB "
)

const (
	_OK_OP_   = "+OK"
	_ERR_OP_  = "-ERR"
	_MSG_OP_  = "MSG"
	_PING_OP_ = "PING"
	_PONG_OP_ = "PONG"
	_INFO_OP_ = "INFO"
)

const (
	conProto   = "CONNECT %s" + _CRLF_
	pingProto  = "PING" + _CRLF_
	pongProto  = "PONG" + _CRLF_
	pubProto   = "PUB %s %s %d" + _CRLF_
	subProto   = "SUB %s %s %d" + _CRLF_
	unsubProto = "UNSUB %d %s" + _CRLF_
	okProto    = _OK_OP_ + _CRLF_
)

// Return the currently selected server
func (nc *Conn) currentServer() (int, *srv) {
	for i, s := range nc.srvPool {
		if s == nil {
			continue
		}
		if s.url == nc.url {
			return i, s
		}
	}
	return -1, nil
}

// Pop the current server and put onto the end of the list. Select head of list as long
// as number of reconnect attempts under MaxReconnect.
func (nc *Conn) selectNextServer() (*srv, error) {
	i, s := nc.currentServer()
	if i < 0 {
		return nil, ErrNoServers
	}
	sp := nc.srvPool
	num := len(sp)
	copy(sp[i:num-1], sp[i+1:num])
	maxReconnect := nc.Opts.MaxReconnect
	if maxReconnect < 0 || s.reconnects < maxReconnect {
		nc.srvPool[num-1] = s
	} else {
		nc.srvPool = sp[0 : num-1]
	}
	if len(nc.srvPool) <= 0 {
		nc.url = nil
		return nil, ErrNoServers
	}
	nc.url = nc.srvPool[0].url
	return nc.srvPool[0], nil
}

// Will assign the correct server to the nc.Url
func (nc *Conn) pickServer() error {
	nc.url = nil
	if len(nc.srvPool) <= 0 {
		return ErrNoServers
	}
	for _, s := range nc.srvPool {
		if s != nil {
			nc.url = s.url
			return nil
		}
	}
	return ErrNoServers
}

const tlsScheme = "tls"

// Create the server pool using the options given.
// We will place a Url option first, followed by any
// Server Options. We will randomize the server pool unlesss
// the NoRandomize flag is set.
func (nc *Conn) setupServerPool() error {
	nc.srvPool = make([]*srv, 0, srvPoolSize)
	nc.urls = make(map[string]struct{}, srvPoolSize)

	// Create srv objects from each url string in nc.Opts.Servers
	// and add them to the pool
	for _, urlString := range nc.Opts.Servers {
		if err := nc.addURLToPool(urlString, false); err != nil {
			return err
		}
	}

	// Randomize if allowed to
	if !nc.Opts.NoRandomize {
		nc.shufflePool()
	}

	// Normally, if this one is set, Options.Servers should not be,
	// but we always allowed that, so continue to do so.
	if nc.Opts.Url != _EMPTY_ {
		// Add to the end of the array
		if err := nc.addURLToPool(nc.Opts.Url, false); err != nil {
			return err
		}
		// Then swap it with first to guarantee that Options.Url is tried first.
		last := len(nc.srvPool) - 1
		if last > 0 {
			nc.srvPool[0], nc.srvPool[last] = nc.srvPool[last], nc.srvPool[0]
		}
	} else if len(nc.srvPool) <= 0 {
		// Place default URL if pool is empty.
		if err := nc.addURLToPool(DefaultURL, false); err != nil {
			return err
		}
	}

	// Check for Scheme hint to move to TLS mode.
	for _, srv := range nc.srvPool {
		if srv.url.Scheme == tlsScheme {
			// FIXME(dlc), this is for all in the pool, should be case by case.
			nc.Opts.Secure = true
			if nc.Opts.TLSConfig == nil {
				nc.Opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
			}
		}
	}

	return nc.pickServer()
}

// addURLToPool adds an entry to the server pool
func (nc *Conn) addURLToPool(sURL string, implicit bool) error {
	u, err := url.Parse(sURL)
	if err != nil {
		return err
	}
	s := &srv{url: u, isImplicit: implicit}
	nc.srvPool = append(nc.srvPool, s)
	nc.urls[u.Host] = struct{}{}
	return nil
}

// shufflePool swaps randomly elements in the server pool
func (nc *Conn) shufflePool() {
	if len(nc.srvPool) <= 1 {
		return
	}
	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)
	for i := range nc.srvPool {
		j := r.Intn(i + 1)
		nc.srvPool[i], nc.srvPool[j] = nc.srvPool[j], nc.srvPool[i]
	}
}

// createConn will connect to the server and wrap the appropriate
// bufio structures. It will do the right thing when an existing
// connection is in place.
func (nc *Conn) createConn() (err error) {
	if nc.Opts.Timeout < 0 {
		return ErrBadTimeout
	}
	if _, cur := nc.currentServer(); cur == nil {
		return ErrNoServers
	} else {
		cur.lastAttempt = time.Now()
	}

	dialer := nc.Opts.Dialer
	nc.conn, err = dialer.Dial("tcp", nc.url.Host)
	if err != nil {
		return err
	}

	// No clue why, but this stalls and kills performance on Mac (Mavericks).
	// https://code.google.com/p/go/issues/detail?id=6930
	//if ip, ok := nc.conn.(*net.TCPConn); ok {
	//	ip.SetReadBuffer(defaultBufSize)
	//}

	if nc.pending != nil && nc.bw != nil {
		// Move to pending buffer.
		nc.bw.Flush()
	}
	nc.bw = bufio.NewWriterSize(nc.conn, defaultBufSize)
	return nil
}

// makeTLSConn will wrap an existing Conn using TLS
func (nc *Conn) makeTLSConn() {
	// Allow the user to configure their own tls.Config structure, otherwise
	// default to InsecureSkipVerify.
	// TODO(dlc) - We should make the more secure version the default.
	if nc.Opts.TLSConfig != nil {
		tlsCopy := util.CloneTLSConfig(nc.Opts.TLSConfig)
		// If its blank we will override it with the current host
		if tlsCopy.ServerName == _EMPTY_ {
			h, _, _ := net.SplitHostPort(nc.url.Host)
			tlsCopy.ServerName = h
		}
		nc.conn = tls.Client(nc.conn, tlsCopy)
	} else {
		nc.conn = tls.Client(nc.conn, &tls.Config{InsecureSkipVerify: true})
	}
	conn := nc.conn.(*tls.Conn)
	conn.Handshake()
	nc.bw = bufio.NewWriterSize(nc.conn, defaultBufSize)
}

// waitForExits will wait for all socket watcher Go routines to
// be shutdown before proceeding.
func (nc *Conn) waitForExits() {
	// Kick old flusher forcefully.
	select {
	case nc.fch <- true:
	default:
	}

	// Wait for any previous go routines.
	nc.wg.Wait()
}

// spinUpGoRoutines will launch the Go routines responsible for
// reading and writing to the socket. This will be launched via a
// go routine itself to release any locks that may be held.
// We also use a WaitGroup to make sure we only start them on a
// reconnect when the previous ones have exited.
func (nc *Conn) spinUpGoRoutines() {
	// Make sure everything has exited.
	nc.waitForExits()

	// We will wait on both.
	nc.wg.Add(2)

	// Spin up the readLoop and the socket flusher.
	go nc.readLoop()
	go nc.flusher()

	nc.mu.Lock()
	if nc.Opts.PingInterval > 0 {
		if nc.ptmr == nil {
			nc.ptmr = time.AfterFunc(nc.Opts.PingInterval, nc.processPingTimer)
		} else {
			nc.ptmr.Reset(nc.Opts.PingInterval)
		}
	}
	nc.mu.Unlock()
}

// Report the connected server's Url
func (nc *Conn) ConnectedUrl() string {
	if nc == nil {
		return _EMPTY_
	}
	nc.mu.Lock()
	defer nc.mu.Unlock()
	if nc.status != CONNECTED {
		return _EMPTY_
	}
	return nc.url.String()
}

// Report the connected server's Id
func (nc *Conn) ConnectedServerId() string {
	if nc == nil {
		return _EMPTY_
	}
	nc.mu.Lock()
	defer nc.mu.Unlock()
	if nc.status != CONNECTED {
		return _EMPTY_
	}
	return nc.info.Id
}

// Low level setup for structs, etc
func (nc *Conn) setup() {
	nc.subs = make(map[int64]*Subscription)
	nc.pongs = make([]chan bool, 0, 8)

	nc.fch = make(chan bool, flushChanSize)

	// Setup scratch outbound buffer for PUB
	pub := nc.scratch[:len(_PUB_P_)]
	copy(pub, _PUB_P_)
}

// Process a connected connection and initialize properly.
func (nc *Conn) processConnectInit() error {

	// Set out deadline for the whole connect process
	nc.conn.SetDeadline(time.Now().Add(nc.Opts.Timeout))
	defer nc.conn.SetDeadline(time.Time{})

	// Set our status to connecting.
	nc.status = CONNECTING

	// Process the INFO protocol received from the server
	err := nc.processExpectedInfo()
	if err != nil {
		return err
	}

	// Send the CONNECT protocol along with the initial PING protocol.
	// Wait for the PONG response (or any error that we get from the server).
	err = nc.sendConnect()
	if err != nil {
		return err
	}

	// Reset the number of PING sent out
	nc.pout = 0

	go nc.spinUpGoRoutines()

	return nil
}

// Main connect function. Will connect to the nats-server
func (nc *Conn) connect() error {
	var returnedErr error

	// Create actual socket connection
	// For first connect we walk all servers in the pool and try
	// to connect immediately.
	nc.mu.Lock()
	// The pool may change inside theloop iteration due to INFO protocol.
	for i := 0; i < len(nc.srvPool); i++ {
		nc.url = nc.srvPool[i].url

		if err := nc.createConn(); err == nil {
			// This was moved out of processConnectInit() because
			// that function is now invoked from doReconnect() too.
			nc.setup()

			err = nc.processConnectInit()

			if err == nil {
				nc.srvPool[i].didConnect = true
				nc.srvPool[i].reconnects = 0
				returnedErr = nil
				break
			} else {
				returnedErr = err
				nc.mu.Unlock()
				nc.close(DISCONNECTED, false)
				nc.mu.Lock()
				nc.url = nil
			}
		} else {
			// Cancel out default connection refused, will trigger the
			// No servers error conditional
			if matched, _ := regexp.Match(`connection refused`, []byte(err.Error())); matched {
				returnedErr = nil
			}
		}
	}
	defer nc.mu.Unlock()

	if returnedErr == nil && nc.status != CONNECTED {
		returnedErr = ErrNoServers
	}
	return returnedErr
}

// This will check to see if the connection should be
// secure. This can be dictated from either end and should
// only be called after the INIT protocol has been received.
func (nc *Conn) checkForSecure() error {
	// Check to see if we need to engage TLS
	o := nc.Opts

	// Check for mismatch in setups
	if o.Secure && !nc.info.TLSRequired {
		return ErrSecureConnWanted
	} else if nc.info.TLSRequired && !o.Secure {
		return ErrSecureConnRequired
	}

	// Need to rewrap with bufio
	if o.Secure {
		nc.makeTLSConn()
	}
	return nil
}

// processExpectedInfo will look for the expected first INFO message
// sent when a connection is established. The lock should be held entering.
func (nc *Conn) processExpectedInfo() error {

	c := &control{}

	// Read the protocol
	err := nc.readOp(c)
	if err != nil {
		return err
	}

	// The nats protocol should send INFO first always.
	if c.op != _INFO_OP_ {
		return ErrNoInfoReceived
	}

	// Parse the protocol
	if err := nc.processInfo(c.args); err != nil {
		return err
	}

	err = nc.checkForSecure()
	if err != nil {
		return err
	}

	return nil
}

// Sends a protocol control message by queuing into the bufio writer
// and kicking the flush Go routine.  These writes are protected.
func (nc *Conn) sendProto(proto string) {
	nc.mu.Lock()
	nc.bw.WriteString(proto)
	nc.kickFlusher()
	nc.mu.Unlock()
}

// Generate a connect protocol message, issuing user/password if
// applicable. The lock is assumed to be held upon entering.
func (nc *Conn) connectProto() (string, error) {
	o := nc.Opts
	var user, pass, token string
	u := nc.url.User
	if u != nil {
		// if no password, assume username is authToken
		if _, ok := u.Password(); !ok {
			token = u.Username()
		} else {
			user = u.Username()
			pass, _ = u.Password()
		}
	} else {
		// Take from options (pssibly all empty strings)
		user = nc.Opts.User
		pass = nc.Opts.Password
		token = nc.Opts.Token
	}
	cinfo := connectInfo{o.Verbose, o.Pedantic,
		user, pass, token,
		o.Secure, o.Name, LangString, Version, clientProtoInfo}
	b, err := json.Marshal(cinfo)
	if err != nil {
		return _EMPTY_, ErrJsonParse
	}
	return fmt.Sprintf(conProto, b), nil
}

// normalizeErr removes the prefix -ERR, trim spaces and remove the quotes.
func normalizeErr(line string) string {
	s := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, _ERR_OP_)))
	s = strings.TrimLeft(strings.TrimRight(s, "'"), "'")
	return s
}

// Send a connect protocol message to the server, issue user/password if
// applicable. Will wait for a flush to return from the server for error
// processing.
func (nc *Conn) sendConnect() error {

	// Construct the CONNECT protocol string
	cProto, err := nc.connectProto()
	if err != nil {
		return err
	}

	// Write the protocol into the buffer
	_, err = nc.bw.WriteString(cProto)
	if err != nil {
		return err
	}

	// Add to the buffer the PING protocol
	_, err = nc.bw.WriteString(pingProto)
	if err != nil {
		return err
	}

	// Flush the buffer
	err = nc.bw.Flush()
	if err != nil {
		return err
	}

	// Now read the response from the server.
	br := bufio.NewReaderSize(nc.conn, defaultBufSize)
	line, err := br.ReadString('\n')
	if err != nil {
		return err
	}

	// If opts.Verbose is set, handle +OK
	if nc.Opts.Verbose && line == okProto {
		// Read the rest now...
		line, err = br.ReadString('\n')
		if err != nil {
			return err
		}
	}

	// We expect a PONG
	if line != pongProto {
		// But it could be something else, like -ERR

		// Since we no longer use ReadLine(), trim the trailing "\r\n"
		line = strings.TrimRight(line, "\r\n")

		// If it's a server error...
		if strings.HasPrefix(line, _ERR_OP_) {
			// Remove -ERR, trim spaces and quotes, and convert to lower case.
			line = normalizeErr(line)
			return errors.New("nats: " + line)
		}

		// Notify that we got an unexpected protocol.
		return errors.New(fmt.Sprintf("nats: expected '%s', got '%s'", _PONG_OP_, line))
	}

	// This is where we are truly connected.
	nc.status = CONNECTED

	return nil
}

// A control protocol line.
type control struct {
	op, args string
}

// Read a control line and process the intended op.
func (nc *Conn) readOp(c *control) error {
	br := bufio.NewReaderSize(nc.conn, defaultBufSize)
	line, err := br.ReadString('\n')
	if err != nil {
		return err
	}
	parseControl(line, c)
	return nil
}

// Parse a control line from the server.
func parseControl(line string, c *control) {
	toks := strings.SplitN(line, _SPC_, 2)
	if len(toks) == 1 {
		c.op = strings.TrimSpace(toks[0])
		c.args = _EMPTY_
	} else if len(toks) == 2 {
		c.op, c.args = strings.TrimSpace(toks[0]), strings.TrimSpace(toks[1])
	} else {
		c.op = _EMPTY_
	}
}

// flushReconnectPending will push the pending items that were
// gathered while we were in a RECONNECTING state to the socket.
func (nc *Conn) flushReconnectPendingItems() {
	if nc.pending == nil {
		return
	}
	if nc.pending.Len() > 0 {
		nc.bw.Write(nc.pending.Bytes())
	}
}

// Try to reconnect using the option parameters.
// This function assumes we are allowed to reconnect.
func (nc *Conn) doReconnect() {
	// We want to make sure we have the other watchers shutdown properly
	// here before we proceed past this point.
	nc.waitForExits()

	// FIXME(dlc) - We have an issue here if we have
	// outstanding flush points (pongs) and they were not
	// sent out, but are still in the pipe.

	// Hold the lock manually and release where needed below,
	// can't do defer here.
	nc.mu.Lock()

	// Clear any queued pongs, e.g. pending flush calls.
	nc.clearPendingFlushCalls()

	// Clear any errors.
	nc.err = nil

	// Perform appropriate callback if needed for a disconnect.
	if nc.Opts.DisconnectedCB != nil {
		nc.ach <- func() { nc.Opts.DisconnectedCB(nc) }
	}

	for len(nc.srvPool) > 0 {
		cur, err := nc.selectNextServer()
		if err != nil {
			nc.err = err
			break
		}

		sleepTime := int64(0)

		// Sleep appropriate amount of time before the
		// connection attempt if connecting to same server
		// we just got disconnected from..
		if time.Since(cur.lastAttempt) < nc.Opts.ReconnectWait {
			sleepTime = int64(nc.Opts.ReconnectWait - time.Since(cur.lastAttempt))
		}

		// On Windows, createConn() will take more than a second when no
		// server is running at that address. So it could be that the
		// time elapsed between reconnect attempts is always > than
		// the set option. Release the lock to give a chance to a parallel
		// nc.Close() to break the loop.
		nc.mu.Unlock()
		if sleepTime <= 0 {
			runtime.Gosched()
		} else {
			time.Sleep(time.Duration(sleepTime))
		}
		nc.mu.Lock()

		// Check if we have been closed first.
		if nc.isClosed() {
			break
		}

		// Mark that we tried a reconnect
		cur.reconnects++

		// Try to create a new connection
		err = nc.createConn()

		// Not yet connected, retry...
		// Continue to hold the lock
		if err != nil {
			nc.err = nil
			continue
		}

		// We are reconnected
		nc.Reconnects++

		// Process connect logic
		if nc.err = nc.processConnectInit(); nc.err != nil {
			nc.status = RECONNECTING
			continue
		}

		// Clear out server stats for the server we connected to..
		cur.didConnect = true
		cur.reconnects = 0

		// Send existing subscription state
		nc.resendSubscriptions()

		// Now send off and clear pending buffer
		nc.flushReconnectPendingItems()

		// Flush the buffer
		nc.err = nc.bw.Flush()
		if nc.err != nil {
			nc.status = RECONNECTING
			continue
		}

		// Done with the pending buffer
		nc.pending = nil

		// This is where we are truly connected.
		nc.status = CONNECTED

		// Queue up the reconnect callback.
		if nc.Opts.ReconnectedCB != nil {
			nc.ach <- func() { nc.Opts.ReconnectedCB(nc) }
		}

		// Release lock here, we will return below.
		nc.mu.Unlock()

		// Make sure to flush everything
		nc.Flush()

		return
	}

	// Call into close.. We have no servers left..
	if nc.err == nil {
		nc.err = ErrNoServers
	}
	nc.mu.Unlock()
	nc.Close()
}

// processOpErr handles errors from reading or parsing the protocol.
// The lock should not be held entering this function.
func (nc *Conn) processOpErr(err error) {
	nc.mu.Lock()
	if nc.isConnecting() || nc.isClosed() || nc.isReconnecting() {
		nc.mu.Unlock()
		return
	}

	if nc.Opts.AllowReconnect && nc.status == CONNECTED {
		// Set our new status
		nc.status = RECONNECTING
		if nc.ptmr != nil {
			nc.ptmr.Stop()
		}
		if nc.conn != nil {
			nc.bw.Flush()
			nc.conn.Close()
			nc.conn = nil
		}

		// Create a new pending buffer to underpin the bufio Writer while
		// we are reconnecting.
		nc.pending = &bytes.Buffer{}
		nc.bw = bufio.NewWriterSize(nc.pending, nc.Opts.ReconnectBufSize)

		go nc.doReconnect()
		nc.mu.Unlock()
		return
	}

	nc.status = DISCONNECTED
	nc.err = err
	nc.mu.Unlock()
	nc.Close()
}

// Marker to close the channel to kick out the Go routine.
func (nc *Conn) closeAsyncFunc() asyncCB {
	return func() {
		nc.mu.Lock()
		if nc.ach != nil {
			close(nc.ach)
			nc.ach = nil
		}
		nc.mu.Unlock()
	}
}

// asyncDispatch is responsible for calling any async callbacks
func (nc *Conn) asyncDispatch() {
	// snapshot since they can change from underneath of us.
	nc.mu.Lock()
	ach := nc.ach
	nc.mu.Unlock()

	// Loop on the channel and process async callbacks.
	for {
		if f, ok := <-ach; !ok {
			return
		} else {
			f()
		}
	}
}

// readLoop() will sit on the socket reading and processing the
// protocol from the server. It will dispatch appropriately based
// on the op type.
func (nc *Conn) readLoop() {
	// Release the wait group on exit
	defer nc.wg.Done()

	// Create a parseState if needed.
	nc.mu.Lock()
	if nc.ps == nil {
		nc.ps = &parseState{}
	}
	nc.mu.Unlock()

	// Stack based buffer.
	b := make([]byte, defaultBufSize)

	for {
		// FIXME(dlc): RWLock here?
		nc.mu.Lock()
		sb := nc.isClosed() || nc.isReconnecting()
		if sb {
			nc.ps = &parseState{}
		}
		conn := nc.conn
		nc.mu.Unlock()

		if sb || conn == nil {
			break
		}

		n, err := conn.Read(b)
		if err != nil {
			nc.processOpErr(err)
			break
		}

		if err := nc.parse(b[:n]); err != nil {
			nc.processOpErr(err)
			break
		}
	}
	// Clear the parseState here..
	nc.mu.Lock()
	nc.ps = nil
	nc.mu.Unlock()
}

// waitForMsgs waits on the conditional shared with readLoop and processMsg.
// It is used to deliver messages to asynchronous subscribers.
func (nc *Conn) waitForMsgs(s *Subscription) {
	var closed bool
	var delivered, max uint64

	for {
		s.mu.Lock()
		if s.pHead == nil && !s.closed {
			s.pCond.Wait()
		}
		// Pop the msg off the list
		m := s.pHead
		if m != nil {
			s.pHead = m.next
			if s.pHead == nil {
				s.pTail = nil
			}
			s.pMsgs--
			s.pBytes -= len(m.Data)
		}
		mcb := s.mcb
		max = s.max
		closed = s.closed
		if !s.closed {
			s.delivered++
			delivered = s.delivered
		}
		s.mu.Unlock()

		if closed {
			break
		}

		// Deliver the message.
		if m != nil && (max <= 0 || delivered <= max) {
			mcb(m)
		}
		// If we have hit the max for delivered msgs, remove sub.
		if max > 0 && delivered >= max {
			nc.mu.Lock()
			nc.removeSub(s)
			nc.mu.Unlock()
			break
		}
	}
}

// processMsg is called by parse and will place the msg on the
// appropriate channel/pending queue for processing. If the channel is full,
// or the pending queue is over the pending limits, the connection is
// considered a slow consumer.
func (nc *Conn) processMsg(data []byte) {
	// Lock from here on out.
	nc.mu.Lock()

	// Stats
	nc.InMsgs++
	nc.InBytes += uint64(len(data))

	sub := nc.subs[nc.ps.ma.sid]
	if sub == nil {
		nc.mu.Unlock()
		return
	}

	// Copy them into string
	subj := string(nc.ps.ma.subject)
	reply := string(nc.ps.ma.reply)

	// Doing message create outside of the sub's lock to reduce contention.
	// It's possible that we end-up not using the message, but that's ok.

	// FIXME(dlc): Need to copy, should/can do COW?
	msgPayload := make([]byte, len(data))
	copy(msgPayload, data)

	// FIXME(dlc): Should we recycle these containers?
	m := &Msg{Data: msgPayload, Subject: subj, Reply: reply, Sub: sub}

	sub.mu.Lock()

	// Subscription internal stats (applicable only for non ChanSubscription's)
	if sub.typ != ChanSubscription {
		sub.pMsgs++
		if sub.pMsgs > sub.pMsgsMax {
			sub.pMsgsMax = sub.pMsgs
		}
		sub.pBytes += len(m.Data)
		if sub.pBytes > sub.pBytesMax {
			sub.pBytesMax = sub.pBytes
		}

		// Check for a Slow Consumer
		if (sub.pMsgsLimit > 0 && sub.pMsgs > sub.pMsgsLimit) ||
			(sub.pBytesLimit > 0 && sub.pBytes > sub.pBytesLimit) {
			goto slowConsumer
		}
	}

	// We have two modes of delivery. One is the channel, used by channel
	// subscribers and syncSubscribers, the other is a linked list for async.
	if sub.mch != nil {
		select {
		case sub.mch <- m:
		default:
			goto slowConsumer
		}
	} else {
		// Push onto the async pList
		if sub.pHead == nil {
			sub.pHead = m
			sub.pTail = m
			sub.pCond.Signal()
		} else {
			sub.pTail.next = m
			sub.pTail = m
		}
	}

	// Clear SlowConsumer status.
	sub.sc = false

	sub.mu.Unlock()
	nc.mu.Unlock()
	return

slowConsumer:
	sub.dropped++
	nc.processSlowConsumer(sub)
	// Undo stats from above
	if sub.typ != ChanSubscription {
		sub.pMsgs--
		sub.pBytes -= len(m.Data)
	}
	sub.mu.Unlock()
	nc.mu.Unlock()
	return
}

// processSlowConsumer will set SlowConsumer state and fire the
// async error handler if registered.
func (nc *Conn) processSlowConsumer(s *Subscription) {
	nc.err = ErrSlowConsumer
	if nc.Opts.AsyncErrorCB != nil && !s.sc {
		nc.ach <- func() { nc.Opts.AsyncErrorCB(nc, s, ErrSlowConsumer) }
	}
	s.sc = true
}

// processPermissionsViolation is called when the server signals a subject
// permissions violation on either publish or subscribe.
func (nc *Conn) processPermissionsViolation(err string) {
	nc.err = errors.New("nats: " + err)
	if nc.Opts.AsyncErrorCB != nil {
		nc.ach <- func() { nc.Opts.AsyncErrorCB(nc, nil, nc.err) }
	}
}

// flusher is a separate Go routine that will process flush requests for the write
// bufio. This allows coalescing of writes to the underlying socket.
func (nc *Conn) flusher() {
	// Release the wait group
	defer nc.wg.Done()

	// snapshot the bw and conn since they can change from underneath of us.
	nc.mu.Lock()
	bw := nc.bw
	conn := nc.conn
	fch := nc.fch
	nc.mu.Unlock()

	if conn == nil || bw == nil {
		return
	}

	for {
		if _, ok := <-fch; !ok {
			return
		}
		nc.mu.Lock()

		// Check to see if we should bail out.
		if !nc.isConnected() || nc.isConnecting() || bw != nc.bw || conn != nc.conn {
			nc.mu.Unlock()
			return
		}
		if bw.Buffered() > 0 {
			if err := bw.Flush(); err != nil {
				if nc.err == nil {
					nc.err = err
				}
			}
		}
		nc.mu.Unlock()
	}
}

// processPing will send an immediate pong protocol response to the
// server. The server uses this mechanism to detect dead clients.
func (nc *Conn) processPing() {
	nc.sendProto(pongProto)
}

// processPong is used to process responses to the client's ping
// messages. We use pings for the flush mechanism as well.
func (nc *Conn) processPong() {
	var ch chan bool

	nc.mu.Lock()
	if len(nc.pongs) > 0 {
		ch = nc.pongs[0]
		nc.pongs = nc.pongs[1:]
	}
	nc.pout = 0
	nc.mu.Unlock()
	if ch != nil {
		ch <- true
	}
}

// processOK is a placeholder for processing OK messages.
func (nc *Conn) processOK() {
	// do nothing
}

// processInfo is used to parse the info messages sent
// from the server.
// This function may update the server pool.
func (nc *Conn) processInfo(info string) error {
	if info == _EMPTY_ {
		return nil
	}
	if err := json.Unmarshal([]byte(info), &nc.info); err != nil {
		return err
	}
	updated := false
	urls := nc.info.ConnectURLs
	for _, curl := range urls {
		if _, present := nc.urls[curl]; !present {
			if err := nc.addURLToPool(fmt.Sprintf("nats://%s", curl), true); err != nil {
				continue
			}
			updated = true
		}
	}
	if updated && !nc.Opts.NoRandomize {
		nc.shufflePool()
	}
	return nil
}

// processAsyncInfo does the same than processInfo, but is called
// from the parser. Calls processInfo under connection's lock
// protection.
func (nc *Conn) processAsyncInfo(info []byte) {
	nc.mu.Lock()
	// Ignore errors, we will simply not update the server pool...
	nc.processInfo(string(info))
	nc.mu.Unlock()
}

// LastError reports the last error encountered via the connection.
// It can be used reliably within ClosedCB in order to find out reason
// why connection was closed for example.
func (nc *Conn) LastError() error {
	if nc == nil {
		return ErrInvalidConnection
	}
	nc.mu.Lock()
	err := nc.err
	nc.mu.Unlock()
	return err
}

// processErr processes any error messages from the server and
// sets the connection's lastError.
func (nc *Conn) processErr(e string) {
	// Trim, remove quotes, convert to lower case.
	e = normalizeErr(e)

	// FIXME(dlc) - process Slow Consumer signals special.
	if e == STALE_CONNECTION {
		nc.processOpErr(ErrStaleConnection)
	} else if strings.HasPrefix(e, PERMISSIONS_ERR) {
		nc.processPermissionsViolation(e)
	} else {
		nc.mu.Lock()
		nc.err = errors.New("nats: " + e)
		nc.mu.Unlock()
		nc.Close()
	}
}

// kickFlusher will send a bool on a channel to kick the
// flush Go routine to flush data to the server.
func (nc *Conn) kickFlusher() {
	if nc.bw != nil {
		select {
		case nc.fch <- true:
		default:
		}
	}
}

// Publish publishes the data argument to the given subject. The data
// argument is left untouched and needs to be correctly interpreted on
// the receiver.
func (nc *Conn) Publish(subj string, data []byte) error {
	return nc.publish(subj, _EMPTY_, data)
}

// PublishMsg publishes the Msg structure, which includes the
// Subject, an optional Reply and an optional Data field.
func (nc *Conn) PublishMsg(m *Msg) error {
	if m == nil {
		return ErrInvalidMsg
	}
	return nc.publish(m.Subject, m.Reply, m.Data)
}

// PublishRequest will perform a Publish() excpecting a response on the
// reply subject. Use Request() for automatically waiting for a response
// inline.
func (nc *Conn) PublishRequest(subj, reply string, data []byte) error {
	return nc.publish(subj, reply, data)
}

// Used for handrolled itoa
const digits = "0123456789"

// publish is the internal function to publish messages to a nats-server.
// Sends a protocol data message by queuing into the bufio writer
// and kicking the flush go routine. These writes should be protected.
func (nc *Conn) publish(subj, reply string, data []byte) error {
	if nc == nil {
		return ErrInvalidConnection
	}
	if subj == "" {
		return ErrBadSubject
	}
	nc.mu.Lock()

	// Proactively reject payloads over the threshold set by server.
	var msgSize int64
	msgSize = int64(len(data))
	if msgSize > nc.info.MaxPayload {
		nc.mu.Unlock()
		return ErrMaxPayload
	}

	if nc.isClosed() {
		nc.mu.Unlock()
		return ErrConnectionClosed
	}

	// Check if we are reconnecting, and if so check if
	// we have exceeded our reconnect outbound buffer limits.
	if nc.isReconnecting() {
		// Flush to underlying buffer.
		nc.bw.Flush()
		// Check if we are over
		if nc.pending.Len() >= nc.Opts.ReconnectBufSize {
			nc.mu.Unlock()
			return ErrReconnectBufExceeded
		}
	}

	msgh := nc.scratch[:len(_PUB_P_)]
	msgh = append(msgh, subj...)
	msgh = append(msgh, ' ')
	if reply != "" {
		msgh = append(msgh, reply...)
		msgh = append(msgh, ' ')
	}

	// We could be smarter here, but simple loop is ok,
	// just avoid strconv in fast path
	// FIXME(dlc) - Find a better way here.
	// msgh = strconv.AppendInt(msgh, int64(len(data)), 10)

	var b [12]byte
	var i = len(b)
	if len(data) > 0 {
		for l := len(data); l > 0; l /= 10 {
			i -= 1
			b[i] = digits[l%10]
		}
	} else {
		i -= 1
		b[i] = digits[0]
	}

	msgh = append(msgh, b[i:]...)
	msgh = append(msgh, _CRLF_...)

	// FIXME, do deadlines here
	_, err := nc.bw.Write(msgh)
	if err == nil {
		_, err = nc.bw.Write(data)
	}
	if err == nil {
		_, err = nc.bw.WriteString(_CRLF_)
	}
	if err != nil {
		nc.mu.Unlock()
		return err
	}

	nc.OutMsgs++
	nc.OutBytes += uint64(len(data))

	if len(nc.fch) == 0 {
		nc.kickFlusher()
	}
	nc.mu.Unlock()
	return nil
}

// Request will create an Inbox and perform a Request() call
// with the Inbox reply and return the first reply received.
// This is optimized for the case of multiple responses.
func (nc *Conn) Request(subj string, data []byte, timeout time.Duration) (*Msg, error) {
	inbox := NewInbox()
	ch := make(chan *Msg, RequestChanLen)

	s, err := nc.subscribe(inbox, _EMPTY_, nil, ch)
	if err != nil {
		return nil, err
	}
	s.AutoUnsubscribe(1)
	defer s.Unsubscribe()

	err = nc.PublishRequest(subj, inbox, data)
	if err != nil {
		return nil, err
	}
	return s.NextMsg(timeout)
}

// InboxPrefix is the prefix for all inbox subjects.
const InboxPrefix = "_INBOX."
const inboxPrefixLen = len(InboxPrefix)

// NewInbox will return an inbox string which can be used for directed replies from
// subscribers. These are guaranteed to be unique, but can be shared and subscribed
// to by others.
func NewInbox() string {
	var b [inboxPrefixLen + 22]byte
	pres := b[:inboxPrefixLen]
	copy(pres, InboxPrefix)
	ns := b[inboxPrefixLen:]
	copy(ns, nuid.Next())
	return string(b[:])
}

// Subscribe will express interest in the given subject. The subject
// can have wildcards (partial:*, full:>). Messages will be delivered
// to the associated MsgHandler. If no MsgHandler is given, the
// subscription is a synchronous subscription and can be polled via
// Subscription.NextMsg().
func (nc *Conn) Subscribe(subj string, cb MsgHandler) (*Subscription, error) {
	return nc.subscribe(subj, _EMPTY_, cb, nil)
}

// ChanSubscribe will place all messages received on the channel.
// You should not close the channel until sub.Unsubscribe() has been called.
func (nc *Conn) ChanSubscribe(subj string, ch chan *Msg) (*Subscription, error) {
	return nc.subscribe(subj, _EMPTY_, nil, ch)
}

// ChanQueueSubscribe will place all messages received on the channel.
// You should not close the channel until sub.Unsubscribe() has been called.
func (nc *Conn) ChanQueueSubscribe(subj, group string, ch chan *Msg) (*Subscription, error) {
	return nc.subscribe(subj, group, nil, ch)
}

// SubscribeSync is syntactic sugar for Subscribe(subject, nil).
func (nc *Conn) SubscribeSync(subj string) (*Subscription, error) {
	if nc == nil {
		return nil, ErrInvalidConnection
	}
	mch := make(chan *Msg, nc.Opts.SubChanLen)
	s, e := nc.subscribe(subj, _EMPTY_, nil, mch)
	if s != nil {
		s.typ = SyncSubscription
	}
	return s, e
}

// QueueSubscribe creates an asynchronous queue subscriber on the given subject.
// All subscribers with the same queue name will form the queue group and
// only one member of the group will be selected to receive any given
// message asynchronously.
func (nc *Conn) QueueSubscribe(subj, queue string, cb MsgHandler) (*Subscription, error) {
	return nc.subscribe(subj, queue, cb, nil)
}

// QueueSubscribeSync creates a synchronous queue subscriber on the given
// subject. All subscribers with the same queue name will form the queue
// group and only one member of the group will be selected to receive any
// given message synchronously.
func (nc *Conn) QueueSubscribeSync(subj, queue string) (*Subscription, error) {
	mch := make(chan *Msg, nc.Opts.SubChanLen)
	s, e := nc.subscribe(subj, queue, nil, mch)
	if s != nil {
		s.typ = SyncSubscription
	}
	return s, e
}

// QueueSubscribeSyncWithChan is syntactic sugar for ChanQueueSubscribe(subject, group, ch).
func (nc *Conn) QueueSubscribeSyncWithChan(subj, queue string, ch chan *Msg) (*Subscription, error) {
	return nc.subscribe(subj, queue, nil, ch)
}

// subscribe is the internal subscribe function that indicates interest in a subject.
func (nc *Conn) subscribe(subj, queue string, cb MsgHandler, ch chan *Msg) (*Subscription, error) {
	if nc == nil {
		return nil, ErrInvalidConnection
	}
	nc.mu.Lock()
	// ok here, but defer is generally expensive
	defer nc.mu.Unlock()
	defer nc.kickFlusher()

	// Check for some error conditions.
	if nc.isClosed() {
		return nil, ErrConnectionClosed
	}

	if cb == nil && ch == nil {
		return nil, ErrBadSubscription
	}

	sub := &Subscription{Subject: subj, Queue: queue, mcb: cb, conn: nc}
	// Set pending limits.
	sub.pMsgsLimit = DefaultSubPendingMsgsLimit
	sub.pBytesLimit = DefaultSubPendingBytesLimit

	// If we have an async callback, start up a sub specific
	// Go routine to deliver the messages.
	if cb != nil {
		sub.typ = AsyncSubscription
		sub.pCond = sync.NewCond(&sub.mu)
		go nc.waitForMsgs(sub)
	} else {
		sub.typ = ChanSubscription
		sub.mch = ch
	}

	sub.sid = atomic.AddInt64(&nc.ssid, 1)
	nc.subs[sub.sid] = sub

	// We will send these for all subs when we reconnect
	// so that we can suppress here.
	if !nc.isReconnecting() {
		nc.bw.WriteString(fmt.Sprintf(subProto, subj, queue, sub.sid))
	}
	return sub, nil
}

// Lock for nc should be held here upon entry
func (nc *Conn) removeSub(s *Subscription) {
	delete(nc.subs, s.sid)
	s.mu.Lock()
	defer s.mu.Unlock()
	// Release callers on NextMsg for SyncSubscription only
	if s.mch != nil && s.typ == SyncSubscription {
		close(s.mch)
	}
	s.mch = nil

	// Mark as invalid
	s.conn = nil
	s.closed = true
	if s.pCond != nil {
		s.pCond.Broadcast()
	}
}

// SubscriptionType is the type of the Subscription.
type SubscriptionType int

// The different types of subscription types.
const (
	AsyncSubscription = SubscriptionType(iota)
	SyncSubscription
	ChanSubscription
	NilSubscription
)

// Type returns the type of Subscription.
func (s *Subscription) Type() SubscriptionType {
	if s == nil {
		return NilSubscription
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.typ
}

// IsValid returns a boolean indicating whether the subscription
// is still active. This will return false if the subscription has
// already been closed.
func (s *Subscription) IsValid() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn != nil
}

// Unsubscribe will remove interest in the given subject.
func (s *Subscription) Unsubscribe() error {
	if s == nil {
		return ErrBadSubscription
	}
	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()
	if conn == nil {
		return ErrBadSubscription
	}
	return conn.unsubscribe(s, 0)
}

// AutoUnsubscribe will issue an automatic Unsubscribe that is
// processed by the server when max messages have been received.
// This can be useful when sending a request to an unknown number
// of subscribers. Request() uses this functionality.
func (s *Subscription) AutoUnsubscribe(max int) error {
	if s == nil {
		return ErrBadSubscription
	}
	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()
	if conn == nil {
		return ErrBadSubscription
	}
	return conn.unsubscribe(s, max)
}

// unsubscribe performs the low level unsubscribe to the server.
// Use Subscription.Unsubscribe()
func (nc *Conn) unsubscribe(sub *Subscription, max int) error {
	nc.mu.Lock()
	// ok here, but defer is expensive
	defer nc.mu.Unlock()
	defer nc.kickFlusher()

	if nc.isClosed() {
		return ErrConnectionClosed
	}

	s := nc.subs[sub.sid]
	// Already unsubscribed
	if s == nil {
		return nil
	}

	maxStr := _EMPTY_
	if max > 0 {
		s.max = uint64(max)
		maxStr = strconv.Itoa(max)
	} else {
		nc.removeSub(s)
	}
	// We will send these for all subs when we reconnect
	// so that we can suppress here.
	if !nc.isReconnecting() {
		nc.bw.WriteString(fmt.Sprintf(unsubProto, s.sid, maxStr))
	}
	return nil
}

// NextMsg() will return the next message available to a synchronous subscriber
// or block until one is available. A timeout can be used to return when no
// message has been delivered.
func (s *Subscription) NextMsg(timeout time.Duration) (*Msg, error) {
	if s == nil {
		return nil, ErrBadSubscription
	}
	s.mu.Lock()
	if s.connClosed {
		s.mu.Unlock()
		return nil, ErrConnectionClosed
	}
	if s.mch == nil {
		if s.max > 0 && s.delivered >= s.max {
			s.mu.Unlock()
			return nil, ErrMaxMessages
		} else if s.closed {
			s.mu.Unlock()
			return nil, ErrBadSubscription
		}
	}
	if s.mcb != nil {
		s.mu.Unlock()
		return nil, ErrSyncSubRequired
	}
	if s.sc {
		s.sc = false
		s.mu.Unlock()
		return nil, ErrSlowConsumer
	}

	// snapshot
	nc := s.conn
	mch := s.mch
	max := s.max
	s.mu.Unlock()

	var ok bool
	var msg *Msg

	t := time.NewTimer(timeout)
	defer t.Stop()

	select {
	case msg, ok = <-mch:
		if !ok {
			return nil, ErrConnectionClosed
		}
		// Update some stats.
		s.mu.Lock()
		s.delivered++
		delivered := s.delivered
		if s.typ == SyncSubscription {
			s.pMsgs--
			s.pBytes -= len(msg.Data)
		}
		s.mu.Unlock()

		if max > 0 {
			if delivered > max {
				return nil, ErrMaxMessages
			}
			// Remove subscription if we have reached max.
			if delivered == max {
				nc.mu.Lock()
				nc.removeSub(s)
				nc.mu.Unlock()
			}
		}

	case <-t.C:
		return nil, ErrTimeout
	}

	return msg, nil
}

// Queued returns the number of queued messages in the client for this subscription.
// DEPRECATED: Use Pending()
func (s *Subscription) QueuedMsgs() (int, error) {
	m, _, err := s.Pending()
	return int(m), err
}

// Pending returns the number of queued messages and queued bytes in the client for this subscription.
func (s *Subscription) Pending() (int, int, error) {
	if s == nil {
		return -1, -1, ErrBadSubscription
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return -1, -1, ErrBadSubscription
	}
	if s.typ == ChanSubscription {
		return -1, -1, ErrTypeSubscription
	}
	return s.pMsgs, s.pBytes, nil
}

// MaxPending returns the maximum number of queued messages and queued bytes seen so far.
func (s *Subscription) MaxPending() (int, int, error) {
	if s == nil {
		return -1, -1, ErrBadSubscription
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return -1, -1, ErrBadSubscription
	}
	if s.typ == ChanSubscription {
		return -1, -1, ErrTypeSubscription
	}
	return s.pMsgsMax, s.pBytesMax, nil
}

// ClearMaxPending resets the maximums seen so far.
func (s *Subscription) ClearMaxPending() error {
	if s == nil {
		return ErrBadSubscription
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return ErrBadSubscription
	}
	if s.typ == ChanSubscription {
		return ErrTypeSubscription
	}
	s.pMsgsMax, s.pBytesMax = 0, 0
	return nil
}

// Pending Limits
const (
	DefaultSubPendingMsgsLimit  = 65536
	DefaultSubPendingBytesLimit = 65536 * 1024
)

// PendingLimits returns the current limits for this subscription.
// If no error is returned, a negative value indicates that the
// given metric is not limited.
func (s *Subscription) PendingLimits() (int, int, error) {
	if s == nil {
		return -1, -1, ErrBadSubscription
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return -1, -1, ErrBadSubscription
	}
	if s.typ == ChanSubscription {
		return -1, -1, ErrTypeSubscription
	}
	return s.pMsgsLimit, s.pBytesLimit, nil
}

// SetPendingLimits sets the limits for pending msgs and bytes for this subscription.
// Zero is not allowed. Any negative value means that the given metric is not limited.
func (s *Subscription) SetPendingLimits(msgLimit, bytesLimit int) error {
	if s == nil {
		return ErrBadSubscription
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return ErrBadSubscription
	}
	if s.typ == ChanSubscription {
		return ErrTypeSubscription
	}
	if msgLimit == 0 || bytesLimit == 0 {
		return ErrInvalidArg
	}
	s.pMsgsLimit, s.pBytesLimit = msgLimit, bytesLimit
	return nil
}

// Delivered returns the number of delivered messages for this subscription.
func (s *Subscription) Delivered() (int64, error) {
	if s == nil {
		return -1, ErrBadSubscription
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return -1, ErrBadSubscription
	}
	return int64(s.delivered), nil
}

// Dropped returns the number of known dropped messages for this subscription.
// This will correspond to messages dropped by violations of PendingLimits. If
// the server declares the connection a SlowConsumer, this number may not be
// valid.
func (s *Subscription) Dropped() (int, error) {
	if s == nil {
		return -1, ErrBadSubscription
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return -1, ErrBadSubscription
	}
	return s.dropped, nil
}

// FIXME: This is a hack
// removeFlushEntry is needed when we need to discard queued up responses
// for our pings as part of a flush call. This happens when we have a flush
// call outstanding and we call close.
func (nc *Conn) removeFlushEntry(ch chan bool) bool {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	if nc.pongs == nil {
		return false
	}
	for i, c := range nc.pongs {
		if c == ch {
			nc.pongs[i] = nil
			return true
		}
	}
	return false
}

// The lock must be held entering this function.
func (nc *Conn) sendPing(ch chan bool) {
	nc.pongs = append(nc.pongs, ch)
	nc.bw.WriteString(pingProto)
	// Flush in place.
	nc.bw.Flush()
}

// This will fire periodically and send a client origin
// ping to the server. Will also check that we have received
// responses from the server.
func (nc *Conn) processPingTimer() {
	nc.mu.Lock()

	if nc.status != CONNECTED {
		nc.mu.Unlock()
		return
	}

	// Check for violation
	nc.pout++
	if nc.pout > nc.Opts.MaxPingsOut {
		nc.mu.Unlock()
		nc.processOpErr(ErrStaleConnection)
		return
	}

	nc.sendPing(nil)
	nc.ptmr.Reset(nc.Opts.PingInterval)
	nc.mu.Unlock()
}

// FlushTimeout allows a Flush operation to have an associated timeout.
func (nc *Conn) FlushTimeout(timeout time.Duration) (err error) {
	if nc == nil {
		return ErrInvalidConnection
	}
	if timeout <= 0 {
		return ErrBadTimeout
	}

	nc.mu.Lock()
	if nc.isClosed() {
		nc.mu.Unlock()
		return ErrConnectionClosed
	}
	t := time.NewTimer(timeout)
	defer t.Stop()

	ch := make(chan bool) // FIXME: Inefficient?
	nc.sendPing(ch)
	nc.mu.Unlock()

	select {
	case _, ok := <-ch:
		if !ok {
			err = ErrConnectionClosed
		} else {
			close(ch)
		}
	case <-t.C:
		err = ErrTimeout
	}

	if err != nil {
		nc.removeFlushEntry(ch)
	}
	return
}

// Flush will perform a round trip to the server and return when it
// receives the internal reply.
func (nc *Conn) Flush() error {
	return nc.FlushTimeout(60 * time.Second)
}

// Buffered will return the number of bytes buffered to be sent to the server.
// FIXME(dlc) take into account disconnected state.
func (nc *Conn) Buffered() (int, error) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	if nc.isClosed() || nc.bw == nil {
		return -1, ErrConnectionClosed
	}
	return nc.bw.Buffered(), nil
}

// resendSubscriptions will send our subscription state back to the
// server. Used in reconnects
func (nc *Conn) resendSubscriptions() {
	for _, s := range nc.subs {
		adjustedMax := uint64(0)
		s.mu.Lock()
		if s.max > 0 {
			if s.delivered < s.max {
				adjustedMax = s.max - s.delivered
			}

			// adjustedMax could be 0 here if the number of delivered msgs
			// reached the max, if so unsubscribe.
			if adjustedMax == 0 {
				s.mu.Unlock()
				nc.bw.WriteString(fmt.Sprintf(unsubProto, s.sid, _EMPTY_))
				continue
			}
		}
		s.mu.Unlock()

		nc.bw.WriteString(fmt.Sprintf(subProto, s.Subject, s.Queue, s.sid))
		if adjustedMax > 0 {
			maxStr := strconv.Itoa(int(adjustedMax))
			nc.bw.WriteString(fmt.Sprintf(unsubProto, s.sid, maxStr))
		}
	}
}

// This will clear any pending flush calls and release pending calls.
// Lock is assumed to be held by the caller.
func (nc *Conn) clearPendingFlushCalls() {
	// Clear any queued pongs, e.g. pending flush calls.
	for _, ch := range nc.pongs {
		if ch != nil {
			close(ch)
		}
	}
	nc.pongs = nil
}

// Low level close call that will do correct cleanup and set
// desired status. Also controls whether user defined callbacks
// will be triggered. The lock should not be held entering this
// function. This function will handle the locking manually.
func (nc *Conn) close(status Status, doCBs bool) {
	nc.mu.Lock()
	if nc.isClosed() {
		nc.status = status
		nc.mu.Unlock()
		return
	}
	nc.status = CLOSED

	// Kick the Go routines so they fall out.
	nc.kickFlusher()
	nc.mu.Unlock()

	nc.mu.Lock()

	// Clear any queued pongs, e.g. pending flush calls.
	nc.clearPendingFlushCalls()

	if nc.ptmr != nil {
		nc.ptmr.Stop()
	}

	// Go ahead and make sure we have flushed the outbound
	if nc.conn != nil {
		nc.bw.Flush()
		defer nc.conn.Close()
	}

	// Close sync subscriber channels and release any
	// pending NextMsg() calls.
	for _, s := range nc.subs {
		s.mu.Lock()

		// Release callers on NextMsg for SyncSubscription only
		if s.mch != nil && s.typ == SyncSubscription {
			close(s.mch)
		}
		s.mch = nil
		// Mark as invalid, for signalling to deliverMsgs
		s.closed = true
		// Mark connection closed in subscription
		s.connClosed = true
		// If we have an async subscription, signals it to exit
		if s.typ == AsyncSubscription && s.pCond != nil {
			s.pCond.Signal()
		}

		s.mu.Unlock()
	}
	nc.subs = nil

	// Perform appropriate callback if needed for a disconnect.
	if doCBs {
		if nc.Opts.DisconnectedCB != nil && nc.conn != nil {
			nc.ach <- func() { nc.Opts.DisconnectedCB(nc) }
		}
		if nc.Opts.ClosedCB != nil {
			nc.ach <- func() { nc.Opts.ClosedCB(nc) }
		}
		nc.ach <- nc.closeAsyncFunc()
	}
	nc.status = status
	nc.mu.Unlock()
}

// Close will close the connection to the server. This call will release
// all blocking calls, such as Flush() and NextMsg()
func (nc *Conn) Close() {
	nc.close(CLOSED, true)
}

// IsClosed tests if a Conn has been closed.
func (nc *Conn) IsClosed() bool {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.isClosed()
}

// IsReconnecting tests if a Conn is reconnecting.
func (nc *Conn) IsReconnecting() bool {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.isReconnecting()
}

// IsConnected tests if a Conn is connected.
func (nc *Conn) IsConnected() bool {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.isConnected()
}

// caller must lock
func (nc *Conn) getServers(implicitOnly bool) []string {
	poolSize := len(nc.srvPool)
	var servers = make([]string, 0)
	for i := 0; i < poolSize; i++ {
		if implicitOnly && !nc.srvPool[i].isImplicit {
			continue
		}
		url := nc.srvPool[i].url
		servers = append(servers, fmt.Sprintf("%s://%s", url.Scheme, url.Host))
	}
	return servers
}

// Servers returns the list of known server urls, including additional
// servers discovered after a connection has been established.  If
// authentication is enabled, use UserInfo or Token when connecting with
// these urls.
func (nc *Conn) Servers() []string {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.getServers(false)
}

// DiscoveredServers returns only the server urls that have been discovered
// after a connection has been established. If authentication is enabled,
// use UserInfo or Token when connecting with these urls.
func (nc *Conn) DiscoveredServers() []string {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.getServers(true)
}

// Status returns the current state of the connection.
func (nc *Conn) Status() Status {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.status
}

// Test if Conn has been closed Lock is assumed held.
func (nc *Conn) isClosed() bool {
	return nc.status == CLOSED
}

// Test if Conn is in the process of connecting
func (nc *Conn) isConnecting() bool {
	return nc.status == CONNECTING
}

// Test if Conn is being reconnected.
func (nc *Conn) isReconnecting() bool {
	return nc.status == RECONNECTING
}

// Test if Conn is connected or connecting.
func (nc *Conn) isConnected() bool {
	return nc.status == CONNECTED
}

// Stats will return a race safe copy of the Statistics section for the connection.
func (nc *Conn) Stats() Statistics {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	stats := nc.Statistics
	return stats
}

// MaxPayload returns the size limit that a message payload can have.
// This is set by the server configuration and delivered to the client
// upon connect.
func (nc *Conn) MaxPayload() int64 {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.info.MaxPayload
}

// AuthRequired will return if the connected server requires authorization.
func (nc *Conn) AuthRequired() bool {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.info.AuthRequired
}

// TLSRequired will return if the connected server requires TLS connections.
func (nc *Conn) TLSRequired() bool {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.info.TLSRequired
}
