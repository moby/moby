package tailfile // import "github.com/docker/docker/pkg/tailfile"

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestTailFile(t *testing.T) {
	f, err := os.CreateTemp("", "tail-test")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	defer os.RemoveAll(f.Name())
	testFile := []byte(`first line
second line
third line
fourth line
fifth line
next first line
next second line
next third line
next fourth line
next fifth line
last first line
next first line
next second line
next third line
next fourth line
next fifth line
next first line
next second line
next third line
next fourth line
next fifth line
last second line
last third line
last fourth line
last fifth line
truncated line`)
	if _, err := f.Write(testFile); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	expected := []string{"last fourth line", "last fifth line"}
	res, err := TailFile(f, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != len(expected) {
		t.Fatalf("\nexpected:\n%s\n\nactual:\n%s", expected, res)
	}
	for i, l := range res {
		if expected[i] != string(l) {
			t.Fatalf("Expected line %q, got %q", expected[i], l)
		}
	}
}

func TestTailFileManyLines(t *testing.T) {
	f, err := os.CreateTemp("", "tail-test")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	defer os.RemoveAll(f.Name())
	testFile := []byte(`first line
second line
truncated line`)
	if _, err := f.Write(testFile); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	expected := []string{"first line", "second line"}
	res, err := TailFile(f, 10000)
	if err != nil {
		t.Fatal(err)
	}
	if len(expected) != len(res) {
		t.Fatalf("\nexpected:\n%s\n\nactual:\n%s", expected, res)
	}
	for i, l := range res {
		if expected[i] != string(l) {
			t.Fatalf("Expected line %s, got %s", expected[i], l)
		}
	}
}

func TestTailEmptyFile(t *testing.T) {
	f, err := os.CreateTemp("", "tail-test")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	defer os.RemoveAll(f.Name())
	res, err := TailFile(f, 10000)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		t.Fatal("Must be empty slice from empty file")
	}
}

func TestTailNegativeN(t *testing.T) {
	f, err := os.CreateTemp("", "tail-test")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	defer os.RemoveAll(f.Name())
	testFile := []byte(`first line
second line
truncated line`)
	if _, err := f.Write(testFile); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if _, err := TailFile(f, -1); err != ErrNonPositiveLinesNumber {
		t.Fatalf("Expected ErrNonPositiveLinesNumber, got %v", err)
	}
	if _, err := TailFile(f, 0); err != ErrNonPositiveLinesNumber {
		t.Fatalf("Expected ErrNonPositiveLinesNumber, got %s", err)
	}
}

func BenchmarkTail(b *testing.B) {
	f, err := os.CreateTemp("", "tail-test")
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	defer os.RemoveAll(f.Name())
	for i := 0; i < 10000; i++ {
		if _, err := f.Write([]byte("tailfile pretty interesting line\n")); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := TailFile(f, 1000); err != nil {
			b.Fatal(err)
		}
	}
}

func TestNewTailReader(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	for dName, delim := range map[string][]byte{
		"no delimiter":          {},
		"single byte delimiter": {'\n'},
		"2 byte delimiter":      []byte(";\n"),
		"4 byte delimiter":      []byte("####"),
		"8 byte delimiter":      []byte("########"),
		"12 byte delimiter":     []byte("############"),
	} {
		t.Run(dName, func(t *testing.T) {
			delim := delim
			t.Parallel()

			s1 := "Hello world."
			s2 := "Today is a fine day."
			s3 := "So long, and thanks for all the fish!"
			s4 := strings.Repeat("a", blockSize/2) // same as block size
			s5 := strings.Repeat("a", blockSize)   // just to make sure
			s6 := strings.Repeat("a", blockSize*2) // bigger than block size
			s7 := strings.Repeat("a", blockSize-1) // single line same as block

			s8 := `{"log":"Don't panic!\n","stream":"stdout","time":"2018-04-04T20:28:44.7207062Z"}`
			jsonTest := make([]string, 0, 20)
			for i := 0; i < 20; i++ {
				jsonTest = append(jsonTest, s8)
			}

			for _, test := range []struct {
				desc string
				data []string
			}{
				{desc: "one small entry", data: []string{s1}},
				{desc: "several small entries", data: []string{s1, s2, s3}},
				{desc: "various sizes", data: []string{s1, s2, s3, s4, s5, s1, s2, s3, s7, s6}},
				{desc: "multiple lines with one more than block", data: []string{s5, s5, s5, s5, s5}},
				{desc: "multiple lines much bigger than block", data: []string{s6, s6, s6, s6, s6}},
				{desc: "multiple lines same as block", data: []string{s4, s4, s4, s4, s4}},
				{desc: "single line same as block", data: []string{s7}},
				{desc: "single line half block", data: []string{s4}},
				{desc: "single line twice block", data: []string{s6}},
				{desc: "json encoded values", data: jsonTest},
				{desc: "no lines", data: []string{}},
				{desc: "same length as delimiter", data: []string{strings.Repeat("a", len(delim))}},
			} {
				t.Run(test.desc, func(t *testing.T) {
					test := test
					t.Parallel()

					max := len(test.data)
					if max > 10 {
						max = 10
					}

					s := strings.Join(test.data, string(delim))
					if len(test.data) > 0 {
						s += string(delim)
					}

					for i := 1; i <= max; i++ {
						t.Run(fmt.Sprintf("%d lines", i), func(t *testing.T) {
							i := i
							t.Parallel()

							r := strings.NewReader(s)
							tr, lines, err := NewTailReaderWithDelimiter(ctx, r, i, delim)
							if len(delim) == 0 {
								assert.Assert(t, err != nil)
								assert.Assert(t, lines == 0)
								return
							}
							assert.NilError(t, err)
							assert.Check(t, lines == i, "%d -- %d", lines, i)

							b, err := io.ReadAll(tr)
							assert.NilError(t, err)

							expectLines := test.data[len(test.data)-i:]
							assert.Check(t, len(expectLines) == i)
							expect := strings.Join(expectLines, string(delim)) + string(delim)
							assert.Check(t, string(b) == expect, "\n%v\n%v", b, []byte(expect))
						})
					}

					t.Run("request more lines than available", func(t *testing.T) {
						t.Parallel()

						r := strings.NewReader(s)
						tr, lines, err := NewTailReaderWithDelimiter(ctx, r, len(test.data)*2, delim)
						if len(delim) == 0 {
							assert.Assert(t, err != nil)
							assert.Assert(t, lines == 0)
							return
						}
						if len(test.data) == 0 {
							assert.Assert(t, err == ErrNonPositiveLinesNumber, err)
							return
						}

						assert.NilError(t, err)
						assert.Check(t, lines == len(test.data), "%d -- %d", lines, len(test.data))
						b, err := io.ReadAll(tr)
						assert.NilError(t, err)
						assert.Check(t, bytes.Equal(b, []byte(s)), "\n%v\n%v", b, []byte(s))
					})
				})
			}
		})
	}
	t.Run("truncated last line", func(t *testing.T) {
		t.Run("more than available", func(t *testing.T) {
			tail, nLines, err := NewTailReader(ctx, strings.NewReader("a\nb\nextra"), 3)
			assert.NilError(t, err)
			assert.Check(t, nLines == 2, nLines)

			rdr := bufio.NewReader(tail)
			data, _, err := rdr.ReadLine()
			assert.NilError(t, err)
			assert.Check(t, string(data) == "a", string(data))

			data, _, err = rdr.ReadLine()
			assert.NilError(t, err)
			assert.Check(t, string(data) == "b", string(data))

			_, _, err = rdr.ReadLine()
			assert.Assert(t, err == io.EOF, err)
		})
	})
	t.Run("truncated last line", func(t *testing.T) {
		t.Run("exact", func(t *testing.T) {
			tail, nLines, err := NewTailReader(ctx, strings.NewReader("a\nb\nextra"), 2)
			assert.NilError(t, err)
			assert.Check(t, nLines == 2, nLines)

			rdr := bufio.NewReader(tail)
			data, _, err := rdr.ReadLine()
			assert.NilError(t, err)
			assert.Check(t, string(data) == "a", string(data))

			data, _, err = rdr.ReadLine()
			assert.NilError(t, err)
			assert.Check(t, string(data) == "b", string(data))

			_, _, err = rdr.ReadLine()
			assert.Assert(t, err == io.EOF, err)
		})
	})

	t.Run("truncated last line", func(t *testing.T) {
		t.Run("one line", func(t *testing.T) {
			tail, nLines, err := NewTailReader(ctx, strings.NewReader("a\nb\nextra"), 1)
			assert.NilError(t, err)
			assert.Check(t, nLines == 1, nLines)

			rdr := bufio.NewReader(tail)
			data, _, err := rdr.ReadLine()
			assert.NilError(t, err)
			assert.Check(t, string(data) == "b", string(data))

			_, _, err = rdr.ReadLine()
			assert.Assert(t, err == io.EOF, err)
		})
	})
}
