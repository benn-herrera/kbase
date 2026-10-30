package mcp

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/mcp/protocol"
)

// HelpURIPrefix leads a command's help resource's URI; the command's name
// follows it.
const HelpURIPrefix = "kbase://help/"

// HelpResources serves the help of each command under root that a tool binds:
// the text `kbase <command> --help` prints, which needs no KB.
func HelpResources(root *cobra.Command) ResourceFamily {
	return ResourceFamily{prefix: HelpURIPrefix, bind: func(serving) (protocol.Resources, *protocol.Subscription) {
		return helpResources{root}, nil
	}}
}

type helpResources struct{ root *cobra.Command }

func (r helpResources) List() ([]protocol.Resource, error) {
	var listed []protocol.Resource
	for _, c := range boundCommands(r.root) {
		listed = append(listed, protocol.Resource{URI: HelpURIPrefix + c.Name(), Name: c.CommandPath() + " --help", MimeType: "text/plain"})
	}
	return listed, nil
}

func (r helpResources) Read(res protocol.Resource) (string, error) {
	name := strings.TrimPrefix(res.URI, HelpURIPrefix)
	commands := boundCommands(r.root)
	i := slices.IndexFunc(commands, func(c *cobra.Command) bool { return c.Name() == name })
	if i < 0 {
		return "", fmt.Errorf("no tool binds a command named %q", name)
	}
	return commandHelp(commands[i]), nil
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
