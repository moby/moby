package dockerignore

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestReadAll(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "dockerignore-test")
	if err != nil {
		t.Fatal(err)
	}
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
	baseDir, err := ioutil.TempDir("", "dockerignore-test")
	if err != nil {
		t.Fatal(err)
	}
	dir0_0C := baseDir
	subdir1_1E, err := ioutil.TempDir(dir0_0C, "subdir1_1E")
	if err != nil {
		t.Fatal(err)
	}
	subdir2_1C, err := ioutil.TempDir(subdir1_1E, "subdir2_1C")
	if err != nil {
		t.Fatal(err)
	}
	subdir1_2C, err := ioutil.TempDir(dir0_0C, "subdir1_2C")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ioutil.TempDir(dir0_0C, "subdir1_3E")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir0_0C)

	// Write content to files
	content := fmt.Sprintf("0_0File1\n/0_0File2\n!/dir/subdir/0_0file\n\n0_0lastfile")
	content2 := fmt.Sprintf("2_1File1\n/2_1File2\n!/dir/subdir/2_1file\n\n2_1lastfile")
	content3 := fmt.Sprintf("1_2File1\n/1_2File2\n!/dir/subdir/1_2file\n\n1_2lastfile")

	err = ioutil.WriteFile(filepath.Join(dir0_0C, ".dockerignore"), []byte(content), 0777)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(subdir2_1C, ".dockerignore"), []byte(content2), 0777)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(subdir1_2C, ".dockerignore"), []byte(content3), 0777)
	if err != nil {
		t.Fatal(err)
	}

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
	baseDir, err := ioutil.TempDir("", "dockerignore-test")
	if err != nil {
		t.Fatal(err)
	}
	dir0_0C := baseDir
	subdir1_1E, err := ioutil.TempDir(dir0_0C, "subdir1_1E")
	if err != nil {
		t.Fatal(err)
	}
	subdir2_1C, err := ioutil.TempDir(subdir1_1E, "subdir2_1C")
	if err != nil {
		t.Fatal(err)
	}
	subdir1_2C, err := ioutil.TempDir(dir0_0C, "subdir1_2C")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ioutil.TempDir(dir0_0C, "subdir1_3E")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir0_0C)

	// Convert all paths to relative paths from base testing directory
	dir0_0CRel, err := filepath.Rel(baseDir, dir0_0C)
	if err != nil {
		t.Fatal(err)
	}
	subdir2_1CRel, err := filepath.Rel(baseDir, subdir2_1C)
	if err != nil {
		t.Fatal(err)
	}
	subdir1_2CRel, err := filepath.Rel(baseDir, subdir1_2C)
	if err != nil {
		t.Fatal(err)
	}

	// Write content to files
	content := fmt.Sprintf("0_0File1\n/0_0File2\n!/dir/subdir/0_0file\n\n0_0lastfile")
	content2 := fmt.Sprintf("2_1File1\n/2_1File2\n!/dir/subdir/2_1file\n\n2_1lastfile")
	content3 := fmt.Sprintf("1_2File1\n/1_2File2\n!/dir/subdir/1_2file\n\n1_2lastfile")

	err = ioutil.WriteFile(filepath.Join(dir0_0C, ".dockerignore"), []byte(content), 0777)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(subdir2_1C, ".dockerignore"), []byte(content2), 0777)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(subdir1_2C, ".dockerignore"), []byte(content3), 0777)
	if err != nil {
		t.Fatal(err)
	}

	// Read from root of item
	di, err := ReadAllRecursive(dir0_0C, dir0_0C)
	if err != nil {
		t.Fatal(err)
	}

	// Check result length
	if diLen := len(di); diLen != 12 {
		t.Fatalf("Expected to have twelve dockerignore entries, got %d", diLen)
	}

	// Test to make sure all paths are correct, relative to base testing directory
	expected := filepath.Join(dir0_0CRel, "0_0File1")
	if di[0] != expected {
		t.Fatalf("9th element is not " + expected)
	}
	expected = filepath.Join(dir0_0CRel, "/0_0File2")
	if di[1] != expected {
		t.Fatalf("10th element is not " + expected)
	}
	expected = "!" + filepath.Join(dir0_0CRel, "/dir/subdir/0_0file")
	if di[2] != expected {
		t.Fatalf("11th element is not " + expected)
	}
	expected = filepath.Join(dir0_0CRel, "0_0lastfile")
	if di[3] != expected {
		t.Fatalf("12th element is not " + expected)
	}
	expected = filepath.Join(subdir2_1CRel, "2_1File1")
	if di[4] != expected {
		t.Fatalf("1st element is not " + expected)
	}
	expected = filepath.Join(subdir2_1CRel, "/2_1File2")
	if di[5] != expected {
		t.Fatalf("2nd element is not " + expected)
	}
	expected = "!" + filepath.Join(subdir2_1CRel, "/dir/subdir/2_1file")
	if di[6] != expected {
		t.Fatalf("3rd element is not " + expected)
	}
	expected = filepath.Join(subdir2_1CRel, "2_1lastfile")
	if di[7] != expected {
		t.Fatalf("4th element is not " + expected)
	}
	expected = filepath.Join(subdir1_2CRel, "1_2File1")
	if di[8] != expected {
		t.Fatalf("5th element is not " + expected)
	}
	expected = filepath.Join(subdir1_2CRel, "/1_2File2")
	if di[9] != expected {
		t.Fatalf("6th element is not " + expected)
	}
	expected = "!" + filepath.Join(subdir1_2CRel, "/dir/subdir/1_2file")
	if di[10] != expected {
		t.Fatalf("7th element is not " + expected)
	}
	expected = filepath.Join(subdir1_2CRel, "1_2lastfile")
	if di[11] != expected {
		t.Fatalf("8th element is not " + expected)
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
	baseDir, err := ioutil.TempDir("", "dockerignore-test")
	if err != nil {
		t.Fatal(err)
	}
	dir0_0C := baseDir
	subdir1_1E, err := ioutil.TempDir(dir0_0C, "subdir1_1E")
	if err != nil {
		t.Fatal(err)
	}
	subdir2_1C, err := ioutil.TempDir(subdir1_1E, "subdir2_1C")
	if err != nil {
		t.Fatal(err)
	}
	subdir1_2C, err := ioutil.TempDir(dir0_0C, "subdir1_2C")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ioutil.TempDir(dir0_0C, "subdir1_3E")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir0_0C)

	// Convert all paths to relative paths from base testing directory
	subdir2_1CRel, err := filepath.Rel(subdir1_1E, subdir2_1C)
	if err != nil {
		t.Fatal(err)
	}

	// Write content to files
	content := fmt.Sprintf("0_0File1\n/0_0File2\n!/dir/subdir/0_0file\n\n0_0lastfile")
	content2 := fmt.Sprintf("2_1File1\n/2_1File2\n!/dir/subdir/2_1file\n\n2_1lastfile")
	content3 := fmt.Sprintf("1_2File1\n/1_2File2\n!/dir/subdir/1_2file\n\n1_2lastfile")

	err = ioutil.WriteFile(filepath.Join(dir0_0C, ".dockerignore"), []byte(content), 0777)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(subdir2_1C, ".dockerignore"), []byte(content2), 0777)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(subdir1_2C, ".dockerignore"), []byte(content3), 0777)
	if err != nil {
		t.Fatal(err)
	}

	// Read starting from subdir1_1E
	di, err := ReadAllRecursive(subdir1_1E, subdir1_1E)
	if err != nil {
		t.Fatal(err)
	}

	// Check length of result
	if diLen := len(di); diLen != 4 {
		t.Fatalf("Expected to have four dockerignore entries, got %d", diLen)
	}

	// Test to make sure all paths are correct, relative to base testing directory
	expected := filepath.Join(subdir2_1CRel, "2_1File1")
	if di[0] != expected {
		t.Fatalf("1st element is not " + expected)
	}
	expected = filepath.Join(subdir2_1CRel, "/2_1File2")
	if di[1] != expected {
		t.Fatalf("2nd element is not " + expected)
	}
	expected = "!" + filepath.Join(subdir2_1CRel, "/dir/subdir/2_1file")
	if di[2] != expected {
		t.Fatalf("3rd element is not " + expected)
	}
	expected = filepath.Join(subdir2_1CRel, "2_1lastfile")
	if di[3] != expected {
		t.Fatalf("4th element is not " + expected)
	}
}
