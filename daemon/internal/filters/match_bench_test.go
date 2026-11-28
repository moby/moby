package filters

import (
	"strconv"
	"strings"
	"testing"
)

func BenchmarkMatch(b *testing.B) {
	for _, n := range []int{32, 256} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			args := NewArgs(
				Arg("name", "app-"),
				Arg("name", `^/k8s_`),
				Arg("name", "nope"),
			)
			sources := make([]string, n)
			for i := range sources {
				sources[i] = "/k8s_pause_" + strconv.Itoa(i) + "_" + strings.Repeat("a", 16)
			}
			b.ReportAllocs()
			b.ResetTimer()
			var matched int
			for i := 0; i < b.N; i++ {
				matched = 0
				for _, source := range sources {
					if args.Match("name", source) {
						matched++
					}
				}
			}
			if matched != n {
				b.Fatalf("matched %d, want %d", matched, n)
			}
		})
	}
}
