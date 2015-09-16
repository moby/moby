package cli

import (
	"flag"
	"testing"
	"time"
)

func TestNewContext(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Int("myflag", 12, "doc")
	globalSet := flag.NewFlagSet("test", 0)
	globalSet.Int("myflag", 42, "doc")
	globalCtx := NewContext(nil, globalSet, nil)
	command := Command{Name: "mycommand"}
	c := NewContext(nil, set, globalCtx)
	c.Command = command
	expect(t, c.Int("myflag"), 12)
	expect(t, c.GlobalInt("myflag"), 42)
	expect(t, c.Command.Name, "mycommand")
}

func TestContext_Int(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Int("myflag", 12, "doc")
	c := NewContext(nil, set, nil)
	expect(t, c.Int("myflag"), 12)
}

func TestContext_Duration(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Duration("myflag", time.Duration(12*time.Second), "doc")
	c := NewContext(nil, set, nil)
	expect(t, c.Duration("myflag"), time.Duration(12*time.Second))
}

func TestContext_String(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.String("myflag", "hello world", "doc")
	c := NewContext(nil, set, nil)
	expect(t, c.String("myflag"), "hello world")
}

func TestContext_Bool(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Bool("myflag", false, "doc")
	c := NewContext(nil, set, nil)
	expect(t, c.Bool("myflag"), false)
}

func TestContext_BoolT(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Bool("myflag", true, "doc")
	c := NewContext(nil, set, nil)
	expect(t, c.BoolT("myflag"), true)
}

func TestContext_Args(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Bool("myflag", false, "doc")
	c := NewContext(nil, set, nil)
	set.Parse([]string{"--myflag", "bat", "baz"})
	expect(t, len(c.Args()), 2)
	expect(t, c.Bool("myflag"), true)
}

func TestContext_IsSet(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Bool("myflag", false, "doc")
	set.String("otherflag", "hello world", "doc")
	globalSet := flag.NewFlagSet("test", 0)
	globalSet.Bool("myflagGlobal", true, "doc")
	globalCtx := NewContext(nil, globalSet, nil)
	c := NewContext(nil, set, globalCtx)
	set.Parse([]string{"--myflag", "bat", "baz"})
	globalSet.Parse([]string{"--myflagGlobal", "bat", "baz"})
	expect(t, c.IsSet("myflag"), true)
	expect(t, c.IsSet("otherflag"), false)
	expect(t, c.IsSet("bogusflag"), false)
	expect(t, c.IsSet("myflagGlobal"), false)
}

func TestContext_GlobalIsSet(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Bool("myflag", false, "doc")
	set.String("otherflag", "hello world", "doc")
	globalSet := flag.NewFlagSet("test", 0)
	globalSet.Bool("myflagGlobal", true, "doc")
	globalSet.Bool("myflagGlobalUnset", true, "doc")
	globalCtx := NewContext(nil, globalSet, nil)
	c := NewContext(nil, set, globalCtx)
	set.Parse([]string{"--myflag", "bat", "baz"})
	globalSet.Parse([]string{"--myflagGlobal", "bat", "baz"})
	expect(t, c.GlobalIsSet("myflag"), false)
	expect(t, c.GlobalIsSet("otherflag"), false)
	expect(t, c.GlobalIsSet("bogusflag"), false)
	expect(t, c.GlobalIsSet("myflagGlobal"), true)
	expect(t, c.GlobalIsSet("myflagGlobalUnset"), false)
	expect(t, c.GlobalIsSet("bogusGlobal"), false)
}

func TestContext_NumFlags(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Bool("myflag", false, "doc")
	set.String("otherflag", "hello world", "doc")
	globalSet := flag.NewFlagSet("test", 0)
	globalSet.Bool("myflagGlobal", true, "doc")
	globalCtx := NewContext(nil, globalSet, nil)
	c := NewContext(nil, set, globalCtx)
	set.Parse([]string{"--myflag", "--otherflag=foo"})
	globalSet.Parse([]string{"--myflagGlobal"})
	expect(t, c.NumFlags(), 2)
}
