package main

import (
	"archive/tar"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/appc/spec/aci"
	"github.com/appc/spec/schema"
)

const (
	typeAppImage    = "appimage"
	typeImageLayout = "layout"
	typeManifest    = "manifest"
)

var (
	valType     string
	cmdValidate = &Command{
		Name:        "validate",
		Description: "Validate one or more AppContainer files",
		Summary:     "Validate that one or more images or manifests meet the AppContainer specification",
		Usage:       "[--type=TYPE] FILE...",
		Run:         runValidate,
	}
	types = []string{
		typeAppImage,
		typeImageLayout,
		typeManifest,
	}
)

func init() {
	cmdValidate.Flags.StringVar(&valType, "type", "",
		fmt.Sprintf(`Type of file to validate. If unset, actool will try to detect the type. One of "%s"`, strings.Join(types, ",")))
}

func runValidate(args []string) (exit int) {
	if len(args) < 1 {
		stderr("must pass one or more files")
		return 1
	}

	for _, path := range args {
		vt := valType
		fi, err := os.Stat(path)
		if err != nil {
			stderr("unable to access %s: %v", path, err)
			return 1
		}
		var fh *os.File
		if fi.IsDir() {
			switch vt {
			case typeImageLayout:
			case "":
				vt = typeImageLayout
			case typeManifest, typeAppImage:
				stderr("%s is a directory (wrong --type?)", path)
				return 1
			default:
				// should never happen
				panic(fmt.Sprintf("unexpected type: %v", vt))
			}
		} else {
			fh, err = os.Open(path)
			if err != nil {
				stderr("%s: unable to open: %v", path, err)
				return 1
			}
		}

		if vt == "" {
			vt, err = detectValType(fh)
			if err != nil {
				stderr("%s: error detecting file type: %v", path, err)
				return 1
			}
		}
		switch vt {
		case typeImageLayout:
			err = aci.ValidateLayout(path)
			if err != nil {
				stderr("%s: invalid image layout: %v", path, err)
				exit = 1
			} else if globalFlags.Debug {
				stderr("%s: valid image layout", path)
			}
		case typeAppImage:
			fr, err := maybeDecompress(fh)
			if err != nil {
				stderr("%s: error decompressing file: %v", path, err)
				return 1
			}
			tr := tar.NewReader(fr)
			err = aci.ValidateArchive(tr)
			fh.Close()
			if err != nil {
				stderr("%s: error validating: %v", path, err)
				exit = 1
			} else if globalFlags.Debug {
				stderr("%s: valid app container image", path)
			}
		case typeManifest:
			b, err := ioutil.ReadAll(fh)
			fh.Close()
			if err != nil {
				stderr("%s: unable to read file %s", path, err)
				return 1
			}
			k := schema.Kind{}
			if err := k.UnmarshalJSON(b); err != nil {
				stderr("%s: error unmarshaling manifest: %v", path, err)
				return 1
			}
			switch k.ACKind {
			case "ImageManifest":
				m := schema.ImageManifest{}
				err = m.UnmarshalJSON(b)
			case "ContainerRuntimeManifest":
				m := schema.ContainerRuntimeManifest{}
				err = m.UnmarshalJSON(b)
			default:
				// Should not get here; schema.Kind unmarshal should fail
				panic("bad ACKind")
			}
			if err != nil {
				stderr("%s: invalid %s: %v", path, k.ACKind, err)
				exit = 1
			} else if globalFlags.Debug {
				stderr("%s: valid %s", path, k.ACKind)
			}
		default:
			stderr("%s: unable to detect filetype (try --type)", path)
			return 1
		}
	}

	return
}

func detectValType(file *os.File) (string, error) {
	typ, err := aci.DetectFileType(file)
	if err != nil {
		return "", err
	}
	if _, err := file.Seek(0, 0); err != nil {
		return "", err
	}
	switch typ {
	case aci.TypeXz, aci.TypeGzip, aci.TypeBzip2, aci.TypeTar:
		return typeAppImage, nil
	case aci.TypeText:
		return typeManifest, nil
	default:
		return "", nil
	}
}

func maybeDecompress(rs io.ReadSeeker) (io.Reader, error) {
	// TODO(jonboulle): this is a bit redundant with detectValType
	typ, err := aci.DetectFileType(rs)
	if err != nil {
		return nil, err
	}
	if _, err := rs.Seek(0, 0); err != nil {
		return nil, err
	}
	var r io.Reader
	switch typ {
	case aci.TypeGzip:
		r, err = gzip.NewReader(rs)
		if err != nil {
			return nil, fmt.Errorf("error reading gzip: %v", err)
		}
	case aci.TypeBzip2:
		r = bzip2.NewReader(rs)
	case aci.TypeXz:
		r = aci.XzReader(rs)
	case aci.TypeTar:
		r = rs
	case aci.TypeUnknown:
		return nil, errors.New("unknown filetype")
	default:
		// should never happen
		panic(fmt.Sprintf("bad type returned from DetectFileType: %v", typ))
	}
	return r, nil
}
