package docker

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
)

// Sys Init code
// This code is run INSIDE the container and is responsible for setting
// up the environment before running the actual process
func SysInit() {
	if len(os.Args) <= 1 {
		fmt.Println("You should not invoke docker-init manually")
		os.Exit(1)
	}

	path, err := exec.LookPath(os.Args[1])
	if err != nil {
		log.Printf("Unable to locate %v", os.Args[1])
		os.Exit(127)
	}

	if err := syscall.Exec(path, os.Args[1:], os.Environ()); err != nil {
		panic(err)
	}
}
