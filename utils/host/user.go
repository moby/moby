package host

import (
	"fmt"
	"io/ioutil"
	"strings"
)

// UserLookup check if the given username or uid is present in /etc/passwd
// and returns the user struct.
// If the username is not found, an error is returned.
func UserLookup(uid string) (*User, error) {
	file, err := ioutil.ReadFile("/etc/passwd")
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(file), "\n") {
		data := strings.Split(line, ":")
		if len(data) > 5 && (data[0] == uid || data[2] == uid) {
			return &User{
				Uid:      data[2],
				Gid:      data[3],
				Username: data[0],
				Name:     data[4],
				HomeDir:  data[5],
			}, nil
		}
	}
	return nil, fmt.Errorf("User not found in /etc/passwd")
}

type User struct {
	Uid      string // user id
	Gid      string // primary group id
	Username string
	Name     string
	HomeDir  string
}
