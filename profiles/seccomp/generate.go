// +build ignore

package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/profiles/seccomp"
)

// saves the specified seccomp profile as a json file with the specified name
func saveProfile(profile *seccomp.Seccomp, jsonFileName string) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	f := filepath.Join(wd, jsonFileName)

	// write the default profile to the file
	b, err := json.MarshalIndent(profile, "", "\t")
	if err != nil {
		panic(err)
	}

	if err := ioutil.WriteFile(f, b, 0644); err != nil {
		panic(err)
	}
}

// saves the default seccomp profiles as json files so people can use them as
// bases for their own custom profiles
func main() {

	saveProfile(seccomp.DefaultProfile(), "default.json")

	saveProfile(seccomp.DefaultProfileWithoutUserNamespaces(), "default-without-user-namespaces.json")

}
