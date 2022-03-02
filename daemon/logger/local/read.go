package local

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/plugins/logdriver"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/loggerutils"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
)

// maxMsgLen is the maximum size of the logger.Message after serialization.
// logger.defaultBufSize caps the size of Line field.
const maxMsgLen int = 1e6 // 1MB.

func (d *driver) ReadLogs(config logger.ReadConfig) *logger.LogWatcher {
	return d.logfile.ReadLogs(config)
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
	// buf keeps bytes from rdr.
	buf []byte
	// offset is the position in buf.
	// If offset > 0, buf[offset:] has bytes which are read but haven't used.
	offset int
	// nextMsgLen is the length of the next log message.
	// If nextMsgLen = 0, a new value must be read from rdr.
	nextMsgLen int
}

func (d *decoder) readRecord(size int) error {
	var err error
	for i := 0; i < maxDecodeRetry; i++ {
		var n int
		n, err = io.ReadFull(d.rdr, d.buf[d.offset:size])
		d.offset += n
		if err != nil {
			if err != io.ErrUnexpectedEOF {
				return err
			}
			continue
		}
		break
	}
	if err != nil {
		return err
	}
	d.offset = 0
	return nil
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

	if d.nextMsgLen == 0 {
		msgLen, err := d.decodeSizeHeader()
		if err != nil {
			return nil, err
		}

		if msgLen > maxMsgLen {
			return nil, fmt.Errorf("log message is too large (%d > %d)", msgLen, maxMsgLen)
		}

		if len(d.buf) < msgLen+encodeBinaryLen {
			d.buf = make([]byte, msgLen+encodeBinaryLen)
		} else if msgLen <= initialBufSize {
			d.buf = d.buf[:initialBufSize]
		} else {
			d.buf = d.buf[:msgLen+encodeBinaryLen]
		}

		d.nextMsgLen = msgLen
	}
	return d.decodeLogEntry()
}

func (d *decoder) Reset(rdr io.Reader) {
	if d.rdr == rdr {
		return
	}

	d.rdr = rdr
	if d.proto != nil {
		resetProto(d.proto)
	}
	if d.buf != nil {
		d.buf = d.buf[:initialBufSize]
	}
	d.offset = 0
	d.nextMsgLen = 0
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

func (d *decoder) decodeSizeHeader() (int, error) {
	err := d.readRecord(encodeBinaryLen)
	if err != nil {
		return 0, errors.Wrap(err, "could not read a size header")
	}

	msgLen := int(binary.BigEndian.Uint32(d.buf[:encodeBinaryLen]))
	return msgLen, nil
}

func (d *decoder) decodeLogEntry() (*logger.Message, error) {
	msgLen := d.nextMsgLen
	err := d.readRecord(msgLen + encodeBinaryLen)
	if err != nil {
		return nil, errors.Wrapf(err, "could not read a log entry (size=%d+%d)", msgLen, encodeBinaryLen)
	}
	d.nextMsgLen = 0

	if err := d.proto.Unmarshal(d.buf[:msgLen]); err != nil {
		return nil, errors.Wrapf(err, "error unmarshalling log entry (size=%d)", msgLen)
	}

	msg := protoToMessage(d.proto)
	if msg.PLogMetaData == nil {
		msg.Line = append(msg.Line, '\n')
	}

	return msg, nil
}
