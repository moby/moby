package filters

import "testing"

func TestParseArgs(t *testing.T) {
	// equivalent of `docker ps -f 'created=today' -f 'image.name=ubuntu*' -f 'image.name=*untu'`
	flagArgs := []string{
		"created=today",
		"image.name=ubuntu*",
		"image.name=*untu",
	}
	var (
		args = Args{}
		err  error
	)
	for i := range flagArgs {
		args, err = ParseFlag(flagArgs[i], args)
		if err != nil {
			t.Errorf("failed to parse %s: %s", flagArgs[i], err)
		}
	}
	if len(args.GetAll("created")) != 1 {
		t.Errorf("failed to set this arg")
	}
	if len(args.GetAll("image.name")) != 2 {
		t.Errorf("the args should have collapsed")
	}
}

func TestParam(t *testing.T) {
	a := Args{
		Arg{Key: "created", Operator: "=", Value: "today"},
		Arg{Key: "image.name", Operator: "=", Value: "*untu"},
		Arg{Key: "image.name", Operator: "=", Value: "ubuntu*"},
	}

	v, err := ToParam(a)
	if err != nil {
		t.Errorf("failed to marshal the filters: %s", err)
	}
	v1, err := FromParam(v)
	if err != nil {
		t.Errorf("%s", err)
	}
	// likely a fragle test
	if v1[0] != a[0] {
		t.Fatalf("%#v -- %#v", a, v1)
	}
}

func TestEmpty(t *testing.T) {
	a := Args{}
	v, err := ToParam(a)
	if err != nil {
		t.Errorf("failed to marshal the filters: %s", err)
	}
	v1, err := FromParam(v)
	if err != nil {
		t.Errorf("%s", err)
	}
	if len(a) != len(v1) {
		t.Errorf("these should both be empty sets")
	}
}
