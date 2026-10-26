package mcp

import (
	"io"

	"github.com/spf13/cobra"

	"kbase/internal/log"
	"kbase/internal/mcp/protocol"
)

// Server is the MCP server over the command tree under Root, each tool and
// resource finding its KB from Dir.
type Server struct {
	// Version and Instructions are what initialize reports.
	Version, Instructions string
	Root                  *cobra.Command
	Dir                   string
	// Stderr takes a command tool's diagnostics.
	Stderr  io.Writer
	Logger  log.Logger
	Globals Globals
	// WriteOp and RenderError are Calls' fields of the same name.
	WriteOp     func(op string, values []byte, out io.Writer) error
	RenderError func(err error, out, stderr io.Writer)
	// Launches is the launch of each command whose tool starts it as a
	// process of its own.
	Launches map[*cobra.Command]Launch
	// Listed is the tools no command stands behind, each made for the
	// directory served.
	Listed []func(dir string) protocol.Tool
	// Resources is the resource families, listed in order. At most one offers
	// a subscription.
	Resources []ResourceFamily
}

// serving is what one Serve binds its launches and resources to: the
// directory served, the build it launched last, and its logger.
type serving struct {
	dir  string
	last *lastBuild
	lg   log.Logger
}

// Serve serves s over in and out until EOF on in.
func (s Server) Serve(in io.Reader, out io.Writer) error {
	sv := serving{dir: s.Dir, last: newLastBuild(s.Dir), lg: s.Logger}
	detached := make(map[*cobra.Command]func(*cobra.Command) (protocol.Tool, error), len(s.Launches))
	for cmd, l := range s.Launches {
		detached[cmd] = func(cmd *cobra.Command) (protocol.Tool, error) { return launchTool(cmd, l, sv, s.Globals) }
	}
	listed := make([]protocol.Tool, len(s.Listed))
	for i, tool := range s.Listed {
		listed[i] = tool(s.Dir)
	}
	tools, err := Tools(s.Root, s.Dir, s.Stderr, Calls{WriteOp: s.WriteOp, RenderError: s.RenderError, Detached: detached, Listed: listed})
	if err != nil {
		return err
	}
	families := make(resourceFamilies, len(s.Resources))
	var subscription *protocol.Subscription
	for i, f := range s.Resources {
		resources, offered := f.bind(sv)
		families[i] = boundFamily{Resources: resources, prefix: f.prefix}
		if offered != nil {
			subscription = offered
		}
	}
	return protocol.Serve(in, out, s.Version, s.Instructions, tools, families, subscription, s.Logger)
}
