//go:build linux
// +build linux

package journald // import "github.com/docker/docker/daemon/logger/journald"

import (
	"fmt"
	"strconv"
	"time"
	"unicode"

	"github.com/coreos/go-systemd/v22/journal"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/loggerutils"
)

const name = "journald"

// Well-known user journal fields.
// https://www.freedesktop.org/software/systemd/man/systemd.journal-fields.html
const (
	fieldSyslogIdentifier = "SYSLOG_IDENTIFIER"
	fieldSyslogTimestamp  = "SYSLOG_TIMESTAMP"
)

// User journal fields used by the log driver.
const (
	fieldContainerID     = "CONTAINER_ID"
	fieldContainerIDFull = "CONTAINER_ID_FULL"
	fieldContainerName   = "CONTAINER_NAME"
	fieldContainerTag    = "CONTAINER_TAG"
	fieldImageName       = "IMAGE_NAME"

	// Fields used to serialize PLogMetaData.

	fieldPLogID         = "CONTAINER_PARTIAL_ID"
	fieldPLogOrdinal    = "CONTAINER_PARTIAL_ORDINAL"
	fieldPLogLast       = "CONTAINER_PARTIAL_LAST"
	fieldPartialMessage = "CONTAINER_PARTIAL_MESSAGE"
)

type journald struct {
	vars map[string]string // additional variables and values to send to the journal along with the log message

	closed chan struct{}

	// Overrides for unit tests.

	sendToJournal  func(message string, priority journal.Priority, vars map[string]string) error
	journalReadDir string //nolint:structcheck,unused // Referenced in read.go, which has more restrictive build constraints.
}

func init() {
	if err := logger.RegisterLogDriver(name, New); err != nil {
		panic(err)
	}
	if err := logger.RegisterLogOptValidator(name, validateLogOpt); err != nil {
		panic(err)
	}
}

// sanitizeKeyMode returns the sanitized string so that it could be used in journald.
// In journald log, there are special requirements for fields.
// Fields must be composed of uppercase letters, numbers, and underscores, but must
// not start with an underscore.
func sanitizeKeyMod(s string) string {
	n := ""
	for _, v := range s {
		if 'a' <= v && v <= 'z' {
			v = unicode.ToUpper(v)
		} else if ('Z' < v || v < 'A') && ('9' < v || v < '0') {
			v = '_'
		}
		// If (n == "" && v == '_'), then we will skip as this is the beginning with '_'
		if !(n == "" && v == '_') {
			n += string(v)
		}
	}
	return n
}

// New creates a journald logger using the configuration passed in on
// the context.
func New(info logger.Info) (logger.Logger, error) {
	if !journal.Enabled() {
		return nil, fmt.Errorf("journald is not enabled on this host")
	}

	return new(info)
}

func new(info logger.Info) (*journald, error) {
	// parse log tag
	tag, err := loggerutils.ParseLogTag(info, loggerutils.DefaultTemplate)
	if err != nil {
		return nil, err
	}

	vars := map[string]string{
		fieldContainerID:      info.ContainerID[:12],
		fieldContainerIDFull:  info.ContainerID,
		fieldContainerName:    info.Name(),
		fieldContainerTag:     tag,
		fieldImageName:        info.ImageName(),
		fieldSyslogIdentifier: tag,
	}
	extraAttrs, err := info.ExtraAttributes(sanitizeKeyMod)
	if err != nil {
		return nil, err
	}
	for k, v := range extraAttrs {
		vars[k] = v
	}
	return &journald{vars: vars, closed: make(chan struct{}), sendToJournal: journal.Send}, nil
}

// We don't actually accept any options, but we have to supply a callback for
// the factory to pass the (probably empty) configuration map to.
func validateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		switch key {
		case "labels":
		case "labels-regex":
		case "env":
		case "env-regex":
		case "tag":
		default:
			return fmt.Errorf("unknown log opt '%s' for journald log driver", key)
		}
	}
	return nil
}

func (s *journald) Log(msg *logger.Message) error {
	vars := map[string]string{}
	for k, v := range s.vars {
		vars[k] = v
	}
	if !msg.Timestamp.IsZero() {
		vars[fieldSyslogTimestamp] = msg.Timestamp.Format(time.RFC3339Nano)
	}
	if msg.PLogMetaData != nil {
		vars[fieldPLogID] = msg.PLogMetaData.ID
		vars[fieldPLogOrdinal] = strconv.Itoa(msg.PLogMetaData.Ordinal)
		vars[fieldPLogLast] = strconv.FormatBool(msg.PLogMetaData.Last)
		if !msg.PLogMetaData.Last {
			vars[fieldPartialMessage] = "true"
		}
	}

	line := string(msg.Line)
	source := msg.Source
	logger.PutMessage(msg)

	if source == "stderr" {
		return s.sendToJournal(line, journal.PriErr, vars)
	}
	return s.sendToJournal(line, journal.PriInfo, vars)
}

func (s *journald) Name() string {
	return name
}

func (s *journald) Close() error {
	close(s.closed)
	return nil
}
