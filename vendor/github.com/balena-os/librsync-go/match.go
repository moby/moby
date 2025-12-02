package librsync

import (
	"encoding/binary"
	"fmt"
	"io"
)

type matchKind uint8

const (
	MATCH_KIND_LITERAL matchKind = iota
	MATCH_KIND_COPY
)

// Size of the output buffer in bytes. We'll flush the match once it gets this
// large. As consequence, this is the maximum size of a LITERAL command we'll
// generate on our deltas.
const OUTPUT_BUFFER_SIZE = 16 * 1024 * 1024

type match struct {
	kind   matchKind
	pos    uint64
	len    uint64
	output io.Writer
	lit    []byte
}

func intSize(d uint64) uint8 {
	switch {
	case d == uint64(uint8(d)):
		return 1
	case d == uint64(uint16(d)):
		return 2
	case d == uint64(uint32(d)):
		return 4
	default:
		return 8
	}
}

func newMatch(output io.Writer, buff []byte) match {
	return match{
		output: output,
		lit:    buff,
	}
}

func (m *match) write(d uint64, size uint8) error {
	switch size {
	case 1:
		return binary.Write(m.output, binary.BigEndian, uint8(d))
	case 2:
		return binary.Write(m.output, binary.BigEndian, uint16(d))
	case 4:
		return binary.Write(m.output, binary.BigEndian, uint32(d))
	case 8:
		return binary.Write(m.output, binary.BigEndian, uint64(d))
	}
	return fmt.Errorf("Invalid size: %v", size)
}

func (m *match) flush() error {
	if m.len == 0 {
		return nil
	}
	posSize := intSize(m.pos)
	lenSize := intSize(m.len)

	var cmd Op

	switch m.kind {
	case MATCH_KIND_COPY:
		switch posSize {
		case 1:
			cmd = OP_COPY_N1_N1
		case 2:
			cmd = OP_COPY_N2_N1
		case 4:
			cmd = OP_COPY_N4_N1
		case 8:
			cmd = OP_COPY_N8_N1
		}

		switch lenSize {
		case 2:
			cmd += 1
		case 4:
			cmd += 2
		case 8:
			cmd += 3
		}

		err := binary.Write(m.output, binary.BigEndian, cmd)
		if err != nil {
			return err
		}
		err = m.write(m.pos, posSize)
		if err != nil {
			return err
		}
		err = m.write(m.len, lenSize)
		if err != nil {
			return err
		}
	case MATCH_KIND_LITERAL:
		cmd = OP_LITERAL_N1
		switch lenSize {
		case 1:
			cmd = OP_LITERAL_N1
		case 2:
			cmd = OP_LITERAL_N2
		case 4:
			cmd = OP_LITERAL_N4
		case 8:
			cmd = OP_LITERAL_N8
		}

		err := binary.Write(m.output, binary.BigEndian, cmd)
		if err != nil {
			return err
		}
		err = m.write(m.len, lenSize)
		if err != nil {
			return err
		}
		_, err = m.output.Write(m.lit)
		if err != nil {
			return err
		}
		m.lit = m.lit[:0] // reuse the same buffer
	}
	m.pos = 0
	m.len = 0
	return nil
}

func (m *match) add(kind matchKind, pos, len uint64) error {
	if len != 0 && m.kind != kind {
		err := m.flush()
		if err != nil {
			return err
		}
	}

	m.kind = kind

	switch kind {
	case MATCH_KIND_LITERAL:
		m.lit = append(m.lit, byte(pos))
		m.len += 1
		if m.len >= OUTPUT_BUFFER_SIZE {
			err := m.flush()
			if err != nil {
				return err
			}
		}
	case MATCH_KIND_COPY:
		if m.pos+m.len != pos {
			err := m.flush()
			if err != nil {
				return err
			}
			m.pos = pos
			m.len = len
		} else {
			m.len += len
		}
	}
	return nil
}
