package user

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const (
	minId = 0
	maxId = 1<<31 - 1 //for 32-bit systems compatibility
)

var (
	ErrRange = fmt.Errorf("Uids and gids must be in range %d-%d", minId, maxId)
)

type User struct {
	Name  string
	Pass  string
	Uid   int
	Gid   int
	Gecos string
	Home  string
	Shell string
}

type Group struct {
	Name string
	Pass string
	Gid  int
	List []string
}

func parseLine(line string, v ...interface{}) {
	if line == "" {
		return
	}

	parts := strings.Split(line, ":")
	for i, p := range parts {
		if len(v) <= i {
			// if we have more "parts" than we have places to put them, bail for great "tolerance" of naughty configuration files
			break
		}

		switch e := v[i].(type) {
		case *string:
			// "root", "adm", "/bin/bash"
			*e = p
		case *int:
			// "0", "4", "1000"
			// ignore string to int conversion errors, for great "tolerance" of naughty configuration files
			*e, _ = strconv.Atoi(p)
		case *[]string:
			// "", "root", "root,adm,daemon"
			if p != "" {
				*e = strings.Split(p, ",")
			} else {
				*e = []string{}
			}
		default:
			// panic, because this is a programming/logic error, not a runtime one
			panic("parseLine expects only pointers!  argument " + strconv.Itoa(i) + " is not a pointer!")
		}
	}
}

func ParsePasswd() ([]*User, error) {
	return ParsePasswdFilter(nil)
}

func ParsePasswdFilter(filter func(*User) bool) ([]*User, error) {
	f, err := os.Open("/etc/passwd")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parsePasswdFile(f, filter)
}

func parsePasswdFile(r io.Reader, filter func(*User) bool) ([]*User, error) {
	var (
		s   = bufio.NewScanner(r)
		out = []*User{}
	)

	for s.Scan() {
		if err := s.Err(); err != nil {
			return nil, err
		}

		text := strings.TrimSpace(s.Text())
		if text == "" {
			continue
		}

		// see: man 5 passwd
		//  name:password:UID:GID:GECOS:directory:shell
		// Name:Pass:Uid:Gid:Gecos:Home:Shell
		//  root:x:0:0:root:/root:/bin/bash
		//  adm:x:3:4:adm:/var/adm:/bin/false
		p := &User{}
		parseLine(
			text,
			&p.Name, &p.Pass, &p.Uid, &p.Gid, &p.Gecos, &p.Home, &p.Shell,
		)

		if filter == nil || filter(p) {
			out = append(out, p)
		}
	}

	return out, nil
}

func ParseGroup() ([]*Group, error) {
	return ParseGroupFilter(nil)
}

func ParseGroupFilter(filter func(*Group) bool) ([]*Group, error) {
	f, err := os.Open("/etc/group")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseGroupFile(f, filter)
}

func parseGroupFile(r io.Reader, filter func(*Group) bool) ([]*Group, error) {
	var (
		s   = bufio.NewScanner(r)
		out = []*Group{}
	)

	for s.Scan() {
		if err := s.Err(); err != nil {
			return nil, err
		}

		text := s.Text()
		if text == "" {
			continue
		}

		// see: man 5 group
		//  group_name:password:GID:user_list
		// Name:Pass:Gid:List
		//  root:x:0:root
		//  adm:x:4:root,adm,daemon
		p := &Group{}
		parseLine(
			text,
			&p.Name, &p.Pass, &p.Gid, &p.List,
		)

		if filter == nil || filter(p) {
			out = append(out, p)
		}
	}

	return out, nil
}

// Given a string like "user", "1000", "user:group", "1000:1000", returns the uid, gid, list of supplementary group IDs, and home directory, if available and/or applicable.
func GetUserGroupSupplementaryHome(userSpec string, defaultUid, defaultGid int, defaultHome string) (int, int, []int, string, error) {
	var (
		uid      = defaultUid
		gid      = defaultGid
		suppGids = []int{}
		home     = defaultHome

		userArg, groupArg string
	)

	// allow for userArg to have either "user" syntax, or optionally "user:group" syntax
	parseLine(userSpec, &userArg, &groupArg)

	users, err := ParsePasswdFilter(func(u *User) bool {
		if userArg == "" {
			return u.Uid == uid
		}
		return u.Name == userArg || strconv.Itoa(u.Uid) == userArg
	})
	if err != nil && !os.IsNotExist(err) {
		if userArg == "" {
			userArg = strconv.Itoa(uid)
		}
		return 0, 0, nil, "", fmt.Errorf("Unable to find user %v: %v", userArg, err)
	}

	haveUser := users != nil && len(users) > 0
	if haveUser {
		// if we found any user entries that matched our filter, let's take the first one as "correct"
		uid = users[0].Uid
		gid = users[0].Gid
		home = users[0].Home
	} else if userArg != "" {
		// we asked for a user but didn't find them...  let's check to see if we wanted a numeric user
		uid, err = strconv.Atoi(userArg)
		if err != nil {
			// not numeric - we have to bail
			return 0, 0, nil, "", fmt.Errorf("Unable to find user %v", userArg)
		}
		if uid < minId || uid > maxId {
			return 0, 0, nil, "", ErrRange
		}

		// if userArg couldn't be found in /etc/passwd but is numeric, just roll with it - this is legit
	}

	if groupArg != "" || (haveUser && users[0].Name != "") {
		groups, err := ParseGroupFilter(func(g *Group) bool {
			if groupArg != "" {
				return g.Name == groupArg || strconv.Itoa(g.Gid) == groupArg
			}
			for _, u := range g.List {
				if u == users[0].Name {
					return true
				}
			}
			return false
		})
		if err != nil && !os.IsNotExist(err) {
			return 0, 0, nil, "", fmt.Errorf("Unable to find groups for user %v: %v", users[0].Name, err)
		}

		haveGroup := groups != nil && len(groups) > 0
		if groupArg != "" {
			if haveGroup {
				// if we found any group entries that matched our filter, let's take the first one as "correct"
				gid = groups[0].Gid
			} else {
				// we asked for a group but didn't find id...  let's check to see if we wanted a numeric group
				gid, err = strconv.Atoi(groupArg)
				if err != nil {
					// not numeric - we have to bail
					return 0, 0, nil, "", fmt.Errorf("Unable to find group %v", groupArg)
				}
				if gid < minId || gid > maxId {
					return 0, 0, nil, "", ErrRange
				}

				// if groupArg couldn't be found in /etc/group but is numeric, just roll with it - this is legit
			}
		} else if haveGroup {
			suppGids = make([]int, len(groups))
			for i, group := range groups {
				suppGids[i] = group.Gid
			}
		}
	}

	return uid, gid, suppGids, home, nil
}
