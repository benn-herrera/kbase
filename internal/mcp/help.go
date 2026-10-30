package mcp

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/mcp/protocol"
)

// HelpURIPrefix leads a help resource's URI: a family tool's name or a
// command's follows it.
const HelpURIPrefix = "kbase://help/"

// HelpResources serves the help of each family tool the server serves, then
// of each command under root that a tool binds: for a family, what its
// operations share and their list; for a command, the text
// `kbase <command> --help` prints. None needs a KB.
func HelpResources(root *cobra.Command) ResourceFamily {
	return ResourceFamily{prefix: HelpURIPrefix, bind: func(s serving) (protocol.Resources, *protocol.Subscription) {
		return helpResources{root, s.families}, nil
	}}
}

type helpResources struct {
	root     *cobra.Command
	families []Family
}

func (r helpResources) List() ([]protocol.Resource, error) {
	var listed []protocol.Resource
	for _, f := range r.families {
		listed = append(listed, protocol.Resource{URI: HelpURIPrefix + f.Name, Name: f.Name + " tool", MimeType: "text/plain"})
	}
	for _, c := range boundCommands(r.root) {
		listed = append(listed, protocol.Resource{URI: HelpURIPrefix + c.Name(), Name: c.CommandPath() + " --help", MimeType: "text/plain"})
	}
	return listed, nil
}

func (r helpResources) Read(res protocol.Resource) (string, error) {
	name := strings.TrimPrefix(res.URI, HelpURIPrefix)
	commands := boundCommands(r.root)
	if i := slices.IndexFunc(r.families, func(f Family) bool { return f.Name == name }); i >= 0 {
		return familyHelp(r.families[i], familyMembers(commands, name)), nil
	}
	i := slices.IndexFunc(commands, func(c *cobra.Command) bool { return c.Name() == name })
	if i < 0 {
		return "", fmt.Errorf("no tool binds a command named %q", name)
	}
	return commandHelp(commands[i]), nil
}

// familyHelp is f's help resource: its operations, listed from the members
// its tool's operation names, then what they share.
func familyHelp(f Family, members []*cobra.Command) string {
	names := make([]string, 0, len(members))
	for _, c := range members {
		names = append(names, c.Name())
	}
	return fmt.Sprintf("The %s tool's operations: %s. An operation's full help is %s<operation>.\n\n%s\n",
		f.Name, strings.Join(names, ", "), HelpURIPrefix, strings.TrimSpace(f.Help))
}

// commandHelp is what `kbase <command> --help` prints for cmd: cobra's help
// function writing to a buffer, after the --help flag cobra adds only when it
// executes the command, so its line is listed as the command line lists it.
func commandHelp(cmd *cobra.Command) string {
	cmd.InitDefaultHelpFlag()
	var out bytes.Buffer
	cmd.SetOut(&out)
	defer cmd.SetOut(nil)
	cmd.HelpFunc()(cmd, nil)
	return out.String()
}
