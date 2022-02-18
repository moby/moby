// Package logentries provides the log driver for forwarding server logs
// to logentries endpoints.
package logentries // import "github.com/moby/moby/daemon/logger/logentries"

import (
	"fmt"
	"strconv"

	"github.com/bsphere/le_go"
	"github.com/moby/moby/daemon/logger"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type logentries struct {
	tag           string
	containerID   string
	containerName string
	writer        *le_go.Logger
	extra         map[string]string
	lineOnly      bool
}

const (
	name     = "logentries"
	token    = "logentries-token"
	lineonly = "line-only"
)

func init() {
	if err := logger.RegisterLogDriver(name, New); err != nil {
		logrus.Fatal(err)
	}
	if err := logger.RegisterLogOptValidator(name, ValidateLogOpt); err != nil {
		logrus.Fatal(err)
	}
}

// New creates a logentries logger using the configuration passed in on
// the context. The supported context configuration variable is
// logentries-token.
func New(info logger.Info) (logger.Logger, error) {
	logrus.WithField("container", info.ContainerID).
		WithField("token", info.Config[token]).
		WithField("line-only", info.Config[lineonly]).
		Debug("logging driver logentries configured")

	log, err := le_go.Connect(info.Config[token])
	if err != nil {
		return nil, errors.Wrap(err, "error connecting to logentries")
	}
	var lineOnly bool
	if info.Config[lineonly] != "" {
		if lineOnly, err = strconv.ParseBool(info.Config[lineonly]); err != nil {
			return nil, errors.Wrap(err, "error parsing lineonly option")
		}
	}
	return &logentries{
		containerID:   info.ContainerID,
		containerName: info.ContainerName,
		writer:        log,
		lineOnly:      lineOnly,
	}, nil
}

func (f *logentries) Log(msg *logger.Message) error {
	if !f.lineOnly {
		data := map[string]string{
			"container_id":   f.containerID,
			"container_name": f.containerName,
			"source":         msg.Source,
			"log":            string(msg.Line),
		}
		for k, v := range f.extra {
			data[k] = v
		}
		ts := msg.Timestamp
		logger.PutMessage(msg)
		f.writer.Println(f.tag, ts, data)
	} else {
		line := string(msg.Line)
		logger.PutMessage(msg)
		f.writer.Println(line)
	}
	return nil
}

func (f *logentries) Close() error {
	return f.writer.Close()
}

func (f *logentries) Name() string {
	return name
}

// ValidateLogOpt looks for logentries specific log option logentries-address.
func ValidateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		switch key {
		case "env":
		case "env-regex":
		case "labels":
		case "labels-regex":
		case "tag":
		case key:
		default:
			return fmt.Errorf("unknown log opt '%s' for logentries log driver", key)
		}
	}

	if cfg[token] == "" {
		return fmt.Errorf("Missing logentries token")
	}

	return nil
}
