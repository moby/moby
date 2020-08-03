package local

import (
	"context"
	"encoding/binary"
	"io"

	"bytes"

	"github.com/docker/docker/api/types/plugins/logdriver"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/loggerutils"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
)

func (d *driver) ReadLogs(config logger.ReadConfig) *logger.LogWatcher {
	logWatcher := logger.NewLogWatcher()

	go d.readLogs(logWatcher, config)
	return logWatcher
}

func (d *driver) readLogs(watcher *logger.LogWatcher, config logger.ReadConfig) {
	defer close(watcher.Msg)

	d.mu.Lock()
	d.readers[watcher] = struct{}{}
	d.mu.Unlock()

	d.logfile.ReadLogs(config, watcher)

	d.mu.Lock()
	delete(d.readers, watcher)
	d.mu.Unlock()
}

func getTailReader(ctx context.Context, r loggerutils.SizeReaderAt, req int) (io.Reader, int, error) {
	size := r.Size()
	if req < 0 {
		return nil, 0, errdefs.InvalidParameter(errors.Errorf("invalid number of lines to tail: %d", req))
	}

	if size < (encodeBinaryLen*2)+1 {
		return bytes.NewReader(nil), 0, nil
	}

	const encodeBinaryLen64 = int64(encodeBinaryLen)
	var found int

	buf := make([]byte, encodeBinaryLen)

	offset := size
	for {
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		default:
		}

		n, err := r.ReadAt(buf, offset-encodeBinaryLen64)
		if err != nil && err != io.EOF {
			return nil, 0, errors.Wrap(err, "error reading log message footer")
		}

		if n != encodeBinaryLen {
			return nil, 0, errdefs.DataLoss(errors.New("unexpected number of bytes read from log message footer"))
		}

		msgLen := binary.BigEndian.Uint32(buf)

		n, err = r.ReadAt(buf, offset-encodeBinaryLen64-encodeBinaryLen64-int64(msgLen))
		if err != nil && err != io.EOF {
			return nil, 0, errors.Wrap(err, "error reading log message header")
		}

		if n != encodeBinaryLen {
			return nil, 0, errdefs.DataLoss(errors.New("unexpected number of bytes read from log message header"))
		}

		if msgLen != binary.BigEndian.Uint32(buf) {
			return nil, 0, errdefs.DataLoss(errors.Wrap(err, "log message header and footer indicate different message sizes"))
		}

		found++
		offset -= int64(msgLen)
		offset -= encodeBinaryLen64 * 2
		if found == req {
			break
		}
		if offset <= 0 {
			break
		}
	}

	return io.NewSectionReader(r, offset, size), found, nil
}

type decoder struct {
	rdr   io.Reader
	proto *logdriver.LogEntry
	buf   []byte
}

func (d *decoder) Decode() (*logger.Message, error) {
	if d.proto == nil {
		d.proto = &logdriver.LogEntry{}
	} else {
		resetProto(d.proto)
	}
	if d.buf == nil {
		d.buf = make([]byte, initialBufSize)
	}
	var (
		read int
		err  error
	)

	for i := 0; i < maxDecodeRetry; i++ {
		var n int
		n, err = io.ReadFull(d.rdr, d.buf[read:encodeBinaryLen])
		if err != nil {
			if err != io.ErrUnexpectedEOF {
				return nil, errors.Wrap(err, "error reading log message length")
			}
			read += n
			continue
		}
		read += n
		break
	}
	if err != nil {
		return nil, errors.Wrapf(err, "could not read log message length: read: %d, expected: %d", read, encodeBinaryLen)
	}

	msgLen := int(binary.BigEndian.Uint32(d.buf[:read]))

	if len(d.buf) < msgLen+encodeBinaryLen {
		d.buf = make([]byte, msgLen+encodeBinaryLen)
	} else {
		if msgLen <= initialBufSize {
			d.buf = d.buf[:initialBufSize]
		} else {
			d.buf = d.buf[:msgLen+encodeBinaryLen]
		}
	}

	return decodeLogEntry(d.rdr, d.proto, d.buf, msgLen)
}

func (d *decoder) Reset(rdr io.Reader) {
	d.rdr = rdr
	if d.proto != nil {
		resetProto(d.proto)
	}
	if d.buf != nil {
		d.buf = d.buf[:initialBufSize]
	}
}

func (d *decoder) Close() {
	d.buf = d.buf[:0]
	d.buf = nil
	if d.proto != nil {
		resetProto(d.proto)
	}
	d.rdr = nil
}

func decodeFunc(rdr io.Reader) loggerutils.Decoder {
	return &decoder{rdr: rdr}
}

func decodeLogEntry(rdr io.Reader, proto *logdriver.LogEntry, buf []byte, msgLen int) (*logger.Message, error) {
	var (
		read int
		err  error
	)
	for i := 0; i < maxDecodeRetry; i++ {
		var n int
		n, err = io.ReadFull(rdr, buf[read:msgLen+encodeBinaryLen])
		if err != nil {
			if err != io.ErrUnexpectedEOF {
				return nil, errors.Wrap(err, "could not decode log entry")
			}
			read += n
			continue
		}
		break
	}
	if err != nil {
		return nil, errors.Wrapf(err, "could not decode entry: read %d, expected: %d", read, msgLen)
	}

	if err := proto.Unmarshal(buf[:msgLen]); err != nil {
		return nil, errors.Wrap(err, "error unmarshalling log entry")
	}

	msg := protoToMessage(proto)
	if msg.PLogMetaData == nil {
		msg.Line = append(msg.Line, '\n')
	}
	return msg, nil
}
