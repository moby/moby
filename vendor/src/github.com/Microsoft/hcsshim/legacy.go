package hcsshim

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Microsoft/go-winio"
)

var errorIterationCanceled = errors.New("")

func openFileOrDir(path string, mode uint32, createDisposition uint32) (file *os.File, err error) {
	winPath, err := syscall.UTF16FromString(path)
	if err != nil {
		return
	}
	h, err := syscall.CreateFile(&winPath[0], mode, syscall.FILE_SHARE_READ, nil, createDisposition, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		err = &os.PathError{"open", path, err}
		return
	}
	file = os.NewFile(uintptr(h), path)
	return
}

func makeLongAbsPath(path string) (string, error) {
	if strings.HasPrefix(path, `\\?\`) || strings.HasPrefix(path, `\\.\`) {
		return path, nil
	}
	if !filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		path = absPath
	}
	if strings.HasPrefix(path, `\\`) {
		return `\\?\UNC\` + path[2:], nil
	}
	return `\\?\` + path, nil
}

type fileEntry struct {
	path string
	fi   os.FileInfo
	err  error
}

type LegacyLayerReader struct {
	root         string
	result       chan *fileEntry
	proceed      chan bool
	currentFile  *os.File
	backupReader *winio.BackupFileReader
	isTP4Format  bool
}

// NewLegacyLayerReader returns a new LayerReader that can read the Windows
// TP4 transport format from disk.
func NewLegacyLayerReader(root string) *LegacyLayerReader {
	r := &LegacyLayerReader{
		root:        root,
		result:      make(chan *fileEntry),
		proceed:     make(chan bool),
		isTP4Format: IsTP4(),
	}
	go r.walk()
	return r
}

func readTombstones(path string) (map[string]([]string), error) {
	tf, err := os.Open(filepath.Join(path, "tombstones.txt"))
	if err != nil {
		return nil, err
	}
	defer tf.Close()
	s := bufio.NewScanner(tf)
	if !s.Scan() || s.Text() != "\xef\xbb\xbfVersion 1.0" {
		return nil, errors.New("Invalid tombstones file")
	}

	ts := make(map[string]([]string))
	for s.Scan() {
		t := s.Text()[1:] // skip leading `\`
		dir := filepath.Dir(t)
		ts[dir] = append(ts[dir], t)
	}
	if err = s.Err(); err != nil {
		return nil, err
	}

	return ts, nil
}

func (r *LegacyLayerReader) walkUntilCancelled() error {
	root, err := makeLongAbsPath(r.root)
	if err != nil {
		return err
	}

	r.root = root
	ts, err := readTombstones(r.root)
	if err != nil {
		return err
	}

	err = filepath.Walk(r.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == r.root || path == filepath.Join(r.root, "tombstones.txt") || strings.HasSuffix(path, ".$wcidirs$") {
			return nil
		}
		r.result <- &fileEntry{path, info, nil}
		if !<-r.proceed {
			return errorIterationCanceled
		}

		// List all the tombstones.
		if info.IsDir() {
			relPath, err := filepath.Rel(r.root, path)
			if err != nil {
				return err
			}
			if dts, ok := ts[relPath]; ok {
				for _, t := range dts {
					r.result <- &fileEntry{t, nil, nil}
					if !<-r.proceed {
						return errorIterationCanceled
					}
				}
			}
		}
		return nil
	})
	if err == errorIterationCanceled {
		return nil
	}
	if err == nil {
		return io.EOF
	}
	return err
}

func (r *LegacyLayerReader) walk() {
	defer close(r.result)
	if !<-r.proceed {
		return
	}

	err := r.walkUntilCancelled()
	if err != nil {
		for {
			r.result <- &fileEntry{err: err}
			if !<-r.proceed {
				return
			}
		}
	}
}

func (r *LegacyLayerReader) reset() {
	if r.backupReader != nil {
		r.backupReader.Close()
		r.backupReader = nil
	}
	if r.currentFile != nil {
		r.currentFile.Close()
		r.currentFile = nil
	}
}

func findBackupStreamSize(r io.Reader) (int64, error) {
	br := winio.NewBackupStreamReader(r)
	for {
		hdr, err := br.Next()
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return 0, err
		}
		if hdr.Id == winio.BackupData {
			return hdr.Size, nil
		}
	}
}

func (r *LegacyLayerReader) Next() (path string, size int64, fileInfo *winio.FileBasicInfo, err error) {
	r.reset()
	r.proceed <- true
	fe := <-r.result
	if fe == nil {
		err = errors.New("LegacyLayerReader closed")
		return
	}
	if fe.err != nil {
		err = fe.err
		return
	}

	path, err = filepath.Rel(r.root, fe.path)
	if err != nil {
		return
	}

	if fe.fi == nil {
		// This is a tombstone. Return a nil fileInfo.
		return
	}

	if fe.fi.IsDir() && strings.HasPrefix(path, `Files\`) {
		fe.path += ".$wcidirs$"
	}

	f, err := openFileOrDir(fe.path, syscall.GENERIC_READ, syscall.OPEN_EXISTING)
	if err != nil {
		return
	}
	defer func() {
		if f != nil {
			f.Close()
		}
	}()

	fileInfo, err = winio.GetFileBasicInfo(f)
	if err != nil {
		return
	}

	if !strings.HasPrefix(path, `Files\`) {
		size = fe.fi.Size()
		r.backupReader = winio.NewBackupFileReader(f, false)
		if path == "Hives" || path == "Files" {
			// The Hives directory has a non-deterministic file time because of the
			// nature of the import process. Use the times from System_Delta.
			var g *os.File
			g, err = os.Open(filepath.Join(r.root, `Hives\System_Delta`))
			if err != nil {
				return
			}
			attr := fileInfo.FileAttributes
			fileInfo, err = winio.GetFileBasicInfo(g)
			g.Close()
			if err != nil {
				return
			}
			fileInfo.FileAttributes = attr
		}

		// The creation time and access time get reset for files outside of the Files path.
		fileInfo.CreationTime = fileInfo.LastWriteTime
		fileInfo.LastAccessTime = fileInfo.LastWriteTime

	} else {
		beginning := int64(0)
		if !r.isTP4Format {
			// In TP5, the file attributes were added before the backup stream
			var attr uint32
			err = binary.Read(f, binary.LittleEndian, &attr)
			if err != nil {
				return
			}
			fileInfo.FileAttributes = uintptr(attr)
			beginning = 4
		}

		// Find the accurate file size.
		if !fe.fi.IsDir() {
			size, err = findBackupStreamSize(f)
			if err != nil {
				err = &os.PathError{"findBackupStreamSize", fe.path, err}
				return
			}
		}

		// Return back to the beginning of the backup stream.
		_, err = f.Seek(beginning, 0)
		if err != nil {
			return
		}
	}

	r.currentFile = f
	f = nil
	return
}

func (r *LegacyLayerReader) Read(b []byte) (int, error) {
	if r.backupReader == nil {
		if r.currentFile == nil {
			return 0, io.EOF
		}
		return r.currentFile.Read(b)
	}
	return r.backupReader.Read(b)
}

func (r *LegacyLayerReader) Close() error {
	r.proceed <- false
	<-r.result
	r.reset()
	return nil
}

type LegacyLayerWriter struct {
	root         string
	currentFile  *os.File
	backupWriter *winio.BackupFileWriter
	tombstones   []string
	isTP4Format  bool
	pathFixed    bool
}

// NewLegacyLayerWriter returns a LayerWriter that can write the TP4 transport format
// to disk.
func NewLegacyLayerWriter(root string) *LegacyLayerWriter {
	return &LegacyLayerWriter{
		root:        root,
		isTP4Format: IsTP4(),
	}
}

func (w *LegacyLayerWriter) init() error {
	if !w.pathFixed {
		path, err := makeLongAbsPath(w.root)
		if err != nil {
			return err
		}
		w.root = path
		w.pathFixed = true
	}
	return nil
}

func (w *LegacyLayerWriter) reset() {
	if w.backupWriter != nil {
		w.backupWriter.Close()
		w.backupWriter = nil
	}
	if w.currentFile != nil {
		w.currentFile.Close()
		w.currentFile = nil
	}
}

func (w *LegacyLayerWriter) Add(name string, fileInfo *winio.FileBasicInfo) error {
	w.reset()
	err := w.init()
	if err != nil {
		return err
	}
	path := filepath.Join(w.root, name)

	createDisposition := uint32(syscall.CREATE_NEW)
	if (fileInfo.FileAttributes & syscall.FILE_ATTRIBUTE_DIRECTORY) != 0 {
		err := os.Mkdir(path, 0)
		if err != nil {
			return err
		}
		path += ".$wcidirs$"
	}

	f, err := openFileOrDir(path, syscall.GENERIC_READ|syscall.GENERIC_WRITE, createDisposition)
	if err != nil {
		return err
	}
	defer func() {
		if f != nil {
			f.Close()
			os.Remove(path)
		}
	}()

	strippedFi := *fileInfo
	strippedFi.FileAttributes = 0
	err = winio.SetFileBasicInfo(f, &strippedFi)
	if err != nil {
		return err
	}

	if strings.HasPrefix(name, `Hives\`) {
		w.backupWriter = winio.NewBackupFileWriter(f, false)
	} else {
		if !w.isTP4Format {
			// In TP5, the file attributes were added to the header
			err = binary.Write(f, binary.LittleEndian, uint32(fileInfo.FileAttributes))
			if err != nil {
				return err
			}
		}
	}

	w.currentFile = f
	f = nil
	return nil
}

func (w *LegacyLayerWriter) Remove(name string) error {
	w.tombstones = append(w.tombstones, name)
	return nil
}

func (w *LegacyLayerWriter) Write(b []byte) (int, error) {
	if w.backupWriter == nil {
		if w.currentFile == nil {
			return 0, errors.New("closed")
		}
		return w.currentFile.Write(b)
	}
	return w.backupWriter.Write(b)
}

func (w *LegacyLayerWriter) Close() error {
	w.reset()
	err := w.init()
	if err != nil {
		return err
	}
	tf, err := os.Create(filepath.Join(w.root, "tombstones.txt"))
	if err != nil {
		return err
	}
	defer tf.Close()
	_, err = tf.Write([]byte("\xef\xbb\xbfVersion 1.0\n"))
	if err != nil {
		return err
	}
	for _, t := range w.tombstones {
		_, err = tf.Write([]byte(filepath.Join(`\`, t) + "\n"))
		if err != nil {
			return err
		}
	}
	return nil
}
