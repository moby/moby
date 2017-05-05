package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"
)

type cmdOption struct {
	Option       string
	Shorthand    string `yaml:",omitempty"`
	DefaultValue string `yaml:"default_value,omitempty"`
	Description  string `yaml:",omitempty"`
}

type cmdDoc struct {
	Name             string      `yaml:"command"`
	SeeAlso          []string    `yaml:"parent,omitempty"`
	Version          string      `yaml:"engine_version,omitempty"`
	Aliases          string      `yaml:",omitempty"`
	Short            string      `yaml:",omitempty"`
	Long             string      `yaml:",omitempty"`
	Usage            string      `yaml:",omitempty"`
	Pname            string      `yaml:",omitempty"`
	Plink            string      `yaml:",omitempty"`
	Cname            []string    `yaml:",omitempty"`
	Clink            []string    `yaml:",omitempty"`
	Options          []cmdOption `yaml:",omitempty"`
	InheritedOptions []cmdOption `yaml:"inherited_options,omitempty"`
	Example          string      `yaml:"examples,omitempty"`
}

// GenYamlTree creates yaml structured ref files
func GenYamlTree(cmd *cobra.Command, dir string) error {
	identity := func(s string) string { return s }
	emptyStr := func(s string) string { return "" }
	return GenYamlTreeCustom(cmd, dir, emptyStr, identity)
}

// GenYamlTreeCustom creates yaml structured ref files
func GenYamlTreeCustom(cmd *cobra.Command, dir string, filePrepender, linkHandler func(string) string) error {
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsHelpCommand() {
			continue
		}
		if err := GenYamlTreeCustom(c, dir, filePrepender, linkHandler); err != nil {
			return err
		}
	}

	basename := strings.Replace(cmd.CommandPath(), " ", "_", -1) + ".yaml"
	filename := filepath.Join(dir, basename)
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.WriteString(f, filePrepender(filename)); err != nil {
		return err
	}
	if err := GenYamlCustom(cmd, f, linkHandler); err != nil {
		return err
	}
	return nil
}

// GenYamlCustom creates custom yaml output
func GenYamlCustom(cmd *cobra.Command, w io.Writer, linkHandler func(string) string) error {
	cliDoc := cmdDoc{}
	cliDoc.Name = cmd.CommandPath()

	// Check experimental: ok := cmd.Tags["experimental"]

	cliDoc.Aliases = strings.Join(cmd.Aliases, ", ")
	cliDoc.Short = cmd.Short
	cliDoc.Long = cmd.Long
	if len(cliDoc.Long) == 0 {
		cliDoc.Long = cliDoc.Short
	}

	if cmd.Runnable() {
		cliDoc.Usage = cmd.UseLine()
	}

	if len(cmd.Example) > 0 {
		cliDoc.Example = cmd.Example
	}

	flags := cmd.NonInheritedFlags()
	if flags.HasFlags() {
		cliDoc.Options = genFlagResult(flags)
	}
	flags = cmd.InheritedFlags()
	if flags.HasFlags() {
		cliDoc.InheritedOptions = genFlagResult(flags)
	}

	if hasSeeAlso(cmd) {
		if cmd.HasParent() {
			parent := cmd.Parent()
			cliDoc.Pname = parent.CommandPath()
			link := cliDoc.Pname + ".yaml"
			cliDoc.Plink = strings.Replace(link, " ", "_", -1)
			cmd.VisitParents(func(c *cobra.Command) {
				if c.DisableAutoGenTag {
					cmd.DisableAutoGenTag = c.DisableAutoGenTag
				}
			})
		}

		children := cmd.Commands()
		sort.Sort(byName(children))

		for _, child := range children {
			if !child.IsAvailableCommand() || child.IsHelpCommand() {
				continue
			}
			currentChild := cliDoc.Name + " " + child.Name()
			cliDoc.Cname = append(cliDoc.Cname, cliDoc.Name+" "+child.Name())
			link := currentChild + ".yaml"
			cliDoc.Clink = append(cliDoc.Clink, strings.Replace(link, " ", "_", -1))
		}
	}

	final, err := yaml.Marshal(&cliDoc)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if _, err := fmt.Fprintln(w, string(final)); err != nil {
		return err
	}
	return nil
}

func genFlagResult(flags *pflag.FlagSet) []cmdOption {
	var result []cmdOption

	flags.VisitAll(func(flag *pflag.Flag) {
		// Todo, when we mark a shorthand is deprecated, but specify an empty message.
		// The flag.ShorthandDeprecated is empty as the shorthand is deprecated.
		// Using len(flag.ShorthandDeprecated) > 0 can't handle this, others are ok.
		if !(len(flag.ShorthandDeprecated) > 0) && len(flag.Shorthand) > 0 {
			opt := cmdOption{
				Option:       flag.Name,
				Shorthand:    flag.Shorthand,
				DefaultValue: flag.DefValue,
				Description:  forceMultiLine(flag.Usage),
			}
			result = append(result, opt)
		} else {
			opt := cmdOption{
				Option:       flag.Name,
				DefaultValue: forceMultiLine(flag.DefValue),
				Description:  forceMultiLine(flag.Usage),
			}
			result = append(result, opt)
		}
	})

	return result
}

// Temporary workaround for yaml lib generating incorrect yaml with long strings
// that do not contain \n.
func forceMultiLine(s string) string {
	if len(s) > 60 && !strings.Contains(s, "\n") {
		s = s + "\n"
	}
	return s
}

// Small duplication for cobra utils
func hasSeeAlso(cmd *cobra.Command) bool {
	if cmd.HasParent() {
		return true
	}
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsHelpCommand() {
			continue
		}
		return true
	}
	return false
}

func parseMDContent(mdString string) (description string, examples string) {
	parsedContent := strings.Split(mdString, "\n## ")
	for _, s := range parsedContent {
		if strings.Index(s, "Description") == 0 {
			description = strings.Trim(s, "Description\n")
		}
		if strings.Index(s, "Examples") == 0 {
			examples = strings.Trim(s, "Examples\n")
		}
	}
	return
}

type byName []*cobra.Command

func (s byName) Len() int           { return len(s) }
func (s byName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s byName) Less(i, j int) bool { return s[i].Name() < s[j].Name() }
