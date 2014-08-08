package helpinfo

import (
	"strings"
	"sync"
)

/*
The idea being for relaying to a client, the options available for particular
flags, where there is a free form argument allowed, but only expected args will
be parsed. This way the usae can be dynamically access, even if the client does
not have the same options compiled as the server.

  --storage-opt=[]                             Set storage driver options:      _                    _
                                                * foo.bar = blah blah blah blah |  2 lines, 1 blurb  |
                                                  blah blah blah blah blah blah _                    |
                                                * foo.bar = blah blah blah blah                      |  2 blurbs
                                                  blah blah blah blah blah blah                      _
                                                  |---------------------------|

                                                ^ asterisk would be added in client formating if desired

*/

func RegisterHelpInfo(command, flag string, blurb Blurb) error {
	return DefaultHelpInfo.Add(command, flag, blurb)
}
func Commands() []string {
	return DefaultHelpInfo.Commands()
}
func Flags(command string) []string {
	return DefaultHelpInfo.Flags(command)
}
func Blurbs(command, flag string) []Blurb {
	return DefaultHelpInfo.Blurbs(command, flag)
}

type Blurb []string
type FlagBlurbs map[string][]Blurb
type CommandFlagBlurbs map[string]FlagBlurbs

var UsageBlurbPrefix = "- "

func UsageFormat(blurbs []Blurb) string {
	if len(blurbs) == 0 {
		return ""
	}
	buf := ""
	indent := len(UsageBlurbPrefix)
	lastBlurb := len(blurbs) - 1
	for i, blurb := range blurbs {
		if len(blurb) == 0 {
			continue
		}
		buf = buf + UsageBlurbPrefix + blurb[0] + "\n"
		lastLine := len(blurb[1:]) - 1
		for j, line := range blurb[1:] {
			buf = buf + strings.Repeat(" ", indent) + line
			if j < lastLine {
				buf = buf + "\n"
			}
		}
		if i < lastBlurb {
			buf = buf + "\n"
		}
	}
	return buf
}

var DefaultHelpInfo = HelpInfo{}

type HelpInfo struct {
	c          CommandFlagBlurbs `json:"flags"`
	sync.Mutex `json:"-"`
}

// Names of the commands with information added
func (hi HelpInfo) Commands() (commands []string) {
	hi.Lock()
	defer hi.Unlock()
	for k := range hi.c {
		commands = append(commands, k)
	}
	return commands
}

func (hi HelpInfo) Flags(command string) (flags []string) {
	hi.Lock()
	defer hi.Unlock()
	for k := range hi.c[command] {
		flags = append(flags, k)
	}
	return flags
}
func (hi HelpInfo) Blurbs(command, flag string) []Blurb {
	return hi.c[command][flag]
}

func (hi *HelpInfo) Add(command, flag string, blurb Blurb) error {
	hi.Lock()
	defer hi.Unlock()
	if hi.c == nil {
		hi.c = CommandFlagBlurbs{}
	}
	if _, ok := hi.c[command]; !ok {
		hi.c[command] = FlagBlurbs{flag: []Blurb{blurb}}
		return nil
	}
	if _, ok := hi.c[command][flag]; !ok {
		hi.c[command][flag] = []Blurb{blurb}
		return nil
	}
	hi.c[command][flag] = append(hi.c[command][flag], blurb)
	return nil
}
