package filters

import "testing"

func TestOperatorParse(t *testing.T) {
	for _, filt := range []struct {
		argStr string
		arg    Arg
	}{
		{"created=today", Arg{"created", "=", "today"}},
		{"exited!=0", Arg{"exited", "!=", "0"}},
		{"created>1d", Arg{"created", ">", "1d"}},
		{"baz>=bif", Arg{"baz", ">=", "bif"}},
		{"foo<=harfblat", Arg{"foo", "<=", "harfblat"}},
		{"foo<:=harfblat", Arg{"foo", "<", ":"}}, // the =harfblat is discarded
	} {
		arg, err := ParseArg(filt.argStr)
		if err != nil {
			t.Errorf("failed to parse [%s]: %s", filt.argStr, err)
		}
		if arg != filt.arg {
			t.Errorf("expected %d arg, got %d", filt.arg, arg)
		}
		if arg.Key != filt.arg.Key {
			t.Errorf("expected %s arg, got %s", filt.arg.Key, arg.Key)
		}
		if arg.Operator != filt.arg.Operator {
			t.Errorf("expected %s arg, got %s", filt.arg.Operator, arg.Operator)
		}
		if arg.Value != filt.arg.Value {
			t.Errorf("expected %s arg, got %s", filt.arg.Value, arg.Value)
		}
	}
}
