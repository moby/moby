/*
	go test buildfile_parser_test.go \
		buildfile_parser.go buildfile_lexer.go \
		buildfile_parser_helper.go
*/

package docker

import (
	"strings"
	"testing"
)

type lextest struct {
	s      string
	tokens []string
}

type parsetest struct {
	s      string
	parsed string
}

var lextests = []lextest{
	{`run    mkdir -p /var/run/sshd`,
		[]string{"run", "mkdir", "-p", "/var/run/sshd"},
	},
	{`run    [ "$(cat /tmp/passwd)" = "root:testpass" ]`,
		[]string{"run", "[", `$(cat /tmp/passwd)`, "=", `root:testpass`, "]"},
	},
	{`from {IMAGE}`,
		[]string{"from", "{IMAGE}"},
	},
	{`run    sh -c 'echo root:testpass \
    > /tmp/passwd'`,
		[]string{"run", "sh", "-c", "echo root:testpass > /tmp/passwd"},
	},
	{`cmd ["/bin/echo", "Hello World"]`,
		[]string{"cmd", "[", `/bin/echo`, ",", `Hello World`, "]"},
	},
}

var parsetests = []parsetest{
	{`run    mkdir -p /var/run/sshd`, `RUN /bin/sh -c mkdir -p /var/run/sshd`},
	{`run    [ "$(cat /tmp/passwd)" = "root:testpass" ]`, `RUN /bin/sh -c [ $(cat /tmp/passwd) = root:testpass ]`},
	{`from   {IMAGE}`, `FROM {IMAGE}`},
	{`from   {IMAGE}:{TAG}`, `FROM {IMAGE}:{TAG}`},
	{`run    sh -c 'echo root:testpass > /tmp/passwd'`, `RUN sh -c echo root:testpass > /tmp/passwd`},
	{`run    [ "$(ls -d /var/run/sshd)" = "/var/run/sshd" ]`, `RUN /bin/sh -c [ $(ls -d /var/run/sshd) = /var/run/sshd ]`},
	{`run    sh -c 'echo root:testpass \
        > /tmp/passwd'`, `RUN sh -c echo root:testpass > /tmp/passwd`},
	{`run    echo "foo \n bar"; echo "baz"`, `RUN /bin/sh -c echo foo \n bar ; echo baz`},
	{`add foo /usr/lib/bla/bar`, `ADD foo /usr/lib/bla/bar`},
	{`run [ "$(cat /usr/lib/bla/bar)" = 'hello' ]`, `RUN /bin/sh -c [ $(cat /usr/lib/bla/bar) = hello ]`},
	{`add http://{SERVERADDR}/baz /usr/lib/baz/quux`, `ADD http://{SERVERADDR}/baz /usr/lib/baz/quux`},
	{`add f /`, `ADD f /`},
	{`add d /somewheeeere/over/the/rainbooow`, `ADD d /somewheeeere/over/the/rainbooow`},
	{`env    FOO BAR`, `ENV FOO BAR`},
	{`run    [ "$FOO" = "BAR" ]`, `RUN /bin/sh -c [ $FOO = BAR ]`},
	{`ENTRYPOINT /bin/echo`, `ENTRYPOINT /bin/sh -c /bin/echo`},
	{`VOLUME /test`, `VOLUME /test`},
	{`env    FOO /foo/baz`, `ENV FOO /foo/baz`},
	{`env    BAR /bar`, `ENV BAR /bar`},
	{`env    BAZ $BAR`, `ENV BAZ $BAR`},
	{`env    FOOPATH $PATH:$FOO`, `ENV FOOPATH $PATH:$FOO`},
	{`add    testfile $BAZ/`, `ADD testfile $BAZ/`},
	{`add    $TEST $FOO`, `ADD $TEST $FOO`},
	{`USER joelr`, `USER joelr`},
	{`MAINTAINER Solomon Hykes <solomon@dotcloud.com>`, `MAINTAINER Solomon Hykes <solomon@dotcloud.com>`},
	{`WORKDIR /foo/bar/baz`, `WORKDIR /foo/bar/baz`},
	{`EXPOSE 1234`, `EXPOSE 1234`},
	{`EXPOSE 1234 5679 3456`, `EXPOSE 1234 5679 3456`},
	{`from 83599e29c455eb719f77d799bc7c51521b9551972f5a850d7ad265bc1b5292f6`, `FROM 83599e29c455eb719f77d799bc7c51521b9551972f5a850d7ad265bc1b5292f6`},
	{`env port 4243`, `ENV port 4243`},
	{`cmd ["/bin/echo", "Hello World"]`, `CMD /bin/echo Hello World`},
	{`cmd Hello world`, `CMD /bin/sh -c Hello world`},
	{`include foo/bar/baz`, `INCLUDE foo/bar/baz`},
}

func TestLexer(t *testing.T) {
	for _, test := range lextests {
		seq(t, testlex(test.s), test.tokens)
	}
}

func TestParser(t *testing.T) {
	for _, test := range parsetests {
		bf, err := testparse(test.s)
		if err != nil {
			t.Fatalf("Error parsing %s: %s", test.s, err)
		}
		if s := bf.String(); s != test.parsed {
			t.Fatalf("Expected '%s' but got '%s'", test.parsed, s)
		}
	}
}

func testlex(s string) []string {
	a := make([]string, 0)
	r := strings.NewReader(s)
	lexer := NewLexer(r)
	lval := &yySymType{}
	for {
		tok := lexer.Lex(lval)
		if tok == 0 {
			break
		}
		a = append(a, token(tok, lval))
	}
	return a
}

func testparse(s string) (*dockerFile, error) {
	return parse(strings.NewReader(s))
}

func seq(t *testing.T, a1, a2 []string) {
	if len(a1) != len(a2) {
		t.Fatalf("%v(%d) != %v(%d)", a1, len(a1), a2, len(a2))
	}
	for i := 0; i < len(a1); i++ {
		if a1[i] != a2[i] {
			t.Fatalf("%v != %v, %v @ %d != %v", a1, a2, a1[i], i, a2[i])
		}
	}
}
