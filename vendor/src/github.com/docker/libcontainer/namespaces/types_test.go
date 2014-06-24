package namespaces

import (
	"testing"
)

func TestNamespacesContains(t *testing.T) {
	ns := Namespaces{
		GetNamespace("NEWPID"),
		GetNamespace("NEWNS"),
		GetNamespace("NEWUTS"),
	}

	if ns.Contains("NEWNET") {
		t.Fatal("namespaces should not contain NEWNET")
	}

	if !ns.Contains("NEWPID") {
		t.Fatal("namespaces should contain NEWPID but does not")
	}

	withNil := Namespaces{
		GetNamespace("UNDEFINED"), // this element will be nil
		GetNamespace("NEWPID"),
	}

	if !withNil.Contains("NEWPID") {
		t.Fatal("namespaces should contain NEWPID but does not")
	}
}
