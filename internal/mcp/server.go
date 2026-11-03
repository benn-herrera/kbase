package mcp

import (
	"io"

	"github.com/spf13/cobra"

	"kbase/internal/argmark"
	"kbase/internal/log"
	"kbase/internal/mcp/protocol"
)

// Located is the project a server located from where it started — its git
// worktree root — or none, where every call names its project. The zero value
// is none.
type Located struct {
	root string
}

// LocatedAt is the located project whose git worktree root is root.
func LocatedAt(root string) Located { return Located{root: root} }

// ProjectRequired is the located project of a server whose every call names
// its project: none.
func ProjectRequired() Located { return Located{} }

// Root is the located project's git worktree root, and whether there is one.
func (l Located) Root() (string, bool) { return l.root, l.root != "" }

// Required is whether every call must name its project: where none is located.
func (l Located) Required() bool { return l.root == "" }

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
	// Families is the family tools, Calls' field of the same name; the help
	// resources serve each one's help.
	Families []Family
	// Resources is the resource families, listed in order. None is
	// subscribable.
	Resources []ResourceFamily
}

// serving is what one Serve binds its launches and resources to: the located
// project, its logger, its family tools and how a document it serves is
// spelled over MCP.
type serving struct {
	located  Located
	lg       log.Logger
	families []Family
	spelling argmark.Spelling
}

// Serve serves s over in and out until EOF on in.
func (s Server) Serve(in io.Reader, out io.Writer) error {
	sv := serving{located: s.Located, lg: s.Logger, families: s.Families, spelling: Spelling(s.Root, true)}
	detached := make(map[*cobra.Command]func(*cobra.Command) (ProjectTool, error), len(s.Launches))
	for cmd, l := range s.Launches {
		detached[cmd] = func(cmd *cobra.Command) (ProjectTool, error) { return launchTool(cmd, l, sv, s.Globals) }
	}
	tools, err := Tools(s.Root, s.Located, s.Stderr, Calls{WriteOp: s.WriteOp, RenderError: s.RenderError, Detached: detached,
		Families: s.Families, Logger: s.Logger})
	if err != nil {
		return err
	}
	families := make(resourceFamilies, len(s.Resources))
	for i, f := range s.Resources {
		families[i] = boundFamily{Resources: f.bind(sv), prefix: f.prefix}
	}
	return protocol.Serve(in, out, s.Version, s.Instructions, tools, families, s.Logger)
}
