package dockerignore

import (
	"fmt"
	"github.com/docker/docker/pkg/precompiledregexp"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestReadAll(t *testing.T) {
	tmpDir := createTmpDir(t, "", "dockerignore-test")
	defer os.RemoveAll(tmpDir)

	di, err := ReadAll(nil)
	if err != nil {
		t.Fatalf("Expected not to have error, got %v", err)
	}

	if diLen := len(di); diLen != 0 {
		t.Fatalf("Expected to have zero dockerignore entry, got %d", diLen)
	}

	diName := filepath.Join(tmpDir, ".dockerignore")
	content := fmt.Sprintf("test1\n/test2\n/a/file/here\n\nlastfile")
	err = ioutil.WriteFile(diName, []byte(content), 0777)
	if err != nil {
		t.Fatal(err)
	}

	diFd, err := os.Open(diName)
	if err != nil {
		t.Fatal(err)
	}
	di, err = ReadAll(diFd)
	if err != nil {
		t.Fatal(err)
	}

	if len(di) != 4 {
		t.Fatalf("Expected to have zero dockerignore entry, got %d", len(di))
	}

	if di[0] != "test1" {
		t.Fatalf("First element is not test1")
	}
	if di[1] != "/test2" {
		t.Fatalf("Second element is not /test2")
	}
	if di[2] != "/a/file/here" {
		t.Fatalf("Third element is not /a/file/here")
	}
	if di[3] != "lastfile" {
		t.Fatalf("Fourth element is not lastfile")
	}
}

func TestReadAllRecursiveEmpty(t *testing.T) {
	/*
		Test Directory Structure:
		> dir0_0C
		>> .dockerignore
		>> subdir1_1E
		>>> subdir2_1C
		>>>> .dockerignore
		>> subdir1_2C
		>>> .dockerignore
		>> subdir1_3E
	*/
	dir0_0C := createTmpDir(t, "", "dockerignore-test")
	subdir1_1E := createTmpDir(t, dir0_0C, "subdir1_1E")
	subdir2_1C := createTmpDir(t, subdir1_1E, "subdir2_1C")
	subdir1_2C := createTmpDir(t, dir0_0C, "subdir1_2C")
	ioutil.TempDir(dir0_0C, "subdir1_3E")
	defer os.RemoveAll(dir0_0C)

	// Write content to files
	content := fmt.Sprintf("0_0File1\n/0_0File2\n!/dir/subdir/0_0file\n\n0_0lastfile\n!")
	content2 := fmt.Sprintf("2_1File1\n/2_1File2\n!/dir/subdir/2_1file\n\n2_1lastfile\n!")
	content3 := fmt.Sprintf("1_2File1\n/1_2File2\n!/dir/subdir/1_2file\n\n1_2lastfile\n!")

	writeFileHelper(t, filepath.Join(dir0_0C, ".dockerignore"), []byte(content), 0777)
	writeFileHelper(t, filepath.Join(subdir2_1C, ".dockerignore"), []byte(content2), 0777)
	writeFileHelper(t, filepath.Join(subdir1_2C, ".dockerignore"), []byte(content3), 0777)

	// Read from current directory; should give nothing - everything is in /tmp
	di, err := ReadAllRecursive("", "")
	if err != nil {
		t.Fatalf("Expected not to have error, got %v", err)
	}

	if diLen := len(di); diLen != 0 {
		t.Fatalf("Expected to have zero dockerignore entry, got %d", diLen)
	}
}
func TestReadAllRecursiveFromRoot(t *testing.T) {
	/*
		Test Directory Structure:
		> dir0_0C
		>> .dockerignore
		>> subdir1_1E
		>>> subdir2_1C
		>>>> .dockerignore
		>> subdir1_2C
		>>> .dockerignore
		>> subdir1_3E
	*/
	dir0_0C := createTmpDir(t, "", "dockerignore-test")
	subdir1_1E := createTmpDir(t, dir0_0C, "subdir1_1E")
	subdir2_1C := createTmpDir(t, subdir1_1E, "subdir2_1C")
	subdir1_2C := createTmpDir(t, dir0_0C, "subdir1_2C")
	ioutil.TempDir(dir0_0C, "subdir1_3E")
	defer os.RemoveAll(dir0_0C)

	// Write content to files
	content := fmt.Sprintf("0_0File1\n/0_0File2\n!/dir/subdir/0_0file\n\n0_0lastfile\n!")
	content2 := fmt.Sprintf("2_1File1\n/2_1File2\n!/dir/subdir/2_1file\n\n2_1lastfile\n!")
	content3 := fmt.Sprintf("1_2File1\n/1_2File2\n!/dir/subdir/1_2file\n\n1_2lastfile\n!")

	writeFileHelper(t, filepath.Join(dir0_0C, ".dockerignore"), []byte(content), 0777)
	writeFileHelper(t, filepath.Join(subdir2_1C, ".dockerignore"), []byte(content2), 0777)
	writeFileHelper(t, filepath.Join(subdir1_2C, ".dockerignore"), []byte(content3), 0777)

	// Read from root of item
	di, err := ReadAllRecursive(dir0_0C, dir0_0C)
	if err != nil {
		t.Fatal(err)
	}

	// Check result length
	if diLen := len(di); diLen != 15 {
		t.Fatalf("Expected to have twelve dockerignore entries, got %d", diLen)
	}

	expectedStrings := []string{
		getRelPath(t, dir0_0C, filepath.Join(dir0_0C, "0_0File1")),
		getRelPath(t, dir0_0C, filepath.Join(dir0_0C, "0_0File2")),
		"!" + getRelPath(t, dir0_0C, filepath.Join(dir0_0C, "/dir/subdir/0_0file")),
		getRelPath(t, dir0_0C, filepath.Join(dir0_0C, "0_0lastfile")),
		getRelPath(t, dir0_0C, filepath.Join(dir0_0C, "!")),
		getRelPath(t, dir0_0C, filepath.Join(subdir2_1C, "2_1File1")),
		getRelPath(t, dir0_0C, filepath.Join(subdir2_1C, "2_1File2")),
		"!" + getRelPath(t, dir0_0C, filepath.Join(subdir2_1C, "/dir/subdir/2_1file")),
		getRelPath(t, dir0_0C, filepath.Join(subdir2_1C, "2_1lastfile")),
		"!" + getRelPath(t, dir0_0C, filepath.Join(subdir2_1C, "")),
		getRelPath(t, dir0_0C, filepath.Join(subdir1_2C, "1_2File1")),
		getRelPath(t, dir0_0C, filepath.Join(subdir1_2C, "1_2File2")),
		"!" + getRelPath(t, dir0_0C, filepath.Join(subdir1_2C, "/dir/subdir/1_2file")),
		getRelPath(t, dir0_0C, filepath.Join(subdir1_2C, "1_2lastfile")),
		"!" + getRelPath(t, dir0_0C, filepath.Join(subdir1_2C, "")),
	}

	expectedNegVals := []bool{
		false,
		false,
		true,
		false,
		true,
		false,
		false,
		true,
		false,
		true,
		false,
		false,
		true,
		false,
		true,
	}

	// Test to make sure all paths are correct, relative to base testing directory
	for i := range di {
		if di[i].Pattern() != expectedStrings[i] {
			t.Fatalf("Element " + strconv.Itoa(i+1) + " is not " + expectedStrings[i] + ": " + di[i].Pattern())
		}
		if di[i].Negative() != expectedNegVals[i] {
			if expectedNegVals[i] {
				t.Fatalf("Element " + strconv.Itoa(i+1) + " expected to be negative")
			} else {
				t.Fatalf("Element " + strconv.Itoa(i+1) + " expected not to be negative")
			}
		}
	}
}
func TestReadAllRecursiveFromSubDir1_1(t *testing.T) {
	/*
		Test Directory Structure:
		> dir0_0C
		>> .dockerignore
		>> subdir1_1E
		>>> subdir2_1C
		>>>> .dockerignore
		>> subdir1_2C
		>>> .dockerignore
		>> subdir1_3E
	*/
	dir0_0C := createTmpDir(t, "", "dockerignore-test")
	subdir1_1E := createTmpDir(t, dir0_0C, "subdir1_1E")
	subdir2_1C := createTmpDir(t, subdir1_1E, "subdir2_1C")
	subdir1_2C := createTmpDir(t, dir0_0C, "subdir1_2C")
	ioutil.TempDir(dir0_0C, "subdir1_3E")
	defer os.RemoveAll(dir0_0C)

	// Write content to files
	content := fmt.Sprintf("0_0File1\n/0_0File2\n!/dir/subdir/0_0file\n\n0_0lastfile\n!")
	content2 := fmt.Sprintf("2_1File1\n/2_1File2\n!/dir/subdir/2_1file\n\n2_1lastfile\n!")
	content3 := fmt.Sprintf("1_2File1\n/1_2File2\n!/dir/subdir/1_2file\n\n1_2lastfile\n!")

	writeFileHelper(t, filepath.Join(dir0_0C, ".dockerignore"), []byte(content), 0777)
	writeFileHelper(t, filepath.Join(subdir2_1C, ".dockerignore"), []byte(content2), 0777)
	writeFileHelper(t, filepath.Join(subdir1_2C, ".dockerignore"), []byte(content3), 0777)

	// Read starting from subdir1_1E
	di, err := ReadAllRecursive(subdir1_1E, subdir1_1E)
	if err != nil {
		t.Fatal(err)
	}

	// Check length of result
	if diLen := len(di); diLen != 5 {
		t.Fatalf("Expected to have four dockerignore entries, got %d", diLen)
	}

	expectedStrings := []string{
		getRelPath(t, subdir1_1E, filepath.Join(subdir2_1C, "2_1File1")),
		getRelPath(t, subdir1_1E, filepath.Join(subdir2_1C, "/2_1File2")),
		"!" + getRelPath(t, subdir1_1E, filepath.Join(subdir2_1C, "/dir/subdir/2_1file")),
		getRelPath(t, subdir1_1E, filepath.Join(subdir2_1C, "2_1lastfile")),
		"!" + getRelPath(t, subdir1_1E, filepath.Join(subdir2_1C, "")),
	}

	expectedNegVals := []bool{
		false,
		false,
		true,
		false,
		true,
	}

	// Test to make sure all paths are correct, relative to base testing directory
	for i := range di {
		if di[i].Pattern() != expectedStrings[i] {
			t.Fatalf("Element " + strconv.Itoa(i+1) + " is not " + expectedStrings[i] + ": " + di[i].Pattern())
		}
		if di[i].Negative() != expectedNegVals[i] {
			if expectedNegVals[i] {
				t.Fatalf("Element " + strconv.Itoa(i+1) + " expected to be negative")
			} else {
				t.Fatalf("Element " + strconv.Itoa(i+1) + " expected not to be negative")
			}
		}
	}

	fmt.Println(precompiledregexp.ToStringExpressions(di))
}

func writeFileHelper(t *testing.T, filename string, data []byte, perm os.FileMode) {
	err := ioutil.WriteFile(filename, data, perm)
	if err != nil {
		t.Fatal(err)
	}
}
func createTmpDir(t *testing.T, dir string, prefix string) string {

	result, err := ioutil.TempDir(dir, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func getRelPath(t *testing.T, baseDir string, path string) string {
	relPath, err := filepath.Rel(baseDir, path)
	if err != nil {
		t.Fatal(err)
	}
	return relPath
}
