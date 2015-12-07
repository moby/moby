// +build linux

package overlay2

import "io/ioutil"

// XXX: copied from aufs

// Return all the directories
func loadIds(root string) ([]string, error) {
	dirs, err := ioutil.ReadDir(root)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, d := range dirs {
		if !d.IsDir() {
			out = append(out, d.Name())
		}
	}
	return out, nil
}
