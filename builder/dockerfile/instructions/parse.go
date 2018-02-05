package instructions // import "github.com/docker/docker/builder/dockerfile/instructions"

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/builder/dockerfile/command"
	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/pkg/errors"
)

type parseRequest struct {
	command    string
	args       []string
	attributes map[string]bool
	flags      *BFlags
	original   string
}

func nodeArgs(node *parser.Node) []string {
	result := []string{}
	for ; node.Next != nil; node = node.Next {
		arg := node.Next
		if len(arg.Children) == 0 {
			result = append(result, arg.Value)
		} else if len(arg.Children) == 1 {
			//sub command
			result = append(result, arg.Children[0].Value)
			result = append(result, nodeArgs(arg.Children[0])...)
		}
	}
	return result
}

func newParseRequestFromNode(node *parser.Node) parseRequest {
	return parseRequest{
		command:    node.Value,
		args:       nodeArgs(node),
		attributes: node.Attributes,
		original:   node.Original,
		flags:      NewBFlagsWithArgs(node.Flags),
	}
}

// ParseInstruction converts an AST to a typed instruction (either a command or a build stage beginning when encountering a `FROM` statement)
func ParseInstruction(node *parser.Node) (interface{}, error) {
	req := newParseRequestFromNode(node)
	switch node.Value {
	case command.Env:
		return parseEnv(req)
	case command.Maintainer:
		return parseMaintainer(req)
	case command.Label:
		return parseLabel(req)
	case command.Add:
		return parseAdd(req)
	case command.Copy:
		return parseCopy(req)
	case command.From:
		return parseFrom(req)
	case command.Onbuild:
		return parseOnBuild(req)
	case command.Workdir:
		return parseWorkdir(req)
	case command.Run:
		return parseRun(req)
	case command.Cmd:
		return parseCmd(req)
	case command.Healthcheck:
		return parseHealthcheck(req)
	case command.Entrypoint:
		return parseEntrypoint(req)
	case command.Expose:
		return parseExpose(req)
	case command.User:
		return parseUser(req)
	case command.Volume:
		return parseVolume(req)
	case command.StopSignal:
		return parseStopSignal(req)
	case command.Arg:
		return parseArg(req)
	case command.Shell:
		return parseShell(req)
	}

	return nil, &UnknownInstruction{Instruction: node.Value, Line: node.StartLine}
}

// ParseCommand converts an AST to a typed Command
func ParseCommand(node *parser.Node) (Command, error) {
	s, err := ParseInstruction(node)
	if err != nil {
		return nil, err
	}
	if c, ok := s.(Command); ok {
		return c, nil
	}
	return nil, errors.Errorf("%T is not a command type", s)
}

// UnknownInstruction represents an error occurring when a command is unresolvable
type UnknownInstruction struct {
	Line        int
	Instruction string
}

func (e *UnknownInstruction) Error() string {
	return fmt.Sprintf("unknown instruction: %s", strings.ToUpper(e.Instruction))
}

// IsUnknownInstruction checks if the error is an UnknownInstruction or a parseError containing an UnknownInstruction
func IsUnknownInstruction(err error) bool {
	_, ok := err.(*UnknownInstruction)
	if !ok {
		var pe *parseError
		if pe, ok = err.(*parseError); ok {
			_, ok = pe.inner.(*UnknownInstruction)
		}
	}
	return ok
}

type parseError struct {
	inner error
	node  *parser.Node
}

func (e *parseError) Error() string {
	return fmt.Sprintf("Dockerfile parse error line %d: %v", e.node.StartLine, e.inner.Error())
}

// Parse a docker file into a collection of buildable stages
func Parse(ast *parser.Node) (stages []Stage, metaArgs []ArgCommand, err error) {
	for _, n := range ast.Children {
		cmd, err := ParseInstruction(n)
		if err != nil {
			return nil, nil, &parseError{inner: err, node: n}
		}
		if len(stages) == 0 {
			// meta arg case
			if a, isArg := cmd.(*ArgCommand); isArg {
				metaArgs = append(metaArgs, *a)
				continue
			}
		}
		switch c := cmd.(type) {
		case *Stage:
			stages = append(stages, *c)
		case Command:
			stage, err := CurrentStage(stages)
			if err != nil {
				return nil, nil, err
			}
			stage.AddCommand(c)
		default:
			return nil, nil, errors.Errorf("%T is not a command type", cmd)
		}

	}
	return stages, metaArgs, nil
}

func parseKvps(args []string, cmdName string) (KeyValuePairs, error) {
	if len(args) == 0 {
		return nil, errAtLeastOneArgument(cmdName)
	}
	if len(args)%2 != 0 {
		// should never get here, but just in case
		return nil, errTooManyArguments(cmdName)
	}
	var res KeyValuePairs
	for j := 0; j < len(args); j += 2 {
		if len(args[j]) == 0 {
			return nil, errBlankCommandNames(cmdName)
		}
		name := args[j]
		value := args[j+1]
		res = append(res, KeyValuePair{Key: name, Value: value})
	}
	return res, nil
}

func parseEnv(req parseRequest) (*EnvCommand, error) {

	if err := req.flags.Parse(); err != nil {
		return nil, err
	}
	envs, err := parseKvps(req.args, "ENV")
	if err != nil {
		return nil, err
	}
	return &EnvCommand{
		Env:             envs,
		withNameAndCode: newWithNameAndCode(req),
	}, nil
}

func parseMaintainer(req parseRequest) (*MaintainerCommand, error) {
	if len(req.args) != 1 {
		return nil, errExactlyOneArgument("MAINTAINER")
	}

	if err := req.flags.Parse(); err != nil {
		return nil, err
	}
	return &MaintainerCommand{
		Maintainer:      req.args[0],
		withNameAndCode: newWithNameAndCode(req),
	}, nil
}

func parseLabel(req parseRequest) (*LabelCommand, error) {

	if err := req.flags.Parse(); err != nil {
		return nil, err
	}

	labels, err := parseKvps(req.args, "LABEL")
	if err != nil {
		return nil, err
	}

	return &LabelCommand{
		Labels:          labels,
		withNameAndCode: newWithNameAndCode(req),
	}, nil
}

func parseAdd(req parseRequest) (*AddCommand, error) {
	if len(req.args) < 2 {
		return nil, errNoDestinationArgument("ADD")
	}
	flChown := req.flags.AddString("chown", "")
	if err := req.flags.Parse(); err != nil {
		return nil, err
	}
	return &AddCommand{
		SourcesAndDest:  SourcesAndDest(req.args),
		withNameAndCode: newWithNameAndCode(req),
		Chown:           flChown.Value,
	}, nil
}

func parseCopy(req parseRequest) (*CopyCommand, error) {
	if len(req.args) < 2 {
		return nil, errNoDestinationArgument("COPY")
	}
	flChown := req.flags.AddString("chown", "")
	flFrom := req.flags.AddString("from", "")
	if err := req.flags.Parse(); err != nil {
		return nil, err
	}
	return &CopyCommand{
		SourcesAndDest:  SourcesAndDest(req.args),
		From:            flFrom.Value,
		withNameAndCode: newWithNameAndCode(req),
		Chown:           flChown.Value,
	}, nil
}

func parseFrom(req parseRequest) (*Stage, error) {
	stageName, err := parseBuildStageName(req.args)
	if err != nil {
		return nil, err
	}

	if err := req.flags.Parse(); err != nil {
		return nil, err
	}
	code := strings.TrimSpace(req.original)

	return &Stage{
		BaseName:   req.args[0],
		Name:       stageName,
		SourceCode: code,
		Commands:   []Command{},
	}, nil

}

func parseBuildStageName(args []string) (string, error) {
	stageName := ""
	switch {
	case len(args) == 3 && strings.EqualFold(args[1], "as"):
		stageName = strings.ToLower(args[2])
		if ok, _ := regexp.MatchString("^[a-z][a-z0-9-_\\.]*$", stageName); !ok {
			return "", errors.Errorf("invalid name for build stage: %q, name can't start with a number or contain symbols", stageName)
		}
	case len(args) != 1:
		return "", errors.New("FROM requires either one or three arguments")
	}

	return stageName, nil
}

func parseOnBuild(req parseRequest) (*OnbuildCommand, error) {
	if len(req.args) == 0 {
		return nil, errAtLeastOneArgument("ONBUILD")
	}
	if err := req.flags.Parse(); err != nil {
		return nil, err
	}

	triggerInstruction := strings.ToUpper(strings.TrimSpace(req.args[0]))
	switch strings.ToUpper(triggerInstruction) {
	case "ONBUILD":
		return nil, errors.New("Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed")
	case "MAINTAINER", "FROM":
		return nil, fmt.Errorf("%s isn't allowed as an ONBUILD trigger", triggerInstruction)
	}

	original := regexp.MustCompile(`(?i)^\s*ONBUILD\s*`).ReplaceAllString(req.original, "")
	return &OnbuildCommand{
		Expression:      original,
		withNameAndCode: newWithNameAndCode(req),
	}, nil

}

func parseWorkdir(req parseRequest) (*WorkdirCommand, error) {
	if len(req.args) != 1 {
		return nil, errExactlyOneArgument("WORKDIR")
	}

	err := req.flags.Parse()
	if err != nil {
		return nil, err
	}
	return &WorkdirCommand{
		Path:            req.args[0],
		withNameAndCode: newWithNameAndCode(req),
	}, nil

}

func parseShellDependentCommand(req parseRequest, emptyAsNil bool) ShellDependantCmdLine {
	args := handleJSONArgs(req.args, req.attributes)
	cmd := strslice.StrSlice(args)
	if emptyAsNil && len(cmd) == 0 {
		cmd = nil
	}
	return ShellDependantCmdLine{
		CmdLine:      cmd,
		PrependShell: !req.attributes["json"],
	}
}

func parseRun(req parseRequest) (*RunCommand, error) {

	if err := req.flags.Parse(); err != nil {
		return nil, err
	}
	return &RunCommand{
		ShellDependantCmdLine: parseShellDependentCommand(req, false),
		withNameAndCode:       newWithNameAndCode(req),
	}, nil

}

func parseCmd(req parseRequest) (*CmdCommand, error) {
	if err := req.flags.Parse(); err != nil {
		return nil, err
	}
	return &CmdCommand{
		ShellDependantCmdLine: parseShellDependentCommand(req, false),
		withNameAndCode:       newWithNameAndCode(req),
	}, nil

}

func parseEntrypoint(req parseRequest) (*EntrypointCommand, error) {
	if err := req.flags.Parse(); err != nil {
		return nil, err
	}

	cmd := &EntrypointCommand{
		ShellDependantCmdLine: parseShellDependentCommand(req, true),
		withNameAndCode:       newWithNameAndCode(req),
	}

	return cmd, nil
}

// parseOptInterval(flag) is the duration of flag.Value, or 0 if
// empty. An error is reported if the value is given and less than minimum duration.
func parseOptInterval(f *Flag) (time.Duration, error) {
	s := f.Value
	if s == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	if d < container.MinimumDuration {
		return 0, fmt.Errorf("Interval %#v cannot be less than %s", f.name, container.MinimumDuration)
	}
	return d, nil
}
func parseHealthcheck(req parseRequest) (*HealthCheckCommand, error) {
	if len(req.args) == 0 {
		return nil, errAtLeastOneArgument("HEALTHCHECK")
	}
	cmd := &HealthCheckCommand{
		withNameAndCode: newWithNameAndCode(req),
	}

	typ := strings.ToUpper(req.args[0])
	args := req.args[1:]
	if typ == "NONE" {
		if len(args) != 0 {
			return nil, errors.New("HEALTHCHECK NONE takes no arguments")
		}
		test := strslice.StrSlice{typ}
		cmd.Health = &container.HealthConfig{
			Test: test,
		}
	} else {

		healthcheck := container.HealthConfig{}

		flInterval := req.flags.AddString("interval", "")
		flTimeout := req.flags.AddString("timeout", "")
		flStartPeriod := req.flags.AddString("start-period", "")
		flRetries := req.flags.AddString("retries", "")

		if err := req.flags.Parse(); err != nil {
			return nil, err
		}

		switch typ {
		case "CMD":
			cmdSlice := handleJSONArgs(args, req.attributes)
			if len(cmdSlice) == 0 {
				return nil, errors.New("Missing command after HEALTHCHECK CMD")
			}

			if !req.attributes["json"] {
				typ = "CMD-SHELL"
			}

			healthcheck.Test = strslice.StrSlice(append([]string{typ}, cmdSlice...))
		default:
			return nil, fmt.Errorf("Unknown type %#v in HEALTHCHECK (try CMD)", typ)
		}

		interval, err := parseOptInterval(flInterval)
		if err != nil {
			return nil, err
		}
		healthcheck.Interval = interval

		timeout, err := parseOptInterval(flTimeout)
		if err != nil {
			return nil, err
		}
		healthcheck.Timeout = timeout

		startPeriod, err := parseOptInterval(flStartPeriod)
		if err != nil {
			return nil, err
		}
		healthcheck.StartPeriod = startPeriod

		if flRetries.Value != "" {
			retries, err := strconv.ParseInt(flRetries.Value, 10, 32)
			if err != nil {
				return nil, err
			}
			if retries < 1 {
				return nil, fmt.Errorf("--retries must be at least 1 (not %d)", retries)
			}
			healthcheck.Retries = int(retries)
		} else {
			healthcheck.Retries = 0
		}

		cmd.Health = &healthcheck
	}
	return cmd, nil
}

func parseExpose(req parseRequest) (*ExposeCommand, error) {
	portsTab := req.args

	if len(req.args) == 0 {
		return nil, errAtLeastOneArgument("EXPOSE")
	}

	if err := req.flags.Parse(); err != nil {
		return nil, err
	}

	sort.Strings(portsTab)
	return &ExposeCommand{
		Ports:           portsTab,
		withNameAndCode: newWithNameAndCode(req),
	}, nil
}

func parseUser(req parseRequest) (*UserCommand, error) {
	if len(req.args) != 1 {
		return nil, errExactlyOneArgument("USER")
	}

	if err := req.flags.Parse(); err != nil {
		return nil, err
	}
	return &UserCommand{
		User:            req.args[0],
		withNameAndCode: newWithNameAndCode(req),
	}, nil
}

func parseVolume(req parseRequest) (*VolumeCommand, error) {
	if len(req.args) == 0 {
		return nil, errAtLeastOneArgument("VOLUME")
	}

	if err := req.flags.Parse(); err != nil {
		return nil, err
	}

	cmd := &VolumeCommand{
		withNameAndCode: newWithNameAndCode(req),
	}

	for _, v := range req.args {
		v = strings.TrimSpace(v)
		if v == "" {
			return nil, errors.New("VOLUME specified can not be an empty string")
		}
		cmd.Volumes = append(cmd.Volumes, v)
	}
	return cmd, nil

}

func parseStopSignal(req parseRequest) (*StopSignalCommand, error) {
	if len(req.args) != 1 {
		return nil, errExactlyOneArgument("STOPSIGNAL")
	}
	sig := req.args[0]

	cmd := &StopSignalCommand{
		Signal:          sig,
		withNameAndCode: newWithNameAndCode(req),
	}
	return cmd, nil

}

func parseArg(req parseRequest) (*ArgCommand, error) {
	if len(req.args) != 1 {
		return nil, errExactlyOneArgument("ARG")
	}

	var (
		name     string
		newValue *string
	)

	arg := req.args[0]
	// 'arg' can just be a name or name-value pair. Note that this is different
	// from 'env' that handles the split of name and value at the parser level.
	// The reason for doing it differently for 'arg' is that we support just
	// defining an arg and not assign it a value (while 'env' always expects a
	// name-value pair). If possible, it will be good to harmonize the two.
	if strings.Contains(arg, "=") {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts[0]) == 0 {
			return nil, errBlankCommandNames("ARG")
		}

		name = parts[0]
		newValue = &parts[1]
	} else {
		name = arg
	}

	return &ArgCommand{
		Key:             name,
		Value:           newValue,
		withNameAndCode: newWithNameAndCode(req),
	}, nil
}

func parseShell(req parseRequest) (*ShellCommand, error) {
	if err := req.flags.Parse(); err != nil {
		return nil, err
	}
	shellSlice := handleJSONArgs(req.args, req.attributes)
	switch {
	case len(shellSlice) == 0:
		// SHELL []
		return nil, errAtLeastOneArgument("SHELL")
	case req.attributes["json"]:
		// SHELL ["powershell", "-command"]

		return &ShellCommand{
			Shell:           strslice.StrSlice(shellSlice),
			withNameAndCode: newWithNameAndCode(req),
		}, nil
	default:
		// SHELL powershell -command - not JSON
		return nil, errNotJSON("SHELL", req.original)
	}
}

func errAtLeastOneArgument(command string) error {
	return errors.Errorf("%s requires at least one argument", command)
}

func errExactlyOneArgument(command string) error {
	return errors.Errorf("%s requires exactly one argument", command)
}

func errNoDestinationArgument(command string) error {
	return errors.Errorf("%s requires at least two arguments, but only one was provided. Destination could not be determined.", command)
}

func errBlankCommandNames(command string) error {
	return errors.Errorf("%s names can not be blank", command)
}

func errTooManyArguments(command string) error {
	return errors.Errorf("Bad input to %s, too many arguments", command)
}
