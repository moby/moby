package caller

import (
	"testing"

	_ "github.com/docker/libnetwork/testutils"
)

func fun1() string {
	return Name(0)
}

func fun2() string {
	return Name(1)
}

func fun3() string {
	return fun4()
}

func fun4() string {
	return Name(0)
}

func fun5() string {
	return fun6()
}

func fun6() string {
	return Name(1)
}

func TestCaller(t *testing.T) {
	funName := fun1()
	if funName != "fun1" {
		t.Fatalf("error on fun1 caller %s", funName)
	}

	funName = fun2()
	if funName != "TestCaller" {
		t.Fatalf("error on fun2 caller %s", funName)
	}

	funName = fun3()
	if funName != "fun4" {
		t.Fatalf("error on fun2 caller %s", funName)
	}

	funName = fun5()
	if funName != "fun5" {
		t.Fatalf("error on fun5 caller %s", funName)
	}
}
