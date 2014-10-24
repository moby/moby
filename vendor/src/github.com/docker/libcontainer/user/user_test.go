package user

import (
	"strings"
	"testing"
)

func TestUserParseLine(t *testing.T) {
	var (
		a, b string
		c    []string
		d    int
	)

	parseLine("", &a, &b)
	if a != "" || b != "" {
		t.Fatalf("a and b should be empty ('%v', '%v')", a, b)
	}

	parseLine("a", &a, &b)
	if a != "a" || b != "" {
		t.Fatalf("a should be 'a' and b should be empty ('%v', '%v')", a, b)
	}

	parseLine("bad boys:corny cows", &a, &b)
	if a != "bad boys" || b != "corny cows" {
		t.Fatalf("a should be 'bad boys' and b should be 'corny cows' ('%v', '%v')", a, b)
	}

	parseLine("", &c)
	if len(c) != 0 {
		t.Fatalf("c should be empty (%#v)", c)
	}

	parseLine("d,e,f:g:h:i,j,k", &c, &a, &b, &c)
	if a != "g" || b != "h" || len(c) != 3 || c[0] != "i" || c[1] != "j" || c[2] != "k" {
		t.Fatalf("a should be 'g', b should be 'h', and c should be ['i','j','k'] ('%v', '%v', '%#v')", a, b, c)
	}

	parseLine("::::::::::", &a, &b, &c)
	if a != "" || b != "" || len(c) != 0 {
		t.Fatalf("a, b, and c should all be empty ('%v', '%v', '%#v')", a, b, c)
	}

	parseLine("not a number", &d)
	if d != 0 {
		t.Fatalf("d should be 0 (%v)", d)
	}

	parseLine("b:12:c", &a, &d, &b)
	if a != "b" || b != "c" || d != 12 {
		t.Fatalf("a should be 'b' and b should be 'c', and d should be 12 ('%v', '%v', %v)", a, b, d)
	}
}

func TestUserParsePasswd(t *testing.T) {
	users, err := parsePasswdFile(strings.NewReader(`
root:x:0:0:root:/root:/bin/bash
adm:x:3:4:adm:/var/adm:/bin/false
this is just some garbage data
`), nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(users) != 3 {
		t.Fatalf("Expected 3 users, got %v", len(users))
	}
	if users[0].Uid != 0 || users[0].Name != "root" {
		t.Fatalf("Expected users[0] to be 0 - root, got %v - %v", users[0].Uid, users[0].Name)
	}
	if users[1].Uid != 3 || users[1].Name != "adm" {
		t.Fatalf("Expected users[1] to be 3 - adm, got %v - %v", users[1].Uid, users[1].Name)
	}
}

func TestUserParseGroup(t *testing.T) {
	groups, err := parseGroupFile(strings.NewReader(`
root:x:0:root
adm:x:4:root,adm,daemon
this is just some garbage data
`), nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(groups) != 3 {
		t.Fatalf("Expected 3 groups, got %v", len(groups))
	}
	if groups[0].Gid != 0 || groups[0].Name != "root" || len(groups[0].List) != 1 {
		t.Fatalf("Expected groups[0] to be 0 - root - 1 member, got %v - %v - %v", groups[0].Gid, groups[0].Name, len(groups[0].List))
	}
	if groups[1].Gid != 4 || groups[1].Name != "adm" || len(groups[1].List) != 3 {
		t.Fatalf("Expected groups[1] to be 4 - adm - 3 members, got %v - %v - %v", groups[1].Gid, groups[1].Name, len(groups[1].List))
	}
}
