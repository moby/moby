package filedescriptors

import (
	"testing"
)

func BenchmarkGetTotalUsedFds(b *testing.B) {
	ctx := b.Context()
	b.ReportAllocs()
	for b.Loop() {
		_ = GetTotalUsedFds(ctx)
	}
}
