package lint

import (
	"bufio"
	"bytes"
	"io"
	"os"
)

var (
	// used by cgo before Go 1.11
	oldCgo = []byte("// Created by cgo - DO NOT EDIT")
	prefix = []byte("// Code generated ")
	suffix = []byte(" DO NOT EDIT.")
	nl     = []byte("\n")
	crnl   = []byte("\r\n")
)

func isGenerated(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	br := bufio.NewReader(f)
	for {
		s, err := br.ReadBytes('\n')
		if err != nil && err != io.EOF {
			return false
		}
		s = bytes.TrimSuffix(s, crnl)
		s = bytes.TrimSuffix(s, nl)
		if bytes.HasPrefix(s, prefix) && bytes.HasSuffix(s, suffix) {
			return true
		}
		if bytes.Equal(s, oldCgo) {
			return true
		}
		if err == io.EOF {
			break
		}
	}
	return false
}
