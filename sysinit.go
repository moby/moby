package docker

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

// Setup networking
func setupNetworking(gw string) {
	if gw == "" {
		return
	}
	cmd := exec.Command("/sbin/route", "add", "default", "gw", gw)
	if err := cmd.Run(); err != nil {
		log.Fatalf("Unable to set up networking: %v", err)
	}
}

// Takes care of dropping privileges to the desired user
func changeUser(u string) {
	if u == "" {
		return
	}
	userent, err := user.LookupId(u)
	if err != nil {
		userent, err = user.Lookup(u)
	}
	if err != nil {
		log.Fatalf("Unable to find user %v: %v", u, err)
	}

	uid, err := strconv.Atoi(userent.Uid)
	if err != nil {
		log.Fatalf("Invalid uid: %v", userent.Uid)
	}
	gid, err := strconv.Atoi(userent.Gid)
	if err != nil {
		log.Fatalf("Invalid gid: %v", userent.Gid)
	}

	if err := syscall.Setgid(gid); err != nil {
		log.Fatalf("setgid failed: %v", err)
	}
	if err := syscall.Setuid(uid); err != nil {
		log.Fatalf("setuid failed: %v", err)
	}
}

// Set the environment to a known, repeatable state
func setupEnv() {
	os.Clearenv()
	os.Setenv("HOME", "/")
	os.Setenv("PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
}

func executeProgram(name string, args []string) {
	path, err := exec.LookPath(name)
	if err != nil {
		log.Printf("Unable to locate %v", name)
		os.Exit(127)
	}

	if err := syscall.Exec(path, args, os.Environ()); err != nil {
		panic(err)
	}
}

// Sys Init code
// This code is run INSIDE the container and is responsible for setting
// up the environment before running the actual process
func SysInit() {
	if len(os.Args) <= 1 {
		fmt.Println("You should not invoke docker-init manually")
		os.Exit(1)
	}
	var u = flag.String("u", "", "username or uid")
	var gw = flag.String("g", "", "gateway address")

	flag.Parse()

	setupNetworking(*gw)
	changeUser(*u)
	setupEnv()
	executeProgram(flag.Arg(0), flag.Args())
}
