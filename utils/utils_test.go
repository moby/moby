package utils

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"strings"
	"testing"
)

func TestBufReader(t *testing.T) {
	reader, writer := io.Pipe()
	bufreader := NewBufReader(reader)

	// Write everything down to a Pipe
	// Usually, a pipe should block but because of the buffered reader,
	// the writes will go through
	done := make(chan bool)
	go func() {
		writer.Write([]byte("hello world"))
		writer.Close()
		done <- true
	}()

	// Drain the reader *after* everything has been written, just to verify
	// it is indeed buffering
	<-done
	output, err := ioutil.ReadAll(bufreader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output, []byte("hello world")) {
		t.Error(string(output))
	}
}

type dummyWriter struct {
	buffer      bytes.Buffer
	failOnWrite bool
}

func (dw *dummyWriter) Write(p []byte) (n int, err error) {
	if dw.failOnWrite {
		return 0, errors.New("Fake fail")
	}
	return dw.buffer.Write(p)
}

func (dw *dummyWriter) String() string {
	return dw.buffer.String()
}

func (dw *dummyWriter) Close() error {
	return nil
}

func TestWriteBroadcaster(t *testing.T) {
	writer := NewWriteBroadcaster()

	// Test 1: Both bufferA and bufferB should contain "foo"
	bufferA := &dummyWriter{}
	writer.AddWriter(bufferA, "")
	bufferB := &dummyWriter{}
	writer.AddWriter(bufferB, "")
	writer.Write([]byte("foo"))

	if bufferA.String() != "foo" {
		t.Errorf("Buffer contains %v", bufferA.String())
	}

	if bufferB.String() != "foo" {
		t.Errorf("Buffer contains %v", bufferB.String())
	}

	// Test2: bufferA and bufferB should contain "foobar",
	// while bufferC should only contain "bar"
	bufferC := &dummyWriter{}
	writer.AddWriter(bufferC, "")
	writer.Write([]byte("bar"))

	if bufferA.String() != "foobar" {
		t.Errorf("Buffer contains %v", bufferA.String())
	}

	if bufferB.String() != "foobar" {
		t.Errorf("Buffer contains %v", bufferB.String())
	}

	if bufferC.String() != "bar" {
		t.Errorf("Buffer contains %v", bufferC.String())
	}

	// Test3: Test eviction on failure
	bufferA.failOnWrite = true
	writer.Write([]byte("fail"))
	if bufferA.String() != "foobar" {
		t.Errorf("Buffer contains %v", bufferA.String())
	}
	if bufferC.String() != "barfail" {
		t.Errorf("Buffer contains %v", bufferC.String())
	}
	// Even though we reset the flag, no more writes should go in there
	bufferA.failOnWrite = false
	writer.Write([]byte("test"))
	if bufferA.String() != "foobar" {
		t.Errorf("Buffer contains %v", bufferA.String())
	}
	if bufferC.String() != "barfailtest" {
		t.Errorf("Buffer contains %v", bufferC.String())
	}

	writer.CloseWriters()
}

type devNullCloser int

func (d devNullCloser) Close() error {
	return nil
}

func (d devNullCloser) Write(buf []byte) (int, error) {
	return len(buf), nil
}

// This test checks for races. It is only useful when run with the race detector.
func TestRaceWriteBroadcaster(t *testing.T) {
	writer := NewWriteBroadcaster()
	c := make(chan bool)
	go func() {
		writer.AddWriter(devNullCloser(0), "")
		c <- true
	}()
	writer.Write([]byte("hello"))
	<-c
}

// Test the behavior of TruncIndex, an index for querying IDs from a non-conflicting prefix.
func TestTruncIndex(t *testing.T) {
	index := NewTruncIndex()
	// Get on an empty index
	if _, err := index.Get("foobar"); err == nil {
		t.Fatal("Get on an empty index should return an error")
	}

	// Spaces should be illegal in an id
	if err := index.Add("I have a space"); err == nil {
		t.Fatalf("Adding an id with ' ' should return an error")
	}

	id := "99b36c2c326ccc11e726eee6ee78a0baf166ef96"
	// Add an id
	if err := index.Add(id); err != nil {
		t.Fatal(err)
	}
	// Get a non-existing id
	assertIndexGet(t, index, "abracadabra", "", true)
	// Get the exact id
	assertIndexGet(t, index, id, id, false)
	// The first letter should match
	assertIndexGet(t, index, id[:1], id, false)
	// The first half should match
	assertIndexGet(t, index, id[:len(id)/2], id, false)
	// The second half should NOT match
	assertIndexGet(t, index, id[len(id)/2:], "", true)

	id2 := id[:6] + "blabla"
	// Add an id
	if err := index.Add(id2); err != nil {
		t.Fatal(err)
	}
	// Both exact IDs should work
	assertIndexGet(t, index, id, id, false)
	assertIndexGet(t, index, id2, id2, false)

	// 6 characters or less should conflict
	assertIndexGet(t, index, id[:6], "", true)
	assertIndexGet(t, index, id[:4], "", true)
	assertIndexGet(t, index, id[:1], "", true)

	// 7 characters should NOT conflict
	assertIndexGet(t, index, id[:7], id, false)
	assertIndexGet(t, index, id2[:7], id2, false)

	// Deleting a non-existing id should return an error
	if err := index.Delete("non-existing"); err == nil {
		t.Fatalf("Deleting a non-existing id should return an error")
	}

	// Deleting id2 should remove conflicts
	if err := index.Delete(id2); err != nil {
		t.Fatal(err)
	}
	// id2 should no longer work
	assertIndexGet(t, index, id2, "", true)
	assertIndexGet(t, index, id2[:7], "", true)
	assertIndexGet(t, index, id2[:11], "", true)

	// conflicts between id and id2 should be gone
	assertIndexGet(t, index, id[:6], id, false)
	assertIndexGet(t, index, id[:4], id, false)
	assertIndexGet(t, index, id[:1], id, false)

	// non-conflicting substrings should still not conflict
	assertIndexGet(t, index, id[:7], id, false)
	assertIndexGet(t, index, id[:15], id, false)
	assertIndexGet(t, index, id, id, false)
}

func assertIndexGet(t *testing.T, index *TruncIndex, input, expectedResult string, expectError bool) {
	if result, err := index.Get(input); err != nil && !expectError {
		t.Fatalf("Unexpected error getting '%s': %s", input, err)
	} else if err == nil && expectError {
		t.Fatalf("Getting '%s' should return an error", input)
	} else if result != expectedResult {
		t.Fatalf("Getting '%s' returned '%s' instead of '%s'", input, result, expectedResult)
	}
}

func assertKernelVersion(t *testing.T, a, b *KernelVersionInfo, result int) {
	if r := CompareKernelVersion(a, b); r != result {
		t.Fatalf("Unepected kernel version comparaison result. Found %d, expected %d", r, result)
	}
}

func TestCompareKernelVersion(t *testing.T) {
	assertKernelVersion(t,
		&KernelVersionInfo{Kernel: 3, Major: 8, Minor: 0},
		&KernelVersionInfo{Kernel: 3, Major: 8, Minor: 0},
		0)
	assertKernelVersion(t,
		&KernelVersionInfo{Kernel: 2, Major: 6, Minor: 0},
		&KernelVersionInfo{Kernel: 3, Major: 8, Minor: 0},
		-1)
	assertKernelVersion(t,
		&KernelVersionInfo{Kernel: 3, Major: 8, Minor: 0},
		&KernelVersionInfo{Kernel: 2, Major: 6, Minor: 0},
		1)
	assertKernelVersion(t,
		&KernelVersionInfo{Kernel: 3, Major: 8, Minor: 0, Flavor: "0"},
		&KernelVersionInfo{Kernel: 3, Major: 8, Minor: 0, Flavor: "16"},
		0)
	assertKernelVersion(t,
		&KernelVersionInfo{Kernel: 3, Major: 8, Minor: 5},
		&KernelVersionInfo{Kernel: 3, Major: 8, Minor: 0},
		1)
	assertKernelVersion(t,
		&KernelVersionInfo{Kernel: 3, Major: 0, Minor: 20, Flavor: "25"},
		&KernelVersionInfo{Kernel: 3, Major: 8, Minor: 0, Flavor: "0"},
		-1)
}

func TestHumanSize(t *testing.T) {

	size := strings.Trim(HumanSize(1000), " \t")
	expect := "1 kB"
	if size != expect {
		t.Errorf("1000 -> expected '%s', got '%s'", expect, size)
	}

	size = strings.Trim(HumanSize(1024), " \t")
	expect = "1.024 kB"
	if size != expect {
		t.Errorf("1024 -> expected '%s', got '%s'", expect, size)
	}
}

func TestParseHost(t *testing.T) {
	if addr := ParseHost("127.0.0.1", 4243, "0.0.0.0"); addr != "tcp://0.0.0.0:4243" {
		t.Errorf("0.0.0.0 -> expected tcp://0.0.0.0:4243, got %s", addr)
	}
	if addr := ParseHost("127.0.0.1", 4243, "0.0.0.1:5555"); addr != "tcp://0.0.0.1:5555" {
		t.Errorf("0.0.0.1:5555 -> expected tcp://0.0.0.1:5555, got %s", addr)
	}
	if addr := ParseHost("127.0.0.1", 4243, ":6666"); addr != "tcp://127.0.0.1:6666" {
		t.Errorf(":6666 -> expected tcp://127.0.0.1:6666, got %s", addr)
	}
	if addr := ParseHost("127.0.0.1", 4243, "tcp://:7777"); addr != "tcp://127.0.0.1:7777" {
		t.Errorf("tcp://:7777 -> expected tcp://127.0.0.1:7777, got %s", addr)
	}
	if addr := ParseHost("127.0.0.1", 4243, "unix:///var/run/docker.sock"); addr != "unix:///var/run/docker.sock" {
		t.Errorf("unix:///var/run/docker.sock -> expected unix:///var/run/docker.sock, got %s", addr)
	}
}

func TestParseRepositoryTag(t *testing.T) {
	if repo, tag := ParseRepositoryTag("root"); repo != "root" || tag != "" {
		t.Errorf("Expected repo: '%s' and tag: '%s', got '%s' and '%s'", "root", "", repo, tag)
	}
	if repo, tag := ParseRepositoryTag("root:tag"); repo != "root" || tag != "tag" {
		t.Errorf("Expected repo: '%s' and tag: '%s', got '%s' and '%s'", "root", "tag", repo, tag)
	}
	if repo, tag := ParseRepositoryTag("user/repo"); repo != "user/repo" || tag != "" {
		t.Errorf("Expected repo: '%s' and tag: '%s', got '%s' and '%s'", "user/repo", "", repo, tag)
	}
	if repo, tag := ParseRepositoryTag("user/repo:tag"); repo != "user/repo" || tag != "tag" {
		t.Errorf("Expected repo: '%s' and tag: '%s', got '%s' and '%s'", "user/repo", "tag", repo, tag)
	}
	if repo, tag := ParseRepositoryTag("url:5000/repo"); repo != "url:5000/repo" || tag != "" {
		t.Errorf("Expected repo: '%s' and tag: '%s', got '%s' and '%s'", "url:5000/repo", "", repo, tag)
	}
	if repo, tag := ParseRepositoryTag("url:5000/repo:tag"); repo != "url:5000/repo" || tag != "tag" {
		t.Errorf("Expected repo: '%s' and tag: '%s', got '%s' and '%s'", "url:5000/repo", "tag", repo, tag)
	}
}

func TestGetResolvConf(t *testing.T) {
	resolvConfUtils, err := GetResolvConf()
	if err != nil {
		t.Fatal(err)
	}
	resolvConfSystem, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		t.Fatal(err)
	}
	if string(resolvConfUtils) != string(resolvConfSystem) {
		t.Fatalf("/etc/resolv.conf and GetResolvConf have different content.")
	}
}

func TestCheclLocalDns(t *testing.T) {
	for resolv, result := range map[string]bool{`# Dynamic
nameserver 10.0.2.3
search dotcloud.net`: false,
		`# Dynamic
nameserver 127.0.0.1
search dotcloud.net`: true,
		`# Dynamic
nameserver 127.0.1.1
search dotcloud.net`: true,
		`# Dynamic
`: true,
		``: true,
	} {
		if CheckLocalDns([]byte(resolv)) != result {
			t.Fatalf("Wrong local dns detection: {%s} should be %v", resolv, result)
		}
	}
}
