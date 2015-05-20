package bridge

import (
	"bytes"
	"testing"
)

func TestResolveConfRead(t *testing.T) {
	b, err := readResolvConf()
	if err != nil {
		t.Fatalf("Failed to read resolv.conf: %v", err)
	}

	if b == nil {
		t.Fatal("Reading resolv.conf returned no content")
	}
}

func TestResolveConfReadLines(t *testing.T) {
	commentChar := []byte("#")

	b, _ := readResolvConf()
	lines := getLines(b, commentChar)
	if lines == nil {
		t.Fatal("Failed to read resolv.conf lines")
	}

	for _, line := range lines {
		if bytes.Index(line, commentChar) != -1 {
			t.Fatal("Returned comment content from resolv.conf")
		}
	}
}

func TestResolvConfNameserversAsCIDR(t *testing.T) {
	resolvConf := `# Commented line
nameserver 1.2.3.4

nameserver 5.6.7.8 # Test
`

	cidrs := getNameserversAsCIDR([]byte(resolvConf))
	if expected := 2; len(cidrs) != expected {
		t.Fatalf("Expected %d nameservers, got %d", expected, len(cidrs))
	}

	expected := []string{"1.2.3.4/32", "5.6.7.8/32"}
	for i, exp := range expected {
		if cidrs[i] != exp {
			t.Fatalf("Expected nameservers %s, got %s", exp, cidrs[i])
		}
	}
}
