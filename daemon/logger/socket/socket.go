// +build linux

// Package socket provides the log driver for writing logs to a unix domain socket
package socket

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/logger"
)

const name = "socket"

type socketLogger struct {
	destination url.URL
	connection  net.Conn
	mutex       sync.Mutex
}

type message struct {
	Timestamp   time.Time `json:"timestamp"`
	Hostname    string    `json:"hostname"`
	ContainerID string    `json:"containerID"`
	Message     string    `json:"message"`
	Source      string    `json:"source"`
}

func init() {
	if err := logger.RegisterLogDriver(name, New); err != nil {
		logrus.Fatal(err)
	}
	if err := logger.RegisterLogOptValidator(name, ValidateLogOpt); err != nil {
		logrus.Fatal(err)
	}
}

func connect(s *socketLogger) error {
	switch s.destination.Scheme {
	case "unix":
		conn, err := net.Dial("unix", s.destination.Path)
		if err != nil {
			return err
		}
		s.connection = conn

	case "tcp", "udp":
		conn, err := net.Dial(s.destination.Scheme, s.destination.Host)
		if err != nil {
			return err
		}
		s.connection = conn
	}

	return nil
}

// New creates socket logger driver using configuration passed in context
func New(ctx logger.Context) (logger.Logger, error) {
	u, err := url.Parse(ctx.Config["destination"])
	if err != nil {
		return nil, err
	}

	s := &socketLogger{
		destination: *u,
		connection:  nil,
		mutex:       sync.Mutex{},
	}

	if err := connect(s); err != nil {
		return s, err
	}

	return s, nil
}

func (s *socketLogger) Log(msg *logger.Message) error {
	res := message{
		Timestamp:   msg.Timestamp,
		Hostname:    "",
		ContainerID: msg.ContainerID,
		Message:     string(msg.Line),
		Source:      msg.Source,
	}

	json, err := json.Marshal(res)
	if err != nil {
		return err
	}

	json = append(json, '\n')

	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, err = s.connection.Write([]byte(json))
	if err != nil {
		if err := connect(s); err != nil {
			return err
		}
		_, err = s.connection.Write([]byte(json))
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *socketLogger) Name() string {
	return name
}

func (s *socketLogger) Close() error {
	if s.connection != nil {
		err := s.connection.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// ValidateLogOpt looks for socket specific log options.
func ValidateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		switch key {
		case "destination":
		default:
			return fmt.Errorf("unknown log opt '%s' for socket log driver", key)
		}
	}

	if _, ok := cfg["destination"]; !ok {
		return fmt.Errorf("missing log opt 'destination'")
	}

	return nil
}
