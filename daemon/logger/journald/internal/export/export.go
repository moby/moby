// Package export implements a serializer for the systemd Journal Export Format
// as documented at https://systemd.io/JOURNAL_EXPORT_FORMATS/
package export // import "github.com/docker/docker/daemon/logger/journald/internal/export"

import (
	"encoding/binary"
	"fmt"
	"io"
	"unicode/utf8"
)

// Returns whether s can be serialized as a field value "as they are" without
// the special binary safe serialization.
func isSerializableAsIs(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, c := range s {
		if c < ' ' && c != '\t' {
			return false
		}
	}
	return true
}

// WriteField writes the field serialized to Journal Export format to w.
//
// The variable name must consist only of uppercase characters, numbers and
// underscores. No validation or sanitization is performed.
func WriteField(w io.Writer, variable, value string) error {
	if isSerializableAsIs(value) {
		_, err := fmt.Fprintf(w, "%s=%s\n", variable, value)
		return err
	}

	if _, err := fmt.Fprintln(w, variable); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint64(len(value))); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, value)
	return err
}

// WriteEndOfEntry terminates the journal entry.
func WriteEndOfEntry(w io.Writer) error {
	_, err := fmt.Fprintln(w)
	return err
}
