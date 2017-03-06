package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
)

const (
	bios = "mobylinux/mkimage-iso-bios:6b3ef6d6bdcc5fdf2ee683febac99533c2268c89@sha256:2484146c4dfbd2eee83d9dd3adf84d9232e5dd739d8762275dcd50bf60a529c6"
	efi  = "mobylinux/mkimage-iso-efi:40f35270037dae95584324427e56f829756ff145@sha256:ae5b37ae560a5e030342f3d493d4ad611f2694bcd54eba86bf42ca069da986a7"
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
			err := outputISO(bios, base+".iso", bzimage, initrd)
			if err != nil {
				return fmt.Errorf("Error writing %s output: %v", o.Format, err)
			}
		case "iso-efi":
			err := outputISO(efi, base+"-efi.iso", bzimage, initrd)
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
func outputISO(image, filename string, bzimage []byte, initrd []byte) error {
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
	iso, err := dockerRunInput(buf, image)
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
