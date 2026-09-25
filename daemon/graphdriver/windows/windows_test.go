//go:build windows

package windows

import (
	"archive/tar"
	"bytes"
	"io"
	"testing"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

// fakeEntry describes one file returned by fakeLayerReader.
type fakeEntry struct {
	name   string
	data   []byte
	nlinks uint32
	fileID [16]byte
}

// fakeLayerReader implements hcsshim.LayerReader over a scripted set of files
// so that writeTarFromLayer can be exercised without a real layer.
type fakeLayerReader struct {
	entries []fakeEntry
	idx     int
	stream  *bytes.Reader
}

func (r *fakeLayerReader) Next() (string, int64, *winio.FileBasicInfo, error) {
	if r.idx >= len(r.entries) {
		return "", 0, nil, io.EOF
	}
	e := r.entries[r.idx]
	r.idx++

	stream, err := backupStream(e.data)
	if err != nil {
		return "", 0, nil, err
	}
	r.stream = bytes.NewReader(stream)

	fileInfo := &winio.FileBasicInfo{FileAttributes: windows.FILE_ATTRIBUTE_NORMAL}
	return e.name, int64(len(e.data)), fileInfo, nil
}

func (r *fakeLayerReader) LinkInfo() (uint32, *winio.FileIDInfo, error) {
	e := r.entries[r.idx-1]
	return e.nlinks, &winio.FileIDInfo{VolumeSerialNumber: 1, FileID: e.fileID}, nil
}

func (r *fakeLayerReader) Read(b []byte) (int, error) {
	if r.stream == nil {
		return 0, io.EOF
	}
	return r.stream.Read(b)
}

func (r *fakeLayerReader) Close() error { return nil }

// backupStream wraps raw file contents in a Win32 backup stream, which is what
// WriteTarFileFromBackupStream expects to read.
func backupStream(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := winio.NewBackupStreamWriter(&buf)
	if err := w.WriteHeader(&winio.BackupHeader{Id: winio.BackupData, Size: int64(len(data))}); err != nil {
		return nil, err
	}
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type tarEntry struct {
	name     string
	typeflag byte
	linkname string
	size     int64
}

func readTar(t *testing.T, b []byte) []tarEntry {
	t.Helper()

	var entries []tarEntry
	tr := tar.NewReader(bytes.NewReader(b))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading tar: %v", err)
		}
		entries = append(entries, tarEntry{
			name:     hdr.Name,
			typeflag: hdr.Typeflag,
			linkname: hdr.Linkname,
			size:     hdr.Size,
		})
	}
	return entries
}

func TestWriteTarFromLayerHardLinks(t *testing.T) {
	fileIDA := [16]byte{1}
	fileIDB := [16]byte{2}

	for _, tc := range []struct {
		name     string
		entries  []fakeEntry
		expected []tarEntry
	}{
		{
			// Every later name for a file ID points at the first name, never at
			// the name immediately before it, so links are never chained.
			name: "repeated names link to the first name",
			entries: []fakeEntry{
				{name: "Files/a.txt", data: []byte("payload"), nlinks: 3, fileID: fileIDA},
				{name: "Files/b.txt", data: []byte("payload"), nlinks: 3, fileID: fileIDA},
				{name: "Files/c.txt", data: []byte("payload"), nlinks: 3, fileID: fileIDA},
			},
			expected: []tarEntry{
				{name: "Files/a.txt", typeflag: tar.TypeReg, size: 7},
				{name: "Files/b.txt", typeflag: tar.TypeLink, linkname: "Files/a.txt"},
				{name: "Files/c.txt", typeflag: tar.TypeLink, linkname: "Files/a.txt"},
			},
		},
		{
			name: "distinct file IDs are never linked",
			entries: []fakeEntry{
				{name: "Files/a.txt", data: []byte("payload"), nlinks: 2, fileID: fileIDA},
				{name: "Files/b.txt", data: []byte("payload"), nlinks: 2, fileID: fileIDB},
			},
			expected: []tarEntry{
				{name: "Files/a.txt", typeflag: tar.TypeReg, size: 7},
				{name: "Files/b.txt", typeflag: tar.TypeReg, size: 7},
			},
		},
		{
			name: "a single link count is never linked",
			entries: []fakeEntry{
				{name: "Files/a.txt", data: []byte("payload"), nlinks: 1, fileID: fileIDA},
				{name: "Files/b.txt", data: []byte("payload"), nlinks: 1, fileID: fileIDA},
			},
			expected: []tarEntry{
				{name: "Files/a.txt", typeflag: tar.TypeReg, size: 7},
				{name: "Files/b.txt", typeflag: tar.TypeReg, size: 7},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := &fakeLayerReader{entries: tc.entries}
			if err := writeTarFromLayer(r, &buf); err != nil {
				t.Fatalf("writeTarFromLayer: %v", err)
			}

			got := readTar(t, buf.Bytes())
			if len(got) != len(tc.expected) {
				t.Fatalf("got %d tar entries, want %d: %+v", len(got), len(tc.expected), got)
			}

			for i, want := range tc.expected {
				if got[i].name != want.name {
					t.Errorf("entry %d: name = %q, want %q", i, got[i].name, want.name)
				}
				if got[i].typeflag != want.typeflag {
					t.Errorf("entry %d (%s): typeflag = %q, want %q", i, want.name, got[i].typeflag, want.typeflag)
				}
				if got[i].linkname != want.linkname {
					t.Errorf("entry %d (%s): linkname = %q, want %q", i, want.name, got[i].linkname, want.linkname)
				}
				// A hard link entry must not carry a second copy of the payload.
				if want.typeflag == tar.TypeLink && got[i].size != 0 {
					t.Errorf("entry %d (%s): size = %d, want 0 for a hard link", i, want.name, got[i].size)
				}
			}
		})
	}
}
