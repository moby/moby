package mount

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
	tmp := path.Join(os.TempDir(), "mount-tests")
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

	if err := Mount(sourcePath, targetPath, "none", "bind,rw"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := Unmount(targetPath); err != nil {
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
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatal(err)
	}
}

func TestMountReadonly(t *testing.T) {
	tmp := path.Join(os.TempDir(), "mount-tests")
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

	if err := Mount(sourcePath, targetPath, "none", "bind,ro"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := Unmount(targetPath); err != nil {
			t.Fatal(err)
		}
	}()

	f, err = os.OpenFile(targetPath, os.O_RDWR, 0777)
	if err == nil {
		t.Fatal("Should not be able to open a ro file as rw")
	}
}
