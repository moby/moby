package archive

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"testing"
	"time"
)

func TestCmdStreamLargeStderr(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "dd if=/dev/zero bs=1k count=1000 of=/dev/stderr; echo hello")
	out, err := CmdStream(cmd, nil, nil)
	if err != nil {
		t.Fatalf("Failed to start command: %s", err)
	}
	errCh := make(chan error)
	go func() {
		_, err := io.Copy(ioutil.Discard, out)
		errCh <- err
	}()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Command should not have failed (err=%.100s...)", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("Command did not complete in 5 seconds; probable deadlock")
	}
}

func TestCmdStreamBad(t *testing.T) {
	badCmd := exec.Command("/bin/sh", "-c", "echo hello; echo >&2 error couldn\\'t reverse the phase pulser; exit 1")
	out, err := CmdStream(badCmd, nil, nil)
	if err != nil {
		t.Fatalf("Failed to start command: %s", err)
	}
	if output, err := ioutil.ReadAll(out); err == nil {
		t.Fatalf("Command should have failed")
	} else if err.Error() != "exit status 1: error couldn't reverse the phase pulser\n" {
		t.Fatalf("Wrong error value (%s)", err)
	} else if s := string(output); s != "hello\n" {
		t.Fatalf("Command output should be '%s', not '%s'", "hello\\n", output)
	}
}

func TestCmdStreamGood(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "echo hello; exit 0")
	out, err := CmdStream(cmd, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if output, err := ioutil.ReadAll(out); err != nil {
		t.Fatalf("Command should not have failed (err=%s)", err)
	} else if s := string(output); s != "hello\n" {
		t.Fatalf("Command output should be '%s', not '%s'", "hello\\n", output)
	}
}

func tarUntar(t *testing.T, origin string, compression Compression) error {
	archive, err := Tar(origin, compression)
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 10)
	if _, err := archive.Read(buf); err != nil {
		return err
	}
	archive = io.MultiReader(bytes.NewReader(buf), archive)

	detectedCompression := DetectCompression(buf)
	if detectedCompression.Extension() != compression.Extension() {
		return fmt.Errorf("Wrong compression detected. Actual compression: %s, found %s", compression.Extension(), detectedCompression.Extension())
	}

	tmp, err := ioutil.TempDir("", "docker-test-untar")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := Untar(archive, tmp, nil); err != nil {
		return err
	}
	if _, err := os.Stat(tmp); err != nil {
		return err
	}
	return nil
}

func TestTarUntar(t *testing.T) {
	origin, err := ioutil.TempDir("", "docker-test-untar-origin")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(origin)
	if err := ioutil.WriteFile(path.Join(origin, "1"), []byte("hello world"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(path.Join(origin, "2"), []byte("welcome!"), 0700); err != nil {
		t.Fatal(err)
	}

	for _, c := range []Compression{
		Uncompressed,
		Gzip,
		Bzip2,
		Xz,
	} {
		if err := tarUntar(t, origin, c); err != nil {
			t.Fatalf("Error tar/untar for compression %s: %s", c.Extension(), err)
		}
	}
}
