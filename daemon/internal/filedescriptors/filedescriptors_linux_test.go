package filedescriptors

import (
	"context"
	"testing"
)

func BenchmarkGetTotalUsedFds(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		_ = GetTotalUsedFds(ctx)
	}
}
