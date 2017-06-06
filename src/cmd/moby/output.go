package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"

	log "github.com/Sirupsen/logrus"
)

const (
	bios = "linuxkit/mkimage-iso-bios:6ebdce90f63991eb1d5a578e6570dc1e5781e9fe@sha256:0c6116d4c069d17ebdaa86737841b3be6ae84f6c69a5e79fe59cd8310156aa96"
	efi  = "linuxkit/mkimage-iso-efi:008fac48c41ec38b36ce1ae62f93a69ee9328569@sha256:35282010b95680fe754e557bc65f0b2ffd85e925bd62f427fb77bf494145083b"
	gcp  = "linuxkit/mkimage-gcp:a8b909202c0a0ed2ac31b5c21f6701d3253ff29a@sha256:2ba307e537d6fae37115848c8a0f5a9b3ed578e102c93c5d2578ece4a91cb828"
	qcow = "linuxkit/mkimage-qcow:a1053b5dc80834adcba2e5f49354f62797e35f84@sha256:3312d523a67e7c7efb3c3eaa5a4dfbd46659549681d6d62cdeb02bd475b3a22c"
	vhd  = "linuxkit/mkimage-vhd:98d6c879a52cb85b87269bc6ecf9df7dd134427a@sha256:0ca6f46690c7890c77295cc6c531f95fc8bb41df42c237ae4b32eea338cec4e7"
	vmdk = "linuxkit/mkimage-vmdk:10b8717b6a2099741b702c31af2d9a42ce50425e@sha256:bf7cf6029e61685e9085a1883b1be1167a7f06199f3b76a944ea87b6f23f60d8"
)

func outputs(m *Moby, base string, bzimage []byte, initrd []byte) error {
	log.Debugf("output: %s %s", m.Outputs, base)
	for _, o := range m.Outputs {
		switch o.Format {
		case "kernel+initrd":
			err := outputKernelInitrd(base, bzimage, initrd, m.Kernel.Cmdline)
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
		case "gcp-img":
			err := outputImg(gcp, base+".img.tar.gz", bzimage, initrd, m.Kernel.Cmdline)
			if err != nil {
				return fmt.Errorf("Error writing %s output: %v", o.Format, err)
			}
		case "gcp-storage":
			err := outputImg(gcp, base+".img.tar.gz", bzimage, initrd, m.Kernel.Cmdline)
			if err != nil {
				return fmt.Errorf("Error writing %s output: %v", o.Format, err)
			}
			if o.Bucket == "" {
				return fmt.Errorf("No bucket specified for GCP output")
			}
			gClient, err := NewGCPClient(o.Keys, o.Project)
			if err != nil {
				return fmt.Errorf("Unable to connect to GCP")
			}
			err = gClient.UploadFile(base+".img.tar.gz", base+".img.tar.gz", o.Bucket, o.Public)
			if err != nil {
				return fmt.Errorf("Error copying to Google Storage: %v", err)
			}
		case "gcp":
			err := outputImg(gcp, base+".img.tar.gz", bzimage, initrd, m.Kernel.Cmdline)
			if err != nil {
				return fmt.Errorf("Error writing %s output: %v", o.Format, err)
			}
			if o.Bucket == "" {
				return fmt.Errorf("No bucket specified for GCP output")
			}
			gClient, err := NewGCPClient(o.Keys, o.Project)
			if err != nil {
				return fmt.Errorf("Unable to connect to GCP")
			}
			err = gClient.UploadFile(base+".img.tar.gz", base+".img.tar.gz", o.Bucket, o.Public)
			if err != nil {
				return fmt.Errorf("Error copying to Google Storage: %v", err)
			}
			err = gClient.CreateImage(base, "https://storage.googleapis.com/"+o.Bucket+"/"+base+".img.tar.gz", o.Family, o.Replace)
			if err != nil {
				return fmt.Errorf("Error creating Google Compute Image: %v", err)
			}
		case "qcow", "qcow2":
			err := outputImg(qcow, base+".qcow2", bzimage, initrd, m.Kernel.Cmdline)
			if err != nil {
				return fmt.Errorf("Error writing %s output: %v", o.Format, err)
			}
		case "vhd":
			err := outputImg(vhd, base+".vhd", bzimage, initrd, m.Kernel.Cmdline)
			if err != nil {
				return fmt.Errorf("Error writing %s output: %v", o.Format, err)
			}
		case "vmdk":
			err := outputImg(vmdk, base+".vmdk", bzimage, initrd, m.Kernel.Cmdline)
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

func tarInitrdKernel(bzimage, initrd []byte) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	hdr := &tar.Header{
		Name: "bzImage",
		Mode: 0600,
		Size: int64(len(bzimage)),
	}
	err := tw.WriteHeader(hdr)
	if err != nil {
		return buf, err
	}
	_, err = tw.Write(bzimage)
	if err != nil {
		return buf, err
	}
	hdr = &tar.Header{
		Name: "initrd.img",
		Mode: 0600,
		Size: int64(len(initrd)),
	}
	err = tw.WriteHeader(hdr)
	if err != nil {
		return buf, err
	}
	_, err = tw.Write(initrd)
	if err != nil {
		return buf, err
	}
	err = tw.Close()
	if err != nil {
		return buf, err
	}
	return buf, nil
}

func outputImg(image, filename string, bzimage []byte, initrd []byte, args ...string) error {
	log.Debugf("output img: %s %s", image, filename)
	log.Infof("  %s", filename)
	buf, err := tarInitrdKernel(bzimage, initrd)
	if err != nil {
		return err
	}
	img, err := dockerRunInput(buf, append([]string{image}, args...)...)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filename, img, os.FileMode(0644))
	if err != nil {
		return err
	}
	return nil
}

func outputISO(image, filename string, bzimage []byte, initrd []byte, args ...string) error {
	log.Debugf("output iso: %s %s", image, filename)
	log.Infof("  %s", filename)
	buf, err := tarInitrdKernel(bzimage, initrd)
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
	return nil
}

func outputKernelInitrd(base string, bzimage []byte, initrd []byte, cmdline string) error {
	log.Debugf("output kernel/initrd: %s %s", base, cmdline)
	log.Infof("  %s %s %s", base+"-bzImage", base+"-initrd.img", base+"-cmdline")
	err := ioutil.WriteFile(base+"-initrd.img", initrd, os.FileMode(0644))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(base+"-bzImage", bzimage, os.FileMode(0644))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(base+"-cmdline", []byte(cmdline), os.FileMode(0644))
	if err != nil {
		return err
	}
	return nil
}
