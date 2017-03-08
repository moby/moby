package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
)

const (
	bios = "mobylinux/mkimage-iso-bios:489b1f054a77a8f379d0bfc6cd91639b4db6b67c@sha256:0f058951aac4367d132682aa19eeb5cdcb05600a5d51fe5d0fcbd97b03ae4f87"
	efi  = "mobylinux/mkimage-iso-efi:b210c58e096e53082d35b28fa2b52dba6ae200c8@sha256:10c2789bf5fbd27c35c5fe2f3b97f75a7108bbde389d0f5ed750e3e2dae95376"
)

func outputs(m *Moby, base string, bzimage []byte, initrd []byte) error {
	for _, o := range m.Outputs {
		switch o.Format {
		case "kernel+initrd":
			err := outputKernelInitrd(base, bzimage, initrd)
			if err != nil {
				return fmt.Errorf("Error writing %s output: %v", o.Format, err)
			}
		case "iso-bios":
			err := outputISO(bios, base+".iso", bzimage, initrd, m.Kernel.Cmdline)
			if err != nil {
				return fmt.Errorf("Error writing %s output: %v", o.Format, err)
			}
		case "iso-efi":
			err := outputISO(efi, base+"-efi.iso", bzimage, initrd, m.Kernel.Cmdline)
			if err != nil {
				return fmt.Errorf("Error writing %s output: %v", o.Format, err)
			}
		case "":
			return fmt.Errorf("No format specified for output")
		default:
			return fmt.Errorf("Unknown output type %s", o.Format)
		}
	}
	return nil
}

// TODO add kernel command line
func outputISO(image, filename string, bzimage []byte, initrd []byte, args ...string) error {
	// first build the input tarball from kernel and initrd
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	hdr := &tar.Header{
		Name: "bzImage",
		Mode: 0600,
		Size: int64(len(bzimage)),
	}
	err := tw.WriteHeader(hdr)
	if err != nil {
		return err
	}
	_, err = tw.Write(bzimage)
	if err != nil {
		return err
	}
	hdr = &tar.Header{
		Name: "initrd.img",
		Mode: 0600,
		Size: int64(len(initrd)),
	}
	err = tw.WriteHeader(hdr)
	if err != nil {
		return err
	}
	_, err = tw.Write(initrd)
	if err != nil {
		return err
	}
	err = tw.Close()
	if err != nil {
		return err
	}
	iso, err := dockerRunInput(buf, append([]string{image}, args...)...)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filename, iso, os.FileMode(0644))
	if err != nil {
		return err
	}
	fmt.Println(filename)
	return nil
}

func outputKernelInitrd(base string, bzimage []byte, initrd []byte) error {
	err := ioutil.WriteFile(base+"-initrd.img", initrd, os.FileMode(0644))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(base+"-bzImage", bzimage, os.FileMode(0644))
	if err != nil {
		return err
	}
	fmt.Println(base + "-bzImage " + base + "-initrd.img")
	return nil
}
