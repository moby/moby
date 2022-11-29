package local // import "github.com/docker/docker/daemon/logger/local"

import (
	"encoding/binary"
	"io"
	"math/bits"
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

var buffersPool = sync.Pool{New: func() interface{} {
	b := make([]byte, initialBufSize)
	return &b
}}

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
	logfile *loggerutils.LogFile
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

func marshal(m *logger.Message, buffer *[]byte) error {
	proto := logdriver.LogEntry{}
	md := logdriver.PartialLogEntryMetadata{}

	resetProto(&proto)

	messageToProto(m, &proto, &md)
	protoSize := proto.Size()
	writeLen := protoSize + (2 * encodeBinaryLen) // + len(messageDelimiter)

	buf := *buffer
	if writeLen > cap(buf) {
		// If we already need to reallocate the buffer, make it larger to be more reusable.
		// Round to the next power of two.
		capacity := 1 << (bits.Len(uint(writeLen)) + 1)

		buf = make([]byte, writeLen, capacity)
	} else {
		buf = buf[:writeLen]
	}
	*buffer = buf

	binary.BigEndian.PutUint32(buf[:encodeBinaryLen], uint32(protoSize))
	n, err := proto.MarshalTo(buf[encodeBinaryLen:writeLen])
	if err != nil {
		return errors.Wrap(err, "error marshaling log entry")
	}
	if n+(encodeBinaryLen*2) != writeLen {
		return io.ErrShortWrite
	}
	binary.BigEndian.PutUint32(buf[writeLen-encodeBinaryLen:writeLen], uint32(protoSize))
	return nil
}

func newDriver(logPath string, cfg *CreateConfig) (logger.Logger, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, errdefs.InvalidParameter(err)
	}

	lf, err := loggerutils.NewLogFile(logPath, cfg.MaxFileSize, cfg.MaxFileCount, !cfg.DisableCompression, decodeFunc, 0o640, getTailReader)
	if err != nil {
		return nil, err
	}
	return &driver{
		logfile: lf,
	}, nil
}

func (d *driver) Name() string {
	return Name
}

func (d *driver) Log(msg *logger.Message) error {
	buf := buffersPool.Get().(*[]byte)
	defer buffersPool.Put(buf)

	timestamp := msg.Timestamp
	err := marshal(msg, buf)
	logger.PutMessage(msg)

	if err != nil {
		return errors.Wrap(err, "error marshalling logger.Message")
	}
	return d.logfile.WriteLogEntry(timestamp, *buf)
}

func (d *driver) Close() error {
	return d.logfile.Close()
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
