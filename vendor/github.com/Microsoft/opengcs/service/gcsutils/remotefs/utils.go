package remotefs

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"syscall"

	"github.com/docker/docker/pkg/archive"
)

// ReadError is an utility function that reads a serialized error from the given reader
// and deserializes it.
func ReadError(in io.Reader) (*ExportedError, error) {
	b, err := ioutil.ReadAll(in)
	if err != nil {
		return nil, err
	}

	// No error
	if len(b) == 0 {
		return nil, nil
	}

	var exportedErr ExportedError
	if err := json.Unmarshal(b, &exportedErr); err != nil {
		return nil, err
	}

	return &exportedErr, nil
}

// ExportedToError will convert a ExportedError to an error. It will try to match
// the error to any existing known error like os.ErrNotExist. Otherwise, it will just
// return an implementation of the error interface.
func ExportedToError(ee *ExportedError) error {
	if ee.Error() == os.ErrNotExist.Error() {
		return os.ErrNotExist
	} else if ee.Error() == os.ErrExist.Error() {
		return os.ErrExist
	} else if ee.Error() == os.ErrPermission.Error() {
		return os.ErrPermission
	}
	return ee
}

// WriteError is an utility function that serializes the error
// and writes it to the output writer.
func WriteError(err error, out io.Writer) error {
	if err == nil {
		return nil
	}
	err = fixOSError(err)

	var errno int
	switch typedError := err.(type) {
	case *os.PathError:
		if se, ok := typedError.Err.(syscall.Errno); ok {
			errno = int(se)
		}
	case *os.LinkError:
		if se, ok := typedError.Err.(syscall.Errno); ok {
			errno = int(se)
		}
	case *os.SyscallError:
		if se, ok := typedError.Err.(syscall.Errno); ok {
			errno = int(se)
		}
	}

	exportedError := &ExportedError{
		ErrString: err.Error(),
		ErrNum:    errno,
	}

	b, err1 := json.Marshal(exportedError)
	if err1 != nil {
		return err1
	}

	_, err1 = out.Write(b)
	if err1 != nil {
		return err1
	}
	return nil
}

// fixOSError converts possible platform dependent error into the portable errors in the
// Go os package if possible.
func fixOSError(err error) error {
	// The os.IsExist, os.IsNotExist, and os.IsPermissions functions are platform
	// dependent, so sending the raw error might break those functions on a different OS.
	// Go defines portable errors for these.
	if os.IsExist(err) {
		return os.ErrExist
	} else if os.IsNotExist(err) {
		return os.ErrNotExist
	} else if os.IsPermission(err) {
		return os.ErrPermission
	}
	return err
}

// ReadTarOptions reads from the specified reader and deserializes an archive.TarOptions struct.
func ReadTarOptions(r io.Reader) (*archive.TarOptions, error) {
	var size uint64
	if err := binary.Read(r, binary.BigEndian, &size); err != nil {
		return nil, err
	}

	rawJSON := make([]byte, size)
	if _, err := io.ReadFull(r, rawJSON); err != nil {
		return nil, err
	}

	var opts archive.TarOptions
	if err := json.Unmarshal(rawJSON, &opts); err != nil {
		return nil, err
	}
	return &opts, nil
}

// WriteTarOptions serializes a archive.TarOptions struct and writes it to the writer.
func WriteTarOptions(w io.Writer, opts *archive.TarOptions) error {
	optsBuf, err := json.Marshal(opts)
	if err != nil {
		return err
	}

	optsSize := uint64(len(optsBuf))
	optsSizeBuf := &bytes.Buffer{}
	if err := binary.Write(optsSizeBuf, binary.BigEndian, optsSize); err != nil {
		return err
	}

	if _, err := optsSizeBuf.WriteTo(w); err != nil {
		return err
	}

	if _, err := w.Write(optsBuf); err != nil {
		return err
	}

	return nil
}

// ReadFileHeader reads from r and returns a deserialized FileHeader
func ReadFileHeader(r io.Reader) (*FileHeader, error) {
	hdr := &FileHeader{}
	if err := binary.Read(r, binary.BigEndian, hdr); err != nil {
		return nil, err
	}
	return hdr, nil
}

// WriteFileHeader serializes a FileHeader and writes it to w, along with any extra data
func WriteFileHeader(w io.Writer, hdr *FileHeader, extraData []byte) error {
	if err := binary.Write(w, binary.BigEndian, hdr); err != nil {
		return err
	}
	if _, err := w.Write(extraData); err != nil {
		return err
	}
	return nil
}
