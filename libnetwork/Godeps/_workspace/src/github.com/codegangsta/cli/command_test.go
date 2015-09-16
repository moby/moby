package cli

import (
	"flag"
	"testing"
)

func TestCommandDoNotIgnoreFlags(t *testing.T) {
	app := NewApp()
	set := flag.NewFlagSet("test", 0)
	test := []string{"blah", "blah", "-break"}
	set.Parse(test)

	c := NewContext(app, set, nil)

	command := Command{
		Name:        "test-cmd",
		Aliases:     []string{"tc"},
		Usage:       "this is for testing",
		Description: "testing",
		Action:      func(_ *Context) {},
	}
	err := command.Run(c)

	expect(t, err.Error(), "flag provided but not defined: -break")
}

func TestCommandIgnoreFlags(t *testing.T) {
	app := NewApp()
	set := flag.NewFlagSet("test", 0)
	test := []string{"blah", "blah"}
	set.Parse(test)

	c := NewContext(app, set, nil)

	command := Command{
		Name:            "test-cmd",
		Aliases:         []string{"tc"},
		Usage:           "this is for testing",
		Description:     "testing",
		Action:          func(_ *Context) {},
		SkipFlagParsing: true,
	}
	err := command.Run(c)

	expect(t, err, nil)
}
