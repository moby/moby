package graphdriver

import (
	"os"
	"path"
	"syscall"
	"testing"
)

func TestMountOptionsParsing(t *testing.T) {
	options := "bind,ro,size=10k"

	flag, data := parseOptions(options)

	if data != "size=10k" {
		t.Fatalf("Expected size=10 got %s", data)
	}

	expectedFlag := syscall.MS_BIND | syscall.MS_RDONLY

	if flag != expectedFlag {
		t.Fatalf("Expected %d got %d", expectedFlag, flag)
	}
}

func TestMounted(t *testing.T) {
	tmp := path.Join(os.TempDir(), "graphdriver-tests")
	if err := os.MkdirAll(tmp, 0777); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	var (
		sourcePath = path.Join(tmp, "sourcefile.txt")
		targetPath = path.Join(tmp, "targetfile.txt")
	)

	f, err := os.Create(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("hello")
	f.Close()

	f, err = os.Create(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	mount := &Mount{
		Device:  sourcePath,
		Target:  targetPath,
		Type:    "none",
		Options: "bind,ro",
	}

	if err := mount.Mount("/"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := mount.Unmount("/"); err != nil {
			t.Fatal(err)
		}
	}()

	mounted, err := Mounted(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !mounted {
		t.Fatalf("Expected %s to be mounted", targetPath)
	}
}
