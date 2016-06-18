package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	flag "github.com/docker/docker/pkg/mflag"
)

// Cli represents a command line interface.
type Cli struct {
	Stderr   io.Writer
	handlers []Handler
	Usage    func()
}

// Handler holds the different commands Cli will call
// It should have methods with names starting with `Cmd` like:
// 	func (h myHandler) CmdFoo(args ...string) error
type Handler interface {
	Command(name string) func(...string) error
}

// Initializer can be optionally implemented by a Handler to
// initialize before each call to one of its commands.
type Initializer interface {
	Initialize() error
}

// New instantiates a ready-to-use Cli.
func New(handlers ...Handler) *Cli {
	// make the generic Cli object the first cli handler
	// in order to handle `docker help` appropriately
	cli := new(Cli)
	cli.handlers = append([]Handler{cli}, handlers...)
	return cli
}

var errCommandNotFound = errors.New("command not found")

func (cli *Cli) command(args ...string) (func(...string) error, error) {
	for _, c := range cli.handlers {
		if c == nil {
			continue
		}
		if cmd := c.Command(strings.Join(args, " ")); cmd != nil {
			if ci, ok := c.(Initializer); ok {
				if err := ci.Initialize(); err != nil {
					return nil, err
				}
			}
			return cmd, nil
		}
	}
	return nil, errCommandNotFound
}

// Run executes the specified command.
func (cli *Cli) Run(args ...string) error {
	if len(args) > 1 {
		command, err := cli.command(args[:2]...)
		if err == nil {
			return command(args[2:]...)
		}
		if err != errCommandNotFound {
			return err
		}
	}
	if len(args) > 0 {
		command, err := cli.command(args[0])
		if err != nil {
			if err == errCommandNotFound {
				cli.noSuchCommand(args[0])
				return nil
			}
			return err
		}
		return command(args[1:]...)
	}
	return cli.CmdHelp()
}

func (cli *Cli) noSuchCommand(command string) {
	if cli.Stderr == nil {
		cli.Stderr = os.Stderr
	}
	fmt.Fprintf(cli.Stderr, "docker: '%s' is not a docker command.\nSee 'docker --help'.\n", command)
	os.Exit(1)
}

// Command returns a command handler, or nil if the command does not exist
func (cli *Cli) Command(name string) func(...string) error {
	return map[string]func(...string) error{
		"help": cli.CmdHelp,
	}[name]
}

// CmdHelp displays information on a Docker command.
//
// If more than one command is specified, information is only shown for the first command.
//
// Usage: docker help COMMAND or docker COMMAND --help
func (cli *Cli) CmdHelp(args ...string) error {
	if len(args) > 1 {
		command, err := cli.command(args[:2]...)
		if err == nil {
			command("--help")
			return nil
		}
		if err != errCommandNotFound {
			return err
		}
	}
	if len(args) > 0 {
		command, err := cli.command(args[0])
		if err != nil {
			if err == errCommandNotFound {
				cli.noSuchCommand(args[0])
				return nil
			}
			return err
		}
		command("--help")
		return nil
	}

	if cli.Usage == nil {
		flag.Usage()
	} else {
		cli.Usage()
	}

	return nil
}

// Subcmd is a subcommand of the main "docker" command.
// A subcommand represents an action that can be performed
// from the Docker command line client.
//
// To see all available subcommands, run "docker --help".
func Subcmd(name string, synopses []string, description string, exitOnError bool) *flag.FlagSet {
	var errorHandling flag.ErrorHandling
	if exitOnError {
		errorHandling = flag.ExitOnError
	} else {
		errorHandling = flag.ContinueOnError
	}
	flags := flag.NewFlagSet(name, errorHandling)
	flags.Usage = func() {
		flags.ShortUsage()
		flags.PrintDefaults()
	}

	flags.ShortUsage = func() {
		options := ""
		if flags.FlagCountUndeprecated() > 0 {
			options = " [OPTIONS]"
		}

		if len(synopses) == 0 {
			synopses = []string{""}
		}

		// Allow for multiple command usage synopses.
		for i, synopsis := range synopses {
			lead := "\t"
			if i == 0 {
				// First line needs the word 'Usage'.
				lead = "Usage:\t"
			}

			if synopsis != "" {
				synopsis = " " + synopsis
			}

			fmt.Fprintf(flags.Out(), "\n%sdocker %s%s%s", lead, name, options, synopsis)
		}

		fmt.Fprintf(flags.Out(), "\n\n%s\n", description)
	}

	return flags
}

// StatusError reports an unsuccessful exit by a command.
type StatusError struct {
	Status     string
	StatusCode int
}

func (e StatusError) Error() string {
	return fmt.Sprintf("Status: %s, Code: %d", e.Status, e.StatusCode)
}
