package registrar

import (
	"reflect"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestReserve(c *check.C) {
	r := NewRegistrar()

	obj := "test1"
	if err := r.Reserve("test", obj); err != nil {
		c.Fatal(err)
	}

	if err := r.Reserve("test", obj); err != nil {
		c.Fatal(err)
	}

	obj2 := "test2"
	err := r.Reserve("test", obj2)
	if err == nil {
		c.Fatalf("expected error when reserving an already reserved name to another object")
	}
	if err != ErrNameReserved {
		c.Fatal("expected `ErrNameReserved` error when attempting to reserve an already reserved name")
	}
}

func (s *DockerSuite) TestRelease(c *check.C) {
	r := NewRegistrar()
	obj := "testing"

	if err := r.Reserve("test", obj); err != nil {
		c.Fatal(err)
	}
	r.Release("test")
	r.Release("test") // Ensure there is no panic here

	if err := r.Reserve("test", obj); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestGetNames(c *check.C) {
	r := NewRegistrar()
	obj := "testing"
	names := []string{"test1", "test2"}

	for _, name := range names {
		if err := r.Reserve(name, obj); err != nil {
			c.Fatal(err)
		}
	}
	r.Reserve("test3", "other")

	names2, err := r.GetNames(obj)
	if err != nil {
		c.Fatal(err)
	}

	if !reflect.DeepEqual(names, names2) {
		c.Fatalf("Exepected: %v, Got: %v", names, names2)
	}
}

func (s *DockerSuite) TestDelete(c *check.C) {
	r := NewRegistrar()
	obj := "testing"
	names := []string{"test1", "test2"}
	for _, name := range names {
		if err := r.Reserve(name, obj); err != nil {
			c.Fatal(err)
		}
	}

	r.Reserve("test3", "other")
	r.Delete(obj)

	_, err := r.GetNames(obj)
	if err == nil {
		c.Fatal("expected error getting names for deleted key")
	}

	if err != ErrNoSuchKey {
		c.Fatal("expected `ErrNoSuchKey`")
	}
}

func (s *DockerSuite) TestGet(c *check.C) {
	r := NewRegistrar()
	obj := "testing"
	name := "test"

	_, err := r.Get(name)
	if err == nil {
		c.Fatal("expected error when key does not exist")
	}
	if err != ErrNameNotReserved {
		c.Fatal(err)
	}

	if err := r.Reserve(name, obj); err != nil {
		c.Fatal(err)
	}

	if _, err = r.Get(name); err != nil {
		c.Fatal(err)
	}

	r.Delete(obj)
	_, err = r.Get(name)
	if err == nil {
		c.Fatal("expected error when key does not exist")
	}
	if err != ErrNameNotReserved {
		c.Fatal(err)
	}
}
