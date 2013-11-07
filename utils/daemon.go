package utils

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
)

func CreatePidFile(pidfile string) error {
	if pidString, err := ioutil.ReadFile(pidfile); err == nil {
		pid, err := strconv.Atoi(string(pidString))
		if err == nil {
			if _, err := os.Stat(fmt.Sprintf("/proc/%d/", pid)); err == nil {
				return fmt.Errorf("pid file found, ensure docker is not running or delete %s", pidfile)
			}
		}
	}

	file, err := os.Create(pidfile)
	if err != nil {
		return err
	}

	defer file.Close()

	_, err = fmt.Fprintf(file, "%d", os.Getpid())
	return err
}

func RemovePidFile(pidfile string) {
	if err := os.Remove(pidfile); err != nil {
		log.Printf("Error removing %s: %s", pidfile, err)
	}
}
