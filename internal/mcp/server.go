package mcp

import (
	"io"

	"github.com/spf13/cobra"

	"kbase/internal/log"
	"kbase/internal/mcp/protocol"
)

// Located is the project a server located from where it started — its git
// worktree root — or none, and where none whether the server locates none by
// its mode, every call naming its project. The zero value is none, with
// calls free to omit the project.
type Located struct {
	root     string
	found    bool
	required bool
}

// LocatedAt is the located project whose git worktree root is root.
func LocatedAt(root string) Located { return Located{root: root, found: true} }

// ProjectRequired is the located project of a server whose every call names
// its project: none.
func ProjectRequired() Located { return Located{required: true} }

// Root is the located project's git worktree root, and whether there is one.
func (l Located) Root() (string, bool) { return l.root, l.found }

// Required is whether every call must name its project.
func (l Located) Required() bool { return l.required }

// Server is the MCP server over the command tree under Root. A call runs in
// the project it names, else in Located; the resources read Located.
type Server struct {
	// Version and Instructions are what initialize reports.
	Version, Instructions string
	Root                  *cobra.Command
	Located               Located
	// Stderr takes a command tool's diagnostics.
	Stderr  io.Writer
	Logger  log.Logger
	Globals Globals
	// WriteOp and RenderError are Calls' fields of the same name.
	WriteOp     func(dir, op string, values []byte, out io.Writer) error
	RenderError func(err error, out, stderr io.Writer)
	// Launches is the launch of each command whose tool starts it as a
	// process of its own.
	Launches map[*cobra.Command]Launch
	// Listed is the tools no command stands behind.
	Listed []ProjectTool
	// Families is the family tools, Calls' field of the same name; the help
	// resources serve each one's help.
	Families []Family
	// Resources is the resource families, listed in order. At most one offers
	// a subscription.
	Resources []ResourceFamily
}

// serving is what one Serve binds its launches and resources to: the located
// project, the build it launched last, its logger and its family tools.
type serving struct {
	located  Located
	last     *lastBuild
	lg       log.Logger
	families []Family
}

// Serve serves s over in and out until EOF on in.
func (s Server) Serve(in io.Reader, out io.Writer) error {
	sv := serving{located: s.Located, last: newLastBuild(s.Located), lg: s.Logger, families: s.Families}
	detached := make(map[*cobra.Command]func(*cobra.Command) (ProjectTool, error), len(s.Launches))
	for cmd, l := range s.Launches {
		detached[cmd] = func(cmd *cobra.Command) (ProjectTool, error) { return launchTool(cmd, l, sv, s.Globals) }
	}
	tools, err := Tools(s.Root, s.Located, s.Stderr, Calls{WriteOp: s.WriteOp, RenderError: s.RenderError, Detached: detached,
		Listed: s.Listed, Families: s.Families})
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
