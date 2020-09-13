// +build ignore

package main

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/profiles/apparmor"
)

// Saves the default AppArmor profile as a Go template so people can use it as a
// base for their own custom profiles.
func main() {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	f := filepath.Join(wd, "default-profile")

	// write the default profile to the file
	tmpl := apparmor.BaseTemplate()
	if err := ioutil.WriteFile(f, []byte(tmpl), 0644); err != nil {
		panic(err)
	}
}
