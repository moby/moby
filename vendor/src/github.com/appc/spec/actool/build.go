package main

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/appc/spec/aci"
	"github.com/appc/spec/schema"
)

var (
	buildNocompress bool
	buildOverwrite  bool
	cmdBuild        = &Command{
		Name: "build",
		Description: `Build an ACI from a given directory. The directory should
contain an Image Layout. The Image Layout will be validated
before the ACI is created. The produced ACI will be
gzip-compressed by default.`,
		Summary: "Build an ACI from an Image Layout (experimental)",
		Usage:   `[--overwrite] [--no-compression] DIRECTORY OUTPUT_FILE`,
		Run:     runBuild,
	}
)

func init() {
	cmdBuild.Flags.BoolVar(&buildOverwrite, "overwrite", false, "Overwrite target file if it already exists")
	cmdBuild.Flags.BoolVar(&buildNocompress, "no-compression", false, "Do not gzip-compress the produced ACI")
}

func runBuild(args []string) (exit int) {
	if len(args) != 2 {
		stderr("build: Must provide directory and output file")
		return 1
	}

	root := args[0]
	tgt := args[1]
	ext := filepath.Ext(tgt)
	if ext != schema.ACIExtension {
		stderr("build: Extension must be %s (given %s)", schema.ACIExtension, ext)
		return 1
	}

	mode := os.O_CREATE | os.O_WRONLY
	if buildOverwrite {
		mode |= os.O_TRUNC
	} else {
		mode |= os.O_EXCL
	}
	fh, err := os.OpenFile(tgt, mode, 0644)
	if err != nil {
		if os.IsExist(err) {
			stderr("build: Target file exists (try --overwrite)")
		} else {
			stderr("build: Unable to open target %s: %v", tgt, err)
		}
		return 1
	}

	var gw *gzip.Writer
	var r io.WriteCloser = fh
	if !buildNocompress {
		gw = gzip.NewWriter(fh)
		r = gw
	}
	tr := tar.NewWriter(r)

	defer func() {
		tr.Close()
		if !buildNocompress {
			gw.Close()
		}
		fh.Close()
		if exit != 0 && !buildOverwrite {
			os.Remove(tgt)
		}
	}()

	// TODO(jonboulle): stream the validation so we don't have to walk the rootfs twice
	if err := aci.ValidateLayout(root); err != nil {
		stderr("build: Layout failed validation: %v", err)
		return 1
	}
	mpath := filepath.Join(root, aci.ManifestFile)
	b, err := ioutil.ReadFile(mpath)
	if err != nil {
		stderr("build: Unable to read Image Manifest: %v", err)
		return 1
	}
	var im schema.ImageManifest
	if err := im.UnmarshalJSON(b); err != nil {
		stderr("build: Unable to load Image Manifest: %v", err)
		return 1
	}
	iw := aci.NewImageWriter(im, tr)

	err = filepath.Walk(root, aci.BuildWalker(root, iw))
	if err != nil {
		stderr("build: Error walking rootfs: %v", err)
		return 1
	}

	err = iw.Close()
	if err != nil {
		stderr("build: Unable to close image %s: %v", tgt, err)
		return 1
	}

	return
}
