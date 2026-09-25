// Copyright IBM Corp. 2013, 2026
// SPDX-License-Identifier: MIT

package metrics

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"time"
)

const (
	// statsdMaxLen is the maximum size of a packet
	// to send to statsd
	statsdMaxLen = 1400
)

// SinkLogger is a minimal structured logging interface that sink implementations
// use to emit operational error messages. It is satisfied by hclog.Logger and
// any other structured logger that provides Warn and Error methods with
// key/value pair arguments.
type SinkLogger interface {
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// StatsdSink provides a MetricSink that can be used
// with a statsite or statsd metrics server. It uses
// only UDP packets, while StatsiteSink uses TCP.
type StatsdSink struct {
	addr        string
	metricQueue chan string
	logger      SinkLogger
}

// NewStatsdSinkFromURL creates an StatsdSink from a URL. It is used
// (and tested) from NewMetricSinkFromURL.
func NewStatsdSinkFromURL(u *url.URL) (MetricSink, error) {
	return NewStatsdSink(u.Host)
}

// NewStatsdSink is used to create a new StatsdSink
func NewStatsdSink(addr string) (*StatsdSink, error) {
	s := &StatsdSink{
		addr:        addr,
		metricQueue: make(chan string, 4096),
	}
	go s.flushMetrics()
	return s, nil
}

// NewStatsdSinkWithLogger is used to create a new StatsdSink with a SinkLogger.
// The logger is set before the background flush goroutine starts, ensuring all
// operational errors are routed through it from the first connection attempt.
func NewStatsdSinkWithLogger(addr string, logger SinkLogger) (*StatsdSink, error) {
	s := &StatsdSink{
		addr:        addr,
		metricQueue: make(chan string, 4096),
		logger:      logger,
	}
	go s.flushMetrics()
	return s, nil
}

// SetLogger assigns a SinkLogger to the sink. When set, operational errors
// such as connection failures and flush errors are emitted through the provided
// logger instead of the stdlib log package, allowing callers to control the
// log format and destination.
func (s *StatsdSink) SetLogger(logger SinkLogger) {
	s.logger = logger
}

// logWarn emits a warning through the configured SinkLogger when one is set,
// falling back to stdlib log.Printf otherwise.
func (s *StatsdSink) logWarn(msg string, err error) {
	if s.logger != nil {
		s.logger.Warn(msg, "error", err)
	} else {
		log.Printf("[WARN] %s Err: %s", msg, err)
	}
}

// logErr emits an error through the configured SinkLogger when one is set,
// falling back to stdlib log.Printf otherwise.
func (s *StatsdSink) logErr(msg string, err error) {
	if s.logger != nil {
		s.logger.Error(msg, "error", err)
	} else {
		log.Printf("[ERR] %s Err: %s", msg, err)
	}
}

// Close is used to stop flushing to statsd
func (s *StatsdSink) Shutdown() {
	close(s.metricQueue)
}

func (s *StatsdSink) SetGauge(key []string, val float32) {
	flatKey := s.flattenKey(key)
	s.pushMetric(fmt.Sprintf("%s:%f|g\n", flatKey, val))
}

func (s *StatsdSink) SetGaugeWithLabels(key []string, val float32, labels []Label) {
	flatKey := s.flattenKeyLabels(key, labels)
	s.pushMetric(fmt.Sprintf("%s:%f|g\n", flatKey, val))
}

func (s *StatsdSink) SetPrecisionGauge(key []string, val float64) {
	flatKey := s.flattenKey(key)
	s.pushMetric(fmt.Sprintf("%s:%f|g\n", flatKey, val))
}

func (s *StatsdSink) SetPrecisionGaugeWithLabels(key []string, val float64, labels []Label) {
	flatKey := s.flattenKeyLabels(key, labels)
	s.pushMetric(fmt.Sprintf("%s:%f|g\n", flatKey, val))
}

func (s *StatsdSink) EmitKey(key []string, val float32) {
	flatKey := s.flattenKey(key)
	s.pushMetric(fmt.Sprintf("%s:%f|kv\n", flatKey, val))
}

func (s *StatsdSink) IncrCounter(key []string, val float32) {
	flatKey := s.flattenKey(key)
	s.pushMetric(fmt.Sprintf("%s:%f|c\n", flatKey, val))
}

func (s *StatsdSink) IncrCounterWithLabels(key []string, val float32, labels []Label) {
	flatKey := s.flattenKeyLabels(key, labels)
	s.pushMetric(fmt.Sprintf("%s:%f|c\n", flatKey, val))
}

func (s *StatsdSink) AddSample(key []string, val float32) {
	flatKey := s.flattenKey(key)
	s.pushMetric(fmt.Sprintf("%s:%f|ms\n", flatKey, val))
}

func (s *StatsdSink) AddSampleWithLabels(key []string, val float32, labels []Label) {
	flatKey := s.flattenKeyLabels(key, labels)
	s.pushMetric(fmt.Sprintf("%s:%f|ms\n", flatKey, val))
}

// Flattens the key for formatting, removes spaces
func (s *StatsdSink) flattenKey(parts []string) string {
	joined := strings.Join(parts, ".")
	return strings.Map(func(r rune) rune {
		switch r {
		case ':':
			fallthrough
		case ' ':
			return '_'
		default:
			return r
		}
	}, joined)
}

// Flattens the key along with labels for formatting, removes spaces
func (s *StatsdSink) flattenKeyLabels(parts []string, labels []Label) string {
	for _, label := range labels {
		parts = append(parts, label.Value)
	}
	return s.flattenKey(parts)
}

// Does a non-blocking push to the metrics queue
func (s *StatsdSink) pushMetric(m string) {
	select {
	case s.metricQueue <- m:
	default:
	}
}

// Flushes metrics
func (s *StatsdSink) flushMetrics() {
	var sock net.Conn
	var err error
	var wait <-chan time.Time
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

CONNECT:
	// Create a buffer
	buf := bytes.NewBuffer(nil)

	// Attempt to connect
	sock, err = net.Dial("udp", s.addr)
	if err != nil {
		s.logErr("Error connecting to statsd!", err)
		goto WAIT
	}

	for {
		select {
		case metric, ok := <-s.metricQueue:
			// Get a metric from the queue
			if !ok {
				goto QUIT
			}

			// Check if this would overflow the packet size
			if len(metric)+buf.Len() > statsdMaxLen {
				_, err := sock.Write(buf.Bytes())
				buf.Reset()
				if err != nil {
					s.logErr("Error writing to statsd!", err)
					goto WAIT
				}
			}

			// Append to the buffer
			buf.WriteString(metric)

		case <-ticker.C:
			if buf.Len() == 0 {
				continue
			}

			_, err := sock.Write(buf.Bytes())
			buf.Reset()
			if err != nil {
				s.logErr("Error flushing to statsd!", err)
				goto WAIT
			}
		}
	}

WAIT:
	// Wait for a while
	wait = time.After(time.Duration(5) * time.Second)
	for {
		select {
		// Dequeue the messages to avoid backlog
		case _, ok := <-s.metricQueue:
			if !ok {
				goto QUIT
			}
		case <-wait:
			goto CONNECT
		}
	}
QUIT:
	s.metricQueue = nil
}
