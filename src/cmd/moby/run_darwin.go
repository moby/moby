// +build darwin

package main

import (
	"io/ioutil"
	"os"
	"os/user"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/hyperkit/go"
)

func run(cpus, mem, diskSz int, disk string, args []string) {
	prefix := "moby"
	if len(args) > 0 {
		prefix = args[0]
	}

	cmdline, err := ioutil.ReadFile(prefix + "-cmdline")
	if err != nil {
		log.Fatalf("Cannot open cmdline file: %v", err)
	}

	if diskSz != 0 && disk == "" {
		disk = prefix + "-disk.img"
	}

	h, err := hyperkit.New("", "", "auto", disk)
	if err != nil {
		log.Fatalln("Error creating hyperkit: ", err)
	}

	h.Kernel = prefix + "-bzImage"
	h.Initrd = prefix + "-initrd.img"
	h.CPUs = cpus
	h.Memory = mem
	h.DiskSize = diskSz

	err = h.Run(string(cmdline))
	if err != nil {
		log.Fatalf("Cannot run hyperkit: %v", err)
	}
}

func getHome() string {
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return os.Getenv("HOME")
}
