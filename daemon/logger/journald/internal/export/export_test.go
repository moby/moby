package export_test

import (
	"bytes"
	"testing"

	"github.com/docker/docker/daemon/logger/journald/internal/export"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
)

func TestExportSerialization(t *testing.T) {
	must := func(err error) { t.Helper(); assert.NilError(t, err) }
	var buf bytes.Buffer
	must(export.WriteField(&buf, "_TRANSPORT", "journal"))
	must(export.WriteField(&buf, "MESSAGE", "this is a single-line message.\t🚀"))
	must(export.WriteField(&buf, "EMPTY_VALUE", ""))
	must(export.WriteField(&buf, "NEWLINE", "\n"))
	must(export.WriteEndOfEntry(&buf))

	must(export.WriteField(&buf, "MESSAGE", "this is a\nmulti line\nmessage"))
	must(export.WriteField(&buf, "INVALID_UTF8", "a\x80b"))
	must(export.WriteField(&buf, "BINDATA", "\x00\x01\x02\x03"))
	must(export.WriteEndOfEntry(&buf))

	golden.Assert(t, buf.String(), "export-serialization.golden")
}
