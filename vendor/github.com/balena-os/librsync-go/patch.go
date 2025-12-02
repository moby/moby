package librsync

import (
	"encoding/binary"
	"fmt"
	"io"
)

type MagicNumber uint32

const (
	DELTA_MAGIC MagicNumber = 0x72730236

	// A signature file with MD4 signatures.
	//
	// Backward compatible with librsync < 1.0, but strongly deprecated because
	// it creates a security vulnerability on files containing partly untrusted
	// data. See <https://github.com/librsync/librsync/issues/5>.
	MD4_SIG_MAGIC MagicNumber = 0x72730136

	// A signature file using the BLAKE2 hash. Supported from librsync 1.0.
	BLAKE2_SIG_MAGIC MagicNumber = 0x72730137
)

func readParam(r io.Reader, size uint8) int64 {
	switch size {
	case 1:
		var tmp uint8
		binary.Read(r, binary.BigEndian, &tmp)
		return int64(tmp)
	case 2:
		var tmp uint16
		binary.Read(r, binary.BigEndian, &tmp)
		return int64(tmp)
	case 4:
		var tmp uint32
		binary.Read(r, binary.BigEndian, &tmp)
		return int64(tmp)
	case 8:
		var tmp int64
		binary.Read(r, binary.BigEndian, &tmp)
		return int64(tmp)
	}
	return 0
}

func Patch(base io.ReadSeeker, delta io.Reader, out io.Writer) error {
	var magic MagicNumber

	err := binary.Read(delta, binary.BigEndian, &magic)
	if err != nil {
		return err
	}

	if magic != DELTA_MAGIC {
		return fmt.Errorf("Got magic number %x rather than expected value %x", magic, DELTA_MAGIC)
	}

	for {
		var op Op
		err := binary.Read(delta, binary.BigEndian, &op)
		if err != nil {
			return err
		}
		cmd := op2cmd[op]

		var param1, param2 int64

		if cmd.Len1 == 0 {
			param1 = int64(cmd.Immediate)
		} else {
			param1 = readParam(delta, cmd.Len1)
			param2 = readParam(delta, cmd.Len2)
		}

		switch cmd.Kind {
		default:
			return fmt.Errorf("Bogus command %x", cmd.Kind)
		case KIND_LITERAL:
			io.CopyN(out, delta, param1)
		case KIND_COPY:
			base.Seek(param1, io.SeekStart)
			io.CopyN(out, base, param2)
		case KIND_END:
			return nil
		}
	}
}
