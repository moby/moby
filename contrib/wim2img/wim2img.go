// This program takes as input a path to a WIM file containing a Windows container base image
// and produces as output a tar file that can be passed to docker load.
package main

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/Microsoft/go-winio/wim"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/wimutils"
	"github.com/docker/engine-api/types/container"
)

type manifest struct {
	Config   string
	RepoTags []string
	Layers   []string
}

type config struct {
	Repo      string
	Cmd       string
	TagLatest bool
	TarLayer  bool
}

func writeTar(wimfile *os.File, out io.Writer, wimimg *wim.ImageInfo, c *config) error {
	if wimimg.Windows == nil {
		return errors.New("not a Windows image")
	}

	layerFilename := "layer.wim"
	if c.TarLayer {
		layerFilename = "layer.tar"
	}

	name := c.Repo
	if name == "" {
		name = strings.ToLower(wimimg.Name)
	}

	m := []manifest{
		{
			Config: "config.json",
			Layers: []string{layerFilename},
		},
	}

	v := &wimimg.Windows.Version
	m[0].RepoTags = append(m[0].RepoTags, fmt.Sprintf("%s:%d.%d.%d.%d", name, v.Major, v.Minor, v.Build, v.SPBuild))

	if c.TagLatest {
		m[0].RepoTags = append(m[0].RepoTags, name+":latest")
	}

	mj, err := json.Marshal(m)
	if err != nil {
		return err
	}

	tw := tar.NewWriter(out)
	hdr := tar.Header{
		Name:     "manifest.json",
		Mode:     0644,
		Typeflag: tar.TypeReg,
		Size:     int64(len(mj)),
	}

	err = tw.WriteHeader(&hdr)
	if err != nil {
		return err
	}

	_, err = tw.Write(mj)
	if err != nil {
		return err
	}

	layerFile := wimfile
	if c.TarLayer {
		r, err := wimutils.TarFromWIM(wimfile)
		if err != nil {
			return err
		}
		defer r.Close()
		layerFile, err = ioutil.TempFile("", "tar")
		if err != nil {
			return err
		}
		defer os.Remove(layerFile.Name())
		defer layerFile.Close()
		buf := make([]byte, 1024*1024)
		_, err = io.CopyBuffer(layerFile, r, buf)
		if err != nil {
			return err
		}
		_, err = layerFile.Seek(0, 0)
		if err != nil {
			return err
		}
	}

	s, err := layerFile.Stat()
	if err != nil {
		return err
	}

	hdr = tar.Header{
		Name:     layerFilename,
		Mode:     0644,
		Typeflag: tar.TypeReg,
		Size:     s.Size(),
	}

	err = tw.WriteHeader(&hdr)
	if err != nil {
		return err
	}

	d := digest.Canonical.New()
	wimr := io.TeeReader(layerFile, d.Hash())
	_, err = io.Copy(tw, wimr)
	if err != nil {
		return err
	}

	img := image.Image{
		V1Image: image.V1Image{
			OS:      "windows",
			Created: wimimg.CreationTime.Time(),
		},
		RootFS: &image.RootFS{
			Type: image.TypeLayers,
			DiffIDs: []layer.DiffID{
				layer.DiffID(d.Digest()),
			},
		},
		OSVersion: fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Build),
	}

	if c.Cmd != "" {
		img.Config = &container.Config{
			Cmd: []string{c.Cmd},
		}
	}

	switch wimimg.Windows.Arch {
	case wim.PROCESSOR_ARCHITECTURE_AMD64:
		img.Architecture = "amd64"
	default:
		return fmt.Errorf("unknown architecture value %d", wimimg.Windows.Arch)
	}

	if wimimg.Name != "NanoServer" {
		img.OSFeatures = append(img.OSFeatures, image.FeatureWin32k)
	}

	imgj, err := img.MarshalJSON()
	if err != nil {
		return err
	}

	hdr = tar.Header{
		Name:     "config.json",
		Mode:     0644,
		Typeflag: tar.TypeReg,
		Size:     int64(len(imgj)),
	}

	err = tw.WriteHeader(&hdr)
	if err != nil {
		return err
	}

	_, err = tw.Write(imgj)
	if err != nil {
		return err
	}

	err = tw.Flush()
	if err != nil {
		return err
	}

	return nil
}

func main() {
	c := &config{}

	outname := flag.String("out", "", "Output tar file. If not present, writes to stdout.")
	flag.BoolVar(&c.TagLatest, "latest", false, "Include a :latest tag on the image.")
	flag.BoolVar(&c.TarLayer, "tarlayer", false, "Convert the WIM to a tar layer.")
	flag.StringVar(&c.Repo, "repo", "", "Repo name (defaults to WIM image name).")
	flag.StringVar(&c.Cmd, "cmd", "", "Default command.")
	flag.Parse()
	filename := flag.Arg(0)

	wimfile, err := os.Open(filename)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer wimfile.Close()

	var out io.Writer = os.Stdout
	if *outname != "" {
		outf, err := os.Create(*outname)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer outf.Close()
		out = outf
	}

	w, err := wim.NewReader(wimfile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	err = writeTar(wimfile, out, &w.Image[0].ImageInfo, c)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
