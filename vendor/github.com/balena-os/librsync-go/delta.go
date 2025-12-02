package librsync

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/balena-os/circbuf"
)

func Delta(sig *SignatureType, i io.Reader, output io.Writer) error {
	buff := make([]byte, 0, OUTPUT_BUFFER_SIZE)
	return DeltaBuff(sig, i, output, buff)
}

// DeltaBuff like Delta but allows to pass literal buffer slice.
// This is useful for efficient computation of multiple deltas.
//
// The slice shall have zero size, and capacity of OUTPUT_BUFFER_SIZE.
//
// Example of usage:
//	 var files []string
//	 var litBuff = make([]byte, 0, OUTPUT_BUFFER_SIZE)
//	 for _, file := range files {
//	   f, _ := os.Open(file)
//	   sig, _ := ReadSignatureFile(file + ".sig")
//	   delta, _ := os.OpenFile(file+".delta", os.O_CREATE|os.O_WRONLY, 0644)
//	   _ = DeltaBuff(sig, f, delta, litBuff)
//	 }
func DeltaBuff(sig *SignatureType, i io.Reader, output io.Writer, litBuff []byte) error {
	if len(litBuff) != 0 || cap(litBuff) != OUTPUT_BUFFER_SIZE {
		return fmt.Errorf("bad literal buffer")
	}

	input := bufio.NewReader(i)

	err := binary.Write(output, binary.BigEndian, DELTA_MAGIC)
	if err != nil {
		return err
	}

	prevByte := byte(0)
	m := newMatch(output, litBuff)

	weakSum := NewRollsum()
	block, _ := circbuf.NewBuffer(int64(sig.blockLen))

	for {
		in, err := input.ReadByte()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		if block.TotalWritten() > 0 {
			prevByte, err = block.Get(0)
			if err != nil {
				return err
			}
		}
		block.WriteByte(in)
		weakSum.Rollin(in)

		if weakSum.count < uint64(sig.blockLen) {
			continue
		}

		if weakSum.count > uint64(sig.blockLen) {
			err := m.add(MATCH_KIND_LITERAL, uint64(prevByte), 1)
			if err != nil {
				return err
			}
			weakSum.Rollout(prevByte)
		}

		if blockIdx, ok := sig.weak2block[weakSum.Digest()]; ok {
			strong2, _ := CalcStrongSum(block.Bytes(), sig.sigType, sig.strongLen)
			if bytes.Equal(sig.strongSigs[blockIdx], strong2) {
				weakSum.Reset()
				block.Reset()
				err := m.add(MATCH_KIND_COPY, uint64(blockIdx)*uint64(sig.blockLen), uint64(sig.blockLen))
				if err != nil {
					return err
				}
			}
		}
	}

	for _, b := range block.Bytes() {
		err := m.add(MATCH_KIND_LITERAL, uint64(b), 1)
		if err != nil {
			return err
		}
	}

	if err := m.flush(); err != nil {
		return err
	}

	return binary.Write(output, binary.BigEndian, OP_END)
}
