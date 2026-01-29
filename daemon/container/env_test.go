package container

import (
	"crypto/rand"
	"testing"

	"gotest.tools/v3/assert"
)

func TestReplaceAndAppendEnvVars(t *testing.T) {
	origEnv := []string{"HOME=/", "FOO=foo_default"}
	// remove FOO from env
	// remove BAR from env (nop)
	overrides := []string{"HOME=/root", "TERM=xterm", "FOO", "BAR"}

	env := ReplaceOrAppendEnvValues(origEnv, overrides)
	expected := []string{"HOME=/root", "TERM=xterm"}
	assert.DeepEqual(t, env, expected)
}

func BenchmarkReplaceOrAppendEnvValues(b *testing.B) {
	b.Run("0", func(b *testing.B) {
		benchmarkReplaceOrAppendEnvValues(b, 0)
	})
	b.Run("100", func(b *testing.B) {
		benchmarkReplaceOrAppendEnvValues(b, 100)
	})
	b.Run("1000", func(b *testing.B) {
		benchmarkReplaceOrAppendEnvValues(b, 1000)
	})
	b.Run("10000", func(b *testing.B) {
		benchmarkReplaceOrAppendEnvValues(b, 10000)
	})
}

func benchmarkReplaceOrAppendEnvValues(b *testing.B, extraEnv int) {
	b.StopTimer()
	// remove FOO from env
	// remove BAR from env (nop)
	o := []string{"HOME=/root", "TERM=xterm", "FOO", "BAR"}

	if extraEnv > 0 {
		buf := make([]byte, 5)
		for range extraEnv {
			n, err := rand.Read(buf)
			assert.NilError(b, err)
			key := string(buf[:n])

			n, err = rand.Read(buf)
			assert.NilError(b, err)
			val := string(buf[:n])

			o = append(o, key+"="+val)
		}
	}
	d := make([]string, 0, len(o)+2)
	d = append(d, []string{"HOME=/", "FOO=foo_default"}...)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_ = ReplaceOrAppendEnvValues(d, o)
	}
}
