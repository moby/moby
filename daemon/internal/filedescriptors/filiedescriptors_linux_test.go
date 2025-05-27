package filedescriptors

import (
	"context"
	"testing"
)

func BenchmarkGetTotalUsedFds(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = GetTotalUsedFds(ctx)
	}
}
