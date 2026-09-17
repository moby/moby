//go:build windows

package windows

import (
	"archive/tar"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"path/filepath"
	"testing"
	"unicode/utf16"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/go-winio/backuptar"
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

func TestNormalizeReparseTarget(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "PowerShell junction with NT prefix and drive letter",
			input:    `\??\C:\Windows`,
			expected: `C:\Windows`,
		},
		{
			name:     "Forward slash NT prefix with drive letter",
			input:    `/??/C:/Windows`,
			expected: `C:\Windows`,
		},
		{
			name:     "Lowercase drive letter with NT prefix",
			input:    `\??\c:\some\path`,
			expected: `c:\some\path`,
		},
		{
			name:     "UNC path with NT prefix",
			input:    `\??\UNC\server\share\folder`,
			expected: `\\server\share\folder`,
		},
		{
			name:     "Forward slash UNC path with NT prefix",
			input:    `/??/UNC/server/share/folder`,
			expected: `\\server\share\folder`,
		},
		{
			name:     "Volume GUID path with NT prefix",
			input:    `\??\Volume{12345678-1234-1234-1234-123456789abc}\`,
			expected: `\\?\Volume{12345678-1234-1234-1234-123456789abc}\`,
		},
		{
			name:     "Win32 extended path with drive letter",
			input:    `\\?\C:\Windows`,
			expected: `C:\Windows`,
		},
		{
			name:     "Win32 extended UNC path",
			input:    `\\?\UNC\server\share`,
			expected: `\\server\share`,
		},
		{
			name:     "Standard absolute drive path unchanged",
			input:    `C:\Windows`,
			expected: `C:\Windows`,
		},
		{
			name:     "Standard UNC path unchanged",
			input:    `\\server\share`,
			expected: `\\server\share`,
		},
		{
			name:     "Relative backslash path unchanged",
			input:    `relative\path`,
			expected: `relative\path`,
		},
		{
			name:     "Relative path with parent traversal unchanged",
			input:    `../relative/path`,
			expected: `../relative/path`,
		},
		{
			name:     "Empty string unchanged",
			input:    ``,
			expected: ``,
		},
		{
			name:     "NT path with GLOBALROOT device unchanged",
			input:    `\??\GLOBALROOT\Device\X`,
			expected: `\??\GLOBALROOT\Device\X`,
		},
		{
			name:     "NT path with Device prefix unchanged",
			input:    `\??\Device\Y`,
			expected: `\??\Device\Y`,
		},
		{
			name:     "Win32 extended path with GLOBALROOT unchanged",
			input:    `\\?\GLOBALROOT\Device\Harddisk0`,
			expected: `\\?\GLOBALROOT\Device\Harddisk0`,
		},
		{
			name:     "Win32 extended path with Device prefix unchanged",
			input:    `\\?\Device\Z`,
			expected: `\\?\Device\Z`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeReparseTarget(tc.input)
			if got != tc.expected {
				t.Errorf("normalizeReparseTarget(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

type recordedLayerFile struct {
	name     string
	fileInfo *winio.FileBasicInfo
	stream   []byte
}

type fakeLayerWriter struct {
	files       []recordedLayerFile
	currentFile *recordedLayerFile
}

func (w *fakeLayerWriter) Add(name string, fileInfo *winio.FileBasicInfo) error {
	w.files = append(w.files, recordedLayerFile{name: name, fileInfo: fileInfo})
	w.currentFile = &w.files[len(w.files)-1]
	return nil
}

func (w *fakeLayerWriter) AddLink(name string, target string) error {
	return nil
}

func (w *fakeLayerWriter) Remove(name string) error {
	return nil
}

func (w *fakeLayerWriter) Write(b []byte) (int, error) {
	if w.currentFile == nil {
		return 0, io.ErrUnexpectedEOF
	}
	w.currentFile.stream = append(w.currentFile.stream, b...)
	return len(b), nil
}

func (w *fakeLayerWriter) Close() error {
	return nil
}

func TestWriteLayerFromTarJunctionTargetNormalization(t *testing.T) {
	// Build a tar archive containing entries to exercise reparse target normalization
	// including PowerShell New-Item created junctions with the \??\ prefix.
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)

	entries := []struct {
		name         string
		linkname     string
		isMountPoint bool
		wantTarget   string
	}{
		{
			name:         "tmp/JuncPS5",
			linkname:     `\??\C:\Windows`,
			isMountPoint: true,
			wantTarget:   `C:\Windows`,
		},
		{
			name:         "tmp/JuncSlash",
			linkname:     `/??/C:/Windows`,
			isMountPoint: true,
			wantTarget:   `C:\Windows`,
		},
		{
			name:         "tmp/JuncUNC",
			linkname:     `\??\UNC\server\share`,
			isMountPoint: true,
			wantTarget:   `\\server\share`,
		},
		{
			name:         "tmp/SymlinkRel",
			linkname:     `..\target\file.txt`,
			isMountPoint: false,
			wantTarget:   `..\target\file.txt`,
		},
	}

	for _, e := range entries {
		fileAttr := uint32(windows.FILE_ATTRIBUTE_REPARSE_POINT)
		if e.isMountPoint {
			fileAttr |= windows.FILE_ATTRIBUTE_DIRECTORY
		}
		pax := map[string]string{
			"MSWINDOWS.fileattr": fmt.Sprintf("%d", fileAttr),
		}
		if e.isMountPoint {
			pax["MSWINDOWS.mountpoint"] = "1"
		}

		hdr := &tar.Header{
			Name:       e.name,
			Typeflag:   tar.TypeSymlink,
			Linkname:   e.linkname,
			PAXRecords: pax,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader(%s): %v", e.name, err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("tw.Close(): %v", err)
	}

	flw := &fakeLayerWriter{}
	tempRoot := t.TempDir()

	_, err := writeLayerFromTar(bytes.NewReader(tarBuf.Bytes()), flw, tempRoot)
	if err != nil {
		t.Fatalf("writeLayerFromTar failed: %v", err)
	}

	if len(flw.files) != len(entries) {
		t.Fatalf("got %d files written to layer, want %d", len(flw.files), len(entries))
	}

	for i, e := range entries {
		rf := flw.files[i]
		if rf.name != filepath.FromSlash(e.name) {
			t.Errorf("entry %d name = %q, want %q", i, rf.name, filepath.FromSlash(e.name))
		}

		// Decode the reparse point written into the backup stream
		br := winio.NewBackupStreamReader(bytes.NewReader(rf.stream))
		var decodedRP *winio.ReparsePoint
		for {
			bhdr, err := br.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("reading backup stream for %s: %v", e.name, err)
			}
			if bhdr.Id == winio.BackupReparseData {
				data, err := io.ReadAll(br)
				if err != nil {
					t.Fatalf("reading BackupReparseData for %s: %v", e.name, err)
				}
				rp, err := winio.DecodeReparsePoint(data)
				if err != nil {
					t.Fatalf("DecodeReparsePoint for %s: %v", e.name, err)
				}
				decodedRP = rp
			}
		}

		if decodedRP == nil {
			t.Fatalf("entry %s did not contain a BackupReparseData stream", e.name)
		}
		if decodedRP.Target != e.wantTarget {
			t.Errorf("entry %s reparse target = %q, want %q", e.name, decodedRP.Target, e.wantTarget)
		}
		if decodedRP.IsMountPoint != e.isMountPoint {
			t.Errorf("entry %s IsMountPoint = %v, want %v", e.name, decodedRP.IsMountPoint, e.isMountPoint)
		}
	}
}

func TestFixReparseBackupStream(t *testing.T) {
	// Construct a raw REPARSE_DATA_BUFFER matching PowerShell's New-Item cmdlet:
	// SubstituteName: \??\C:\Windows
	// PrintName: "" (length 0)
	subTarget := `\??\C:\Windows`
	subTarget16 := utf16.Encode([]rune(subTarget + "\x00"))

	tag := uint32(reparseTagMountPoint)
	// Header size: 8 bytes (tag:4, dataLength:2, reserved:2)
	// MountPoint specific header: 8 bytes (subOffset:2, subLen:2, printOffset:2, printLen:2)
	subOffset := uint16(0)
	subLen := uint16((len(subTarget16) - 1) * 2)
	printOffset := uint16(len(subTarget16) * 2)
	printLen := uint16(0)

	dataLen := uint16(8 + len(subTarget16)*2)

	var rawRP bytes.Buffer
	_ = binary.Write(&rawRP, binary.LittleEndian, tag)
	_ = binary.Write(&rawRP, binary.LittleEndian, dataLen)
	_ = binary.Write(&rawRP, binary.LittleEndian, uint16(0)) // reserved
	_ = binary.Write(&rawRP, binary.LittleEndian, subOffset)
	_ = binary.Write(&rawRP, binary.LittleEndian, subLen)
	_ = binary.Write(&rawRP, binary.LittleEndian, printOffset)
	_ = binary.Write(&rawRP, binary.LittleEndian, printLen)
	_ = binary.Write(&rawRP, binary.LittleEndian, subTarget16)

	// Verify standard winio.DecodeReparsePoint fails on this buffer (extracts empty target)
	unfixedRP, err := winio.DecodeReparsePoint(rawRP.Bytes())
	if err != nil {
		t.Fatalf("DecodeReparsePoint failed: %v", err)
	}
	if unfixedRP.Target != "" {
		t.Fatalf("expected unpatched DecodeReparsePoint to return empty target, got %q", unfixedRP.Target)
	}

	// Pack into a Win32 backup stream
	var inStream bytes.Buffer
	bsw := winio.NewBackupStreamWriter(&inStream)
	err = bsw.WriteHeader(&winio.BackupHeader{
		Id:   winio.BackupReparseData,
		Size: int64(rawRP.Len()),
	})
	if err != nil {
		t.Fatalf("WriteHeader failed: %v", err)
	}
	if _, err := bsw.Write(rawRP.Bytes()); err != nil {
		t.Fatalf("bsw.Write failed: %v", err)
	}

	inStreamBytes := append([]byte(nil), inStream.Bytes()...)

	// Now run through fixReparseBackupStream
	fixedStream, err := fixReparseBackupStream(bytes.NewReader(inStreamBytes))
	if err != nil {
		t.Fatalf("fixReparseBackupStream failed: %v", err)
	}

	// Verify the fixed backup stream decodes with valid Target = "C:\\Windows"
	fixedReader := winio.NewBackupStreamReader(fixedStream)
	fixedHdr, err := fixedReader.Next()
	if err != nil {
		t.Fatalf("fixedReader.Next() failed: %v", err)
	}
	if fixedHdr.Id != winio.BackupReparseData {
		t.Fatalf("expected BackupReparseData stream, got %d", fixedHdr.Id)
	}
	fixedData, err := io.ReadAll(fixedReader)
	if err != nil {
		t.Fatalf("reading fixedData: %v", err)
	}

	fixedRP, err := winio.DecodeReparsePoint(fixedData)
	if err != nil {
		t.Fatalf("DecodeReparsePoint on fixed data failed: %v", err)
	}
	if fixedRP.Target != `C:\Windows` {
		t.Errorf("fixedRP.Target = %q, want %q", fixedRP.Target, `C:\Windows`)
	}
	if !fixedRP.IsMountPoint {
		t.Errorf("fixedRP.IsMountPoint = %v, want true", fixedRP.IsMountPoint)
	}

	// Also verify that backuptar.WriteTarFileFromBackupStream writes clean tar header
	fixedStream2, err := fixReparseBackupStream(bytes.NewReader(inStreamBytes))
	if err != nil {
		t.Fatalf("fixReparseBackupStream 2 failed: %v", err)
	}
	var tarOut bytes.Buffer
	tw := tar.NewWriter(&tarOut)
	fileInfo := &winio.FileBasicInfo{
		FileAttributes: windows.FILE_ATTRIBUTE_DIRECTORY | windows.FILE_ATTRIBUTE_REPARSE_POINT,
	}
	err = backuptar.WriteTarFileFromBackupStream(tw, fixedStream2, "tmp/JuncPS5", 0, fileInfo)
	if err != nil {
		t.Fatalf("WriteTarFileFromBackupStream failed: %v", err)
	}
	_ = tw.Close()

	tr := tar.NewReader(&tarOut)
	tarHdr, err := tr.Next()
	if err != nil {
		t.Fatalf("tr.Next() failed: %v", err)
	}
	if tarHdr.Linkname != `C:\Windows` {
		t.Errorf("tarHdr.Linkname = %q, want %q", tarHdr.Linkname, `C:\Windows`)
	}
	if tarHdr.PAXRecords["MSWINDOWS.mountpoint"] != "1" {
		t.Errorf("tarHdr.PAXRecords[MSWINDOWS.mountpoint] = %q, want '1'", tarHdr.PAXRecords["MSWINDOWS.mountpoint"])
	}
}

func TestFixReparseBackupStreamSymlinkUntouched(t *testing.T) {
	// Construct a valid relative symlink with SYMLINK_FLAG_RELATIVE (flags = 1)
	symlinkTarget := `..\relative\target\file.txt`
	rp := &winio.ReparsePoint{
		Target:       symlinkTarget,
		IsMountPoint: false,
	}
	originalRPBytes := winio.EncodeReparsePoint(rp)

	// Verify that the encoded buffer has tag reparseTagSymlink (0xA000000C)
	tag := binary.LittleEndian.Uint32(originalRPBytes[0:4])
	if tag != 0xA000000C {
		t.Fatalf("expected symlink tag 0xA000000C, got 0x%X", tag)
	}
	// Verify SYMLINK_FLAG_RELATIVE is set (offset 16: 4 bytes of flags)
	flags := binary.LittleEndian.Uint32(originalRPBytes[16:20])
	if flags&1 == 0 {
		t.Fatalf("expected SYMLINK_FLAG_RELATIVE (1) to be set, got %d", flags)
	}

	// Pack into a Win32 backup stream
	var inStream bytes.Buffer
	bsw := winio.NewBackupStreamWriter(&inStream)
	err := bsw.WriteHeader(&winio.BackupHeader{
		Id:   winio.BackupReparseData,
		Size: int64(len(originalRPBytes)),
	})
	if err != nil {
		t.Fatalf("WriteHeader failed: %v", err)
	}
	if _, err := bsw.Write(originalRPBytes); err != nil {
		t.Fatalf("bsw.Write failed: %v", err)
	}

	// Run through fixReparseBackupStream
	fixedStream, err := fixReparseBackupStream(bytes.NewReader(inStream.Bytes()))
	if err != nil {
		t.Fatalf("fixReparseBackupStream failed: %v", err)
	}

	// Read the output stream
	fixedReader := winio.NewBackupStreamReader(fixedStream)
	fixedHdr, err := fixedReader.Next()
	if err != nil {
		t.Fatalf("fixedReader.Next() failed: %v", err)
	}
	if fixedHdr.Id != winio.BackupReparseData {
		t.Fatalf("expected BackupReparseData stream, got %d", fixedHdr.Id)
	}
	fixedData, err := io.ReadAll(fixedReader)
	if err != nil {
		t.Fatalf("reading fixedData: %v", err)
	}

	// The bytes MUST be 100% identical to the original bytes (no flag or metadata loss)
	if !bytes.Equal(fixedData, originalRPBytes) {
		t.Fatalf("fixReparseBackupStream altered symlink bytes!\ngot:  %x\nwant: %x", fixedData, originalRPBytes)
	}

	// Verify decoding still preserves target and relative nature
	decodedRP, err := winio.DecodeReparsePoint(fixedData)
	if err != nil {
		t.Fatalf("DecodeReparsePoint failed: %v", err)
	}
	if decodedRP.Target != symlinkTarget {
		t.Errorf("decodedRP.Target = %q, want %q", decodedRP.Target, symlinkTarget)
	}
	if decodedRP.IsMountPoint {
		t.Errorf("expected IsMountPoint to be false for symlink")
	}
}

func TestFixReparseBackupStreamValidMountPointUntouched(t *testing.T) {
	// Construct a valid mount point where PrintName is already populated (e.g. mklink /J)
	rp := &winio.ReparsePoint{
		Target:       `C:\Windows`,
		IsMountPoint: true,
	}
	originalRPBytes := winio.EncodeReparsePoint(rp)

	// Pack into a Win32 backup stream
	var inStream bytes.Buffer
	bsw := winio.NewBackupStreamWriter(&inStream)
	err := bsw.WriteHeader(&winio.BackupHeader{
		Id:   winio.BackupReparseData,
		Size: int64(len(originalRPBytes)),
	})
	if err != nil {
		t.Fatalf("WriteHeader failed: %v", err)
	}
	if _, err := bsw.Write(originalRPBytes); err != nil {
		t.Fatalf("bsw.Write failed: %v", err)
	}

	// Run through fixReparseBackupStream
	fixedStream, err := fixReparseBackupStream(bytes.NewReader(inStream.Bytes()))
	if err != nil {
		t.Fatalf("fixReparseBackupStream failed: %v", err)
	}

	// Read the output stream
	fixedReader := winio.NewBackupStreamReader(fixedStream)
	fixedHdr, err := fixedReader.Next()
	if err != nil {
		t.Fatalf("fixedReader.Next() failed: %v", err)
	}
	if fixedHdr.Id != winio.BackupReparseData {
		t.Fatalf("expected BackupReparseData stream, got %d", fixedHdr.Id)
	}
	fixedData, err := io.ReadAll(fixedReader)
	if err != nil {
		t.Fatalf("reading fixedData: %v", err)
	}

	// Must be 100% byte-for-byte identical
	if !bytes.Equal(fixedData, originalRPBytes) {
		t.Fatalf("fixReparseBackupStream altered valid mount point bytes!\ngot:  %x\nwant: %x", fixedData, originalRPBytes)
	}
}
