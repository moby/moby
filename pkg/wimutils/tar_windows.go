package wimutils

import (
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"path/filepath"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/go-winio/archive/tar"
	"github.com/Microsoft/go-winio/wim"
)

func writeFile(w *tar.Writer, f *wim.File, p string, linkData map[int64]string) error {
	var err error
	hdr := &tar.Header{
		Name:         path.Join(p, f.Name),
		Mode:         0644,
		ModTime:      f.LastWriteTime.Time(),
		AccessTime:   f.LastAccessTime.Time(),
		CreationTime: f.CreationTime.Time(),
		Typeflag:     tar.TypeReg,
		Winheaders:   make(map[string]string),
	}
	hdr.Winheaders["fileattr"] = fmt.Sprintf("%d", f.Attributes)
	if len(f.SecurityDescriptor) > 0 {
		hdr.Winheaders["sd"], err = winio.SecurityDescriptorToSddl(f.SecurityDescriptor)
		if err != nil {
			return err
		}
	}
	if f.IsDir() {
		hdr.Typeflag = tar.TypeDir
		hdr.Mode |= 0111
	} else if f.Attributes&wim.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		hdr.Typeflag = tar.TypeSymlink
		r, err := f.Open()
		if err != nil {
			return err
		}
		data, err := ioutil.ReadAll(r)
		if err != nil {
			return err
		}
		rp, err := winio.DecodeReparsePointData(f.ReparseTag, data)
		if err != nil {
			return err
		}
		hdr.Linkname = filepath.ToSlash(rp.Target)
		if rp.IsMountPoint {
			hdr.Winheaders["mountpoint"] = "1"
		}
	} else {
		hdr.Size = f.Size
		// Files that share the same LinkID are hard linked together.
		// Include the first such file as a regular file, and subsequent
		// ones as links.
		if f.LinkID != 0 {
			if linkName, ok := linkData[f.LinkID]; ok {
				hdr.Size = 0
				hdr.Typeflag = tar.TypeLink
				hdr.Linkname = linkName
			} else {
				linkData[f.LinkID] = hdr.Name
			}
		}
	}
	err = w.WriteHeader(hdr)
	if err != nil {
		return err
	}

	// Copy the file data.
	if hdr.Size != 0 {
		r, err := f.Open()
		if err != nil {
			return err
		}
		_, err = io.Copy(w, r)
		if err != nil {
			return err
		}
	}

	// Write alternate data streams.
	for _, s := range f.Streams {
		if s.Name == "" {
			continue
		}
		shdr := &tar.Header{}
		*shdr = *hdr
		hdr.Size = s.Size
		hdr.Typeflag = tar.TypeReg
		hdr.Name += ":" + s.Name
		hdr.Winheaders = nil
		err = w.WriteHeader(hdr)
		if err != nil {
			return err
		}
		if s.Size != 0 {
			r, err := s.Open()
			if err != nil {
				return err
			}
			_, err = io.Copy(w, r)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func writeDirectory(w *tar.Writer, d *wim.File, p string, linkData map[int64]string) error {
	files, err := d.Readdir()
	if err != nil {
		return err
	}
	for _, f := range files {
		err = writeFile(w, f, p, linkData)
		if err != nil {
			return err
		}
		if f.IsDir() {
			err = writeDirectory(w, f, path.Join(p, f.Name), linkData)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func writeTarFromImage(w io.Writer, img *wim.Image, uimg *wim.Image) error {
	root, err := img.Open()
	if err != nil {
		return err
	}
	tw := tar.NewWriter(w)

	// For now, put WIM contents under a directory called Files, since
	// this is what the Windows graph driver expects.
	hdr := &tar.Header{
		Name:     "Files",
		Mode:     0755,
		ModTime:  root.LastWriteTime.Time(),
		Typeflag: tar.TypeDir,
	}
	err = tw.WriteHeader(hdr)
	if err != nil {
		return err
	}

	err = writeDirectory(tw, root, "Files", make(map[int64]string))
	if err != nil {
		return err
	}

	// Include the utility VM image if it is present.
	if uimg != nil {
		uroot, err := uimg.Open()
		if err != nil {
			return err
		}

		hdr.Name = "UtilityVM"
		err = tw.WriteHeader(hdr)
		if err != nil {
			return err
		}

		hdr.Name = "UtilityVM/Files"
		err = tw.WriteHeader(hdr)
		if err != nil {
			return err
		}

		err = writeDirectory(tw, uroot, "UtilityVM/Files", make(map[int64]string))
		if err != nil {
			return err
		}
	}

	err = tw.Flush()
	if err != nil {
		return err
	}

	return nil
}

// TarFromWIM returns a reader that yields the contents of a WIM file in tar format.
func TarFromWIM(r io.ReaderAt) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		wr, err := wim.NewReader(r)
		if err == nil {
			var utilityVM *wim.Image
			if len(wr.Image) > 1 && wr.Image[1].Name == "UtilityVM" {
				utilityVM = wr.Image[1]
			}
			err = writeTarFromImage(pw, wr.Image[0], utilityVM)
		}
		pw.CloseWithError(err)
	}()

	return pr, nil
}
