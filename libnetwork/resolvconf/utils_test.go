package resolvconf

import "testing"

func TestHashData(t *testing.T) {
	const expected = "sha256:4d11186aed035cc624d553e10db358492c84a7cd6b9670d92123c144930450aa"
	if actual := hashData([]byte("hash-me")); actual != expected {
		t.Fatalf("Expecting %s, got %s", expected, actual)
	}
}

func BenchmarkHashData(b *testing.B) {
	b.ReportAllocs()
	data := []byte("hash-me")
	for i := 0; i < b.N; i++ {
		_ = hashData(data)
	}
}
