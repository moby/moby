package archive

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
	"testing"
)

func TestHardLinkOrder(t *testing.T) {
	names := []string{"file1.txt", "file2.txt", "file3.txt"}
	msg := []byte("Hey y'all")

	// Create dir
	src, err := ioutil.TempDir("", "docker-hardlink-test-src-")
	if err != nil {
		t.Fatal(err)
	}
	//defer os.RemoveAll(src)
	for _, name := range names {
		func() {
			fh, err := os.Create(path.Join(src, name))
			if err != nil {
				t.Fatal(err)
			}
			defer fh.Close()
			if _, err = fh.Write(msg); err != nil {
				t.Fatal(err)
			}
		}()
	}
	// Create dest, with changes that includes hardlinks
	dest, err := ioutil.TempDir("", "docker-hardlink-test-dest-")
	if err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(dest) // we just want the name, at first
	if err := copyDir(src, dest); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dest)
	for _, name := range names {
		for i := 0; i < 5; i++ {
			if err := os.Link(path.Join(dest, name), path.Join(dest, fmt.Sprintf("%s.link%d", name, i))); err != nil {
				t.Fatal(err)
			}
		}
	}

	// get changes
	changes, err := ChangesDirs(dest, src)
	if err != nil {
		t.Fatal(err)
	}

	// sort
	sort.Sort(changesByPath(changes))

	// ExportChanges
	ar, err := ExportChanges(dest, changes)
	if err != nil {
		t.Fatal(err)
	}
	hdrs, err := walkHeaders(ar)
	if err != nil {
		t.Fatal(err)
	}

	// reverse sort
	sort.Sort(sort.Reverse(changesByPath(changes)))
	// ExportChanges
	arRev, err := ExportChanges(dest, changes)
	if err != nil {
		t.Fatal(err)
	}
	hdrsRev, err := walkHeaders(arRev)
	if err != nil {
		t.Fatal(err)
	}

	// line up the two sets
	sort.Sort(tarHeaders(hdrs))
	sort.Sort(tarHeaders(hdrsRev))

	// compare Size and LinkName
	for i := range hdrs {
		if hdrs[i].Name != hdrsRev[i].Name {
			t.Errorf("headers - expected name %q; but got %q", hdrs[i].Name, hdrsRev[i].Name)
		}
		if hdrs[i].Size != hdrsRev[i].Size {
			t.Errorf("headers - %q expected size %d; but got %d", hdrs[i].Name, hdrs[i].Size, hdrsRev[i].Size)
		}
		if hdrs[i].Typeflag != hdrsRev[i].Typeflag {
			t.Errorf("headers - %q expected type %d; but got %d", hdrs[i].Name, hdrs[i].Typeflag, hdrsRev[i].Typeflag)
		}
		if hdrs[i].Linkname != hdrsRev[i].Linkname {
			t.Errorf("headers - %q expected linkname %q; but got %q", hdrs[i].Name, hdrs[i].Linkname, hdrsRev[i].Linkname)
		}
	}

}

func TestExportChanges(t *testing.T) {
	src, err := ioutil.TempDir("", "docker-changes-test")
	defer os.RemoveAll(src)
	dir := path.Join(src, "dir")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}

	names := []string{"file1.txt", "file2.txt", "dir/file3.txt"}
	msg := []byte("orig")

	for _, name := range names {
		if err = ioutil.WriteFile(path.Join(src, name), msg, 0744); err != nil {
			t.Fatal(err)
		}
	}

	// Create dest, with changes
	dest, err := ioutil.TempDir("", "docker-changes-test-dest-")
	if err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(dest) // we just want the name, at first
	if err := copyDir(src, dest); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dest)

	// mock some changes
	if err := ioutil.WriteFile(path.Join(dest, "file2.txt"), []byte("new"), 0744); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(path.Join(dest, "dir/file3.txt"), []byte("new"), 0744); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(path.Join(dest, "file4.txt"), []byte("new"), 0744); err != nil {
		t.Fatal(err)
	}

	// get changes
	changes, err := ChangesDirs(dest, src)
	if err != nil {
		t.Fatal(err)
	}

	// export changes
	ar, err := ExportChanges(dest, changes)
	if err != nil {
		t.Fatal(err)
	}
	hdrs, err := walkHeaders(ar)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != len(hdrs) {
		t.Fatalf("changes length mismatch. expected %d, got %d", len(changes), len(hdrs))
	}

	// exclude files
	excludes := []string{"/file*.txt"}
	arEx, err := ExportChangesExcludes(dest, changes, excludes)
	hdrs, err = walkHeaders(arEx)
	if err != nil {
		t.Fatal(err)
	}
	for _, hdr := range hdrs {
		if hdr.Name == "file1.txt" || hdr.Name == "file4.txt" {
			t.Fatalf("ExportChangesExcludes - file %s is expected to be excluded, but it's not", hdr.Name)
		}
	}

	// exclude a directory
	excludes = []string{"/dir"}
	arEx, err = ExportChangesExcludes(dest, changes, excludes)
	hdrs, err = walkHeaders(arEx)
	if err != nil {
		t.Fatal(err)
	}
	for _, hdr := range hdrs {
		if strings.HasPrefix(hdr.Name, "dir") {
			t.Fatalf("ExportChangesExcludes - file %s is expected to be excluded, but it's not", hdr.Name)
		}
	}
}

type tarHeaders []tar.Header

func (th tarHeaders) Len() int           { return len(th) }
func (th tarHeaders) Swap(i, j int)      { th[j], th[i] = th[i], th[j] }
func (th tarHeaders) Less(i, j int) bool { return th[i].Name < th[j].Name }

func walkHeaders(r io.Reader) ([]tar.Header, error) {
	t := tar.NewReader(r)
	headers := []tar.Header{}
	for {
		hdr, err := t.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return headers, err
		}
		headers = append(headers, *hdr)
	}
	return headers, nil
}
