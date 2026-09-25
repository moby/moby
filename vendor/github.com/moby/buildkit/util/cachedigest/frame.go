package cachedigest

import (
	"encoding/binary"

	"github.com/pkg/errors"
)

type FrameID uint32

const (
	FrameIDType FrameID = 1
	FrameIDData FrameID = 2
	FrameIDSkip FrameID = 3
)

func (f FrameID) String() string {
	switch f {
	case FrameIDType:
		return "type"
	case FrameIDData:
		return "data"
	case FrameIDSkip:
		return "skip"
	default:
		return "unknown"
	}
}

type Frame struct {
	ID   FrameID `json:"type"`
	Data []byte  `json:"data,omitempty"`
}

// encodeFrames encodes a series of frames: [frameID:uint32][len:uint32][data:len]
func encodeFrames(frames []Frame) ([]byte, error) {
	var out []byte
	for _, f := range frames {
		buf := make([]byte, 8+len(f.Data))
		binary.BigEndian.PutUint32(buf[0:4], uint32(f.ID))
		binary.BigEndian.PutUint32(buf[4:8], uint32(len(f.Data)))
		copy(buf[8:], f.Data)
		out = append(out, buf...)
	}
	return out, nil
}

// decodeFrames decodes a series of frames from data.
func decodeFrames(data []byte) ([]Frame, error) {
	var frames []Frame
	i := 0
	for i+8 <= len(data) {
		frameID := binary.BigEndian.Uint32(data[i : i+4])
		length := binary.BigEndian.Uint32(data[i+4 : i+8])
		if i+8+int(length) > len(data) {
			return nil, errors.WithStack(ErrInvalidEncoding)
		}
		frames = append(frames, Frame{
			ID:   FrameID(frameID),
			Data: data[i+8 : i+8+int(length)],
		})
		i += 8 + int(length)
	}
	if i != len(data) {
		return nil, errors.WithStack(ErrInvalidEncoding)
	}
	return frames, nil
}
