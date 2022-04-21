package local // import "github.com/docker/docker/daemon/logger/local"

import (
	"encoding/binary"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/plugins/logdriver"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/loggerutils"
	"github.com/docker/docker/errdefs"
	units "github.com/docker/go-units"
	"github.com/pkg/errors"
)

const (
	// Name is the name of the driver
	Name = "local"

	encodeBinaryLen = 4
	initialBufSize  = 2048
	maxDecodeRetry  = 20000

	defaultMaxFileSize  int64 = 20 * 1024 * 1024
	defaultMaxFileCount       = 5
	defaultCompressLogs       = true
)

// LogOptKeys are the keys names used for log opts passed in to initialize the driver.
var LogOptKeys = map[string]bool{
	"max-file": true,
	"max-size": true,
	"compress": true,
}

// ValidateLogOpt looks for log driver specific options.
func ValidateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		if !LogOptKeys[key] {
			return errors.Errorf("unknown log opt '%s' for log driver %s", key, Name)
		}
	}
	return nil
}

func init() {
	if err := logger.RegisterLogDriver(Name, New); err != nil {
		panic(err)
	}
	if err := logger.RegisterLogOptValidator(Name, ValidateLogOpt); err != nil {
		panic(err)
	}
}

type driver struct {
	mu      sync.Mutex
	closed  bool
	logfile *loggerutils.LogFile
	readers map[*logger.LogWatcher]struct{} // stores the active log followers
}

// New creates a new local logger
// You must provide the `LogPath` in the passed in info argument, this is the file path that logs are written to.
func New(info logger.Info) (logger.Logger, error) {
	if info.LogPath == "" {
		return nil, errdefs.System(errors.New("log path is missing -- this is a bug and should not happen"))
	}

	cfg := newDefaultConfig()
	if capacity, ok := info.Config["max-size"]; ok {
		var err error
		cfg.MaxFileSize, err = units.FromHumanSize(capacity)
		if err != nil {
			return nil, errdefs.InvalidParameter(errors.Wrapf(err, "invalid value for max-size: %s", capacity))
		}
	}

	if userMaxFileCount, ok := info.Config["max-file"]; ok {
		var err error
		cfg.MaxFileCount, err = strconv.Atoi(userMaxFileCount)
		if err != nil {
			return nil, errdefs.InvalidParameter(errors.Wrapf(err, "invalid value for max-file: %s", userMaxFileCount))
		}
	}

	if userCompress, ok := info.Config["compress"]; ok {
		compressLogs, err := strconv.ParseBool(userCompress)
		if err != nil {
			return nil, errdefs.InvalidParameter(errors.Wrap(err, "error reading compress log option"))
		}
		cfg.DisableCompression = !compressLogs
	}
	return newDriver(info.LogPath, cfg)
}

func makeMarshaller() func(m *logger.Message) ([]byte, error) {
	buf := make([]byte, initialBufSize)

	// allocate the partial log entry separately, which allows for easier re-use
	proto := &logdriver.LogEntry{}
	md := &logdriver.PartialLogEntryMetadata{}

	return func(m *logger.Message) ([]byte, error) {
		resetProto(proto)

		messageToProto(m, proto, md)
		protoSize := proto.Size()
		writeLen := protoSize + (2 * encodeBinaryLen) // + len(messageDelimiter)

		if writeLen > len(buf) {
			buf = make([]byte, writeLen)
		} else {
			// shrink the buffer back down
			if writeLen <= initialBufSize {
				buf = buf[:initialBufSize]
			} else {
				buf = buf[:writeLen]
			}
		}

		binary.BigEndian.PutUint32(buf[:encodeBinaryLen], uint32(protoSize))
		n, err := proto.MarshalTo(buf[encodeBinaryLen:writeLen])
		if err != nil {
			return nil, errors.Wrap(err, "error marshaling log entry")
		}
		if n+(encodeBinaryLen*2) != writeLen {
			return nil, io.ErrShortWrite
		}
		binary.BigEndian.PutUint32(buf[writeLen-encodeBinaryLen:writeLen], uint32(protoSize))
		return buf[:writeLen], nil
	}
}

func newDriver(logPath string, cfg *CreateConfig) (logger.Logger, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, errdefs.InvalidParameter(err)
	}

	lf, err := loggerutils.NewLogFile(logPath, cfg.MaxFileSize, cfg.MaxFileCount, !cfg.DisableCompression, makeMarshaller(), decodeFunc, 0640, getTailReader)
	if err != nil {
		return nil, err
	}
	return &driver{
		logfile: lf,
		readers: make(map[*logger.LogWatcher]struct{}),
	}, nil
}

func (d *driver) Name() string {
	return Name
}

func (d *driver) Log(msg *logger.Message) error {
	d.mu.Lock()
	err := d.logfile.WriteLogEntry(msg)
	d.mu.Unlock()
	return err
}

func (d *driver) Close() error {
	d.mu.Lock()
	d.closed = true
	err := d.logfile.Close()
	for r := range d.readers {
		r.ProducerGone()
		delete(d.readers, r)
	}
	d.mu.Unlock()
	return err
}

func messageToProto(msg *logger.Message, proto *logdriver.LogEntry, partial *logdriver.PartialLogEntryMetadata) {
	proto.Source = msg.Source
	proto.TimeNano = msg.Timestamp.UnixNano()
	proto.Line = append(proto.Line[:0], msg.Line...)
	proto.Partial = msg.PLogMetaData != nil
	if proto.Partial {
		partial.Ordinal = int32(msg.PLogMetaData.Ordinal)
		partial.Last = msg.PLogMetaData.Last
		partial.Id = msg.PLogMetaData.ID
		proto.PartialLogMetadata = partial
	} else {
		proto.PartialLogMetadata = nil
	}
}

func protoToMessage(proto *logdriver.LogEntry) *logger.Message {
	msg := &logger.Message{
		Source:    proto.Source,
		Timestamp: time.Unix(0, proto.TimeNano),
	}
	if proto.Partial {
		var md backend.PartialLogMetaData
		md.Last = proto.GetPartialLogMetadata().GetLast()
		md.ID = proto.GetPartialLogMetadata().GetId()
		md.Ordinal = int(proto.GetPartialLogMetadata().GetOrdinal())
		msg.PLogMetaData = &md
	}
	msg.Line = append(msg.Line[:0], proto.Line...)
	return msg
}

func resetProto(proto *logdriver.LogEntry) {
	proto.Source = ""
	proto.Line = proto.Line[:0]
	proto.TimeNano = 0
	proto.Partial = false
	if proto.PartialLogMetadata != nil {
		proto.PartialLogMetadata.Id = ""
		proto.PartialLogMetadata.Last = false
		proto.PartialLogMetadata.Ordinal = 0
	}
	proto.PartialLogMetadata = nil
}
