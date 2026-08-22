package namesgenerator

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestNameFormat(t *testing.T) {
	t.Parallel()

	name := generateName(0)
	assert.Assert(t, strings.Contains(name, "_"))
	assert.Assert(t, !strings.ContainsAny(name, "0123456789"))
	assert.Assert(t, name != "boring_wozniak")
}

func TestNameRetries(t *testing.T) {
	t.Parallel()

	name := generateName(1)
	assert.Assert(t, strings.Contains(name, "_"))
	assert.Assert(t, !strings.ContainsAny(name[:len(name)-1], "0123456789"))
	assert.Assert(t, strings.ContainsRune("0123456789", rune(name[len(name)-1])))
}

func BenchmarkGenerateName(b *testing.B) {
	b.ReportAllocs()
	var out string
	for b.Loop() {
		out = generateName(5)
	}
	b.Log("Last result:", out)
}
