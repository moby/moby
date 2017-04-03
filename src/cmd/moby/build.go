package main

import (
	"archive/tar"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/moby/src/initrd"
)

// Process the build arguments and execute build
func build(args []string) {
	buildCmd := flag.NewFlagSet("build", flag.ExitOnError)
	buildCmd.Usage = func() {
		fmt.Printf("USAGE: %s build [options] <file>[.yml]\n\n", os.Args[0])
		fmt.Printf("Options:\n")
		buildCmd.PrintDefaults()
	}
	buildName := buildCmd.String("name", "", "Name to use for output files")
	buildPull := buildCmd.Bool("pull", false, "Always pull images")

	buildCmd.Parse(args)
	remArgs := buildCmd.Args()

	if len(remArgs) == 0 {
		fmt.Println("Please specify a configuration file")
		buildCmd.Usage()
		os.Exit(1)
	}
	conf := remArgs[0]
	if filepath.Ext(conf) == "" {
		conf = conf + ".yml"
	}

	buildInternal(*buildName, *buildPull, conf)
}

// Perform the actual build process
func buildInternal(name string, pull bool, conf string) {
	if name == "" {
		name = filepath.Base(conf)
		ext := filepath.Ext(conf)
		if ext != "" {
			name = name[:len(name)-len(ext)]
		}
	}

	config, err := ioutil.ReadFile(conf)
	if err != nil {
		log.Fatalf("Cannot open config file: %v", err)
	}

	m, err := NewConfig(config)
	if err != nil {
		log.Fatalf("Invalid config: %v", err)
	}

	containers := []*bytes.Buffer{}

	if pull {
		log.Infof("Pull kernel image: %s", m.Kernel.Image)
		err := dockerPull(m.Kernel.Image)
		if err != nil {
			log.Fatalf("Could not pull image %s: %v", m.Kernel.Image, err)
		}
	}
	// get kernel bzImage and initrd tarball from container
	// TODO examine contents to see what names they might have
	log.Infof("Extract kernel image: %s", m.Kernel.Image)
	const (
		bzimageName = "bzImage"
		ktarName    = "kernel.tar"
	)
	out, err := dockerRun(m.Kernel.Image, "tar", "cf", "-", bzimageName, ktarName)
	if err != nil {
		log.Fatalf("Failed to extract kernel image and tarball: %v", err)
	}
	buf := bytes.NewBuffer(out)
	bzimage, ktar, err := untarKernel(buf, bzimageName, ktarName)
	if err != nil {
		log.Fatalf("Could not extract bzImage and kernel filesystem from tarball. %v", err)
	}
	containers = append(containers, ktar)

	// convert init image to tarball
	if pull {
		log.Infof("Pull init: %s", m.Init)
		err := dockerPull(m.Init)
		if err != nil {
			log.Fatalf("Could not pull image %s: %v", m.Init, err)
		}
	}
	log.Infof("Process init: %s", m.Init)
	init, err := ImageExtract(m.Init, "")
	if err != nil {
		log.Fatalf("Failed to build init tarball: %v", err)
	}
	buffer := bytes.NewBuffer(init)
	containers = append(containers, buffer)

	log.Infof("Add system containers:")
	for i, image := range m.System {
		if pull {
			log.Infof("  Pull: %s", image.Image)
			err := dockerPull(image.Image)
			if err != nil {
				log.Fatalf("Could not pull image %s: %v", image.Image, err)
			}
		}
		log.Infof("  Create OCI config for %s", image.Image)
		config, err := ConfigToOCI(&image)
		if err != nil {
			log.Fatalf("Failed to create config.json for %s: %v", image.Image, err)
		}
		so := fmt.Sprintf("%03d", i)
		path := "containers/system/" + so + "-" + image.Name
		out, err := ImageBundle(path, image.Image, config)
		if err != nil {
			log.Fatalf("Failed to extract root filesystem for %s: %v", image.Image, err)
		}
		buffer := bytes.NewBuffer(out)
		containers = append(containers, buffer)
	}

	log.Infof("Add daemon containers:")
	for _, image := range m.Daemon {
		if pull {
			log.Infof("  Pull: %s", image.Image)
			err := dockerPull(image.Image)
			if err != nil {
				log.Fatalf("Could not pull image %s: %v", image.Image, err)
			}
		}
		log.Infof("  Create OCI config for %s", image.Image)
		config, err := ConfigToOCI(&image)
		if err != nil {
			log.Fatalf("Failed to create config.json for %s: %v", image.Image, err)
		}
		path := "containers/daemon/" + image.Name
		out, err := ImageBundle(path, image.Image, config)
		if err != nil {
			log.Fatalf("Failed to extract root filesystem for %s: %v", image.Image, err)
		}
		buffer := bytes.NewBuffer(out)
		containers = append(containers, buffer)
	}

	// add files
	buffer, err = filesystem(m)
	if err != nil {
		log.Fatalf("failed to add filesystem parts: %v", err)
	}
	containers = append(containers, buffer)

	log.Infof("Create initial ram disk")
	initrd, err := containersInitrd(containers)
	if err != nil {
		log.Fatalf("Failed to make initrd %v", err)
	}

	log.Infof("Create outputs:")
	err = outputs(m, name, bzimage.Bytes(), initrd.Bytes())
	if err != nil {
		log.Fatalf("Error writing outputs: %v", err)
	}
}

func untarKernel(buf *bytes.Buffer, bzimageName, ktarName string) (*bytes.Buffer, *bytes.Buffer, error) {
	tr := tar.NewReader(buf)

	var bzimage, ktar *bytes.Buffer

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalln(err)
		}
		switch hdr.Name {
		case bzimageName:
			bzimage = new(bytes.Buffer)
			_, err := io.Copy(bzimage, tr)
			if err != nil {
				return nil, nil, err
			}
		case ktarName:
			ktar = new(bytes.Buffer)
			_, err := io.Copy(bzimage, tr)
			if err != nil {
				return nil, nil, err
			}
		default:
			continue
		}
	}

	if ktar == nil || bzimage == nil {
		return nil, nil, errors.New("did not find bzImage and kernel.tar in tarball")
	}

	return bzimage, ktar, nil
}

func containersInitrd(containers []*bytes.Buffer) (*bytes.Buffer, error) {
	w := new(bytes.Buffer)
	iw := initrd.NewWriter(w)
	defer iw.Close()
	for _, file := range containers {
		_, err := initrd.Copy(iw, file)
		if err != nil {
			return nil, err
		}
	}

	return w, nil
}
