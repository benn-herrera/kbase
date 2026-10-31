package main

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/spf13/cobra"

	"kbase/internal/auth"
	"kbase/internal/config"
	"kbase/internal/log"
	toolresult "kbase/internal/result"
	"kbase/internal/secret"
	"kbase/internal/statedir"
)

// authProviderArg is the positional the add and remove verbs take, spelled as
// their help spells it. A refusal with no single provider to name, or whose
// name is unusable, names this instead.
const authProviderArg = "<provider>"

// authOptions is the input of the auth verbs: the provider name a verb acts
// on, the store's directory, and for add the configured entry names, the
// key's stream and whether it is a terminal.
type authOptions struct {
	Provider        string
	Dir             string
	Configured      []string
	Stdin           io.Reader
	StdinIsTerminal bool
	Stdout          io.Writer
	Logger          log.Logger
}

// runAuthVerb runs one auth verb and writes its result document.
func runAuthVerb(name string, verb func(authOptions) (string, []toolresult.Field), opts authOptions) (int, error) {
	outcome, fields := verb(opts)
	return emitResult(name, opts.Stdout, outcome, fields)
}

// authAddInputLimit is the most bytes of stdin auth add accepts as a key.
const authAddInputLimit = 65536

// authAdd stores the key piped on stdin for the provider, replacing any stored
// for it. An unusable name, a name no configured entry carries, a terminal on
// stdin, input over authAddInputLimit, input holding nothing but whitespace and
// a trimmed key holding a whitespace or control character are refused, in that
// order, before anything is written; the first three before stdin is read. The
// key is trimmed as a key file's is.
func authAdd(opts authOptions) (string, []toolresult.Field) {
	if err := auth.CheckName(opts.Provider); err != nil {
		return authStoreFailure(err, "kbase show-config")
	}
	if !slices.Contains(opts.Configured, opts.Provider) {
		return refused(nil, toolresult.Item{Check: toolresult.CheckUsage, Key: opts.Provider, Remedy: "kbase show-config",
			Detail: fmt.Sprintf("%q is no providers.toml entry's name: give the name exactly as the entry spells it, case included", opts.Provider)})
	}
	inputRefused := func(detail string) (string, []toolresult.Field) {
		return refused(nil, toolresult.Item{Check: toolresult.CheckUsage, Remedy: config.AuthAddRemedy(opts.Provider), Detail: detail})
	}
	if opts.StdinIsTerminal {
		return inputRefused("stdin is a terminal: kbase never prompts for a key, so it reads one from a pipe alone")
	}
	b, err := io.ReadAll(io.LimitReader(opts.Stdin, authAddInputLimit+1))
	if err != nil {
		return failed(nil, fmt.Errorf("read the key from stdin: %w", err))
	}
	if len(b) > authAddInputLimit {
		return inputRefused("stdin held more than 64 KiB: pipe the key alone, not a file or stream that holds it")
	}
	key := strings.TrimSpace(string(b))
	if key == "" {
		return inputRefused("stdin held no key: the piped input was empty or whitespace alone")
	}
	if strings.ContainsFunc(key, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return inputRefused("stdin held whitespace or a control character inside the key: pipe the key alone, not a file or record that contains it")
	}
	if err := auth.NewStore(opts.Dir).Add(opts.Provider, secret.NewKey(key)); err != nil {
		return failed(nil, err)
	}
	opts.Logger.Debug("auth add: key stored", "provider", opts.Provider, "dir", opts.Dir)
	return toolresult.Done, []toolresult.Field{{Key: "provider", Value: opts.Provider}}
}

// authRemove clears the key stored for the provider: unchanged where none was.
func authRemove(opts authOptions) (string, []toolresult.Field) {
	removed, err := auth.NewStore(opts.Dir).Remove(opts.Provider)
	if err != nil {
		return authStoreFailure(err, "kbase auth list")
	}
	outcome := toolresult.Done
	if !removed {
		outcome = toolresult.Unchanged
	}
	return outcome, []toolresult.Field{{Key: "provider", Value: opts.Provider}}
}

// authList names the providers with a stored key, in name order.
func authList(opts authOptions) (string, []toolresult.Field) {
	names, err := auth.NewStore(opts.Dir).List()
	if err != nil {
		return failed(nil, err)
	}
	return toolresult.Done, []toolresult.Field{{Key: "providers", Value: names}}
}

// authStoreFailure is the result of a store error: a name that cannot name a
// key file is a usage refusal keyed by the name as given and carrying remedy,
// anything else a failure.
func authStoreFailure(err error, remedy string) (string, []toolresult.Field) {
	var nameErr *auth.NameError
	if errors.As(err, &nameErr) {
		return refused(nil, toolresult.Item{Check: toolresult.CheckUsage, Key: nameErr.Name, Remedy: remedy,
			Detail: nameErr.Detail})
	}
	return failed(nil, err)
}

// authStoreDir is the credentials store's directory under kbase's state base.
func authStoreDir() (string, error) {
	base, err := statedir.Base()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, auth.DirName), nil
}

// isTerminal reports whether r is a terminal: a file whose mode is a
// character device.
func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// authArgs accepts exactly n positionals and refuses any other count with
// refusal, a fixed text that names no argument given.
func authArgs(n int, refusal toolresult.Item) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) != n {
			return toolresult.Refusal{refusal}
		}
		return nil
	}
}

// configuredProviderNames is every providers.toml entry name the loaded
// configuration carries: the pool's, and each an entry or store-file fault
// names, so an entry a fault dropped from the pool can still be given a key.
// The store directory's fault names none. A name may repeat.
func configuredProviderNames(opts providerOptions) []string {
	names := slices.Collect(maps.Keys(opts.Providers))
	for _, f := range opts.Faults {
		if f.Source != config.SourceStoreDir {
			names = append(names, f.Name)
		}
	}
	return names
}

// authVerbCommand is one auth verb: its store resolved under the state base,
// its stdin handed in, the configured entry names where readsConfig, and its
// result written as the verb's document.
func authVerbCommand(use, short, long string, args cobra.PositionalArgs, readsConfig bool, verb func(authOptions) (string, []toolresult.Field)) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		Args:  args,
		RunE: func(cmd *cobra.Command, positionals []string) error {
			dir, err := authStoreDir()
			if err != nil {
				return err
			}
			opts := authOptions{Dir: dir, Stdin: cmd.InOrStdin(), Stdout: cmd.OutOrStdout(), Logger: processLog.logger}
			if readsConfig {
				wd, err := invocationDir(cmd)
				if err != nil {
					return err
				}
				loaded, err := loadVerbContext(wd)
				if err != nil {
					return err
				}
				opts.Configured = configuredProviderNames(loaded)
			}
			opts.StdinIsTerminal = isTerminal(opts.Stdin)
			if len(positionals) > 0 {
				opts.Provider = positionals[0]
			}
			code, err := runAuthVerb(cmd.CommandPath(), verb, opts)
			if err != nil {
				return err
			}
			if code != 0 {
				return exitCode(code)
			}
			return nil
		},
	}
}

// authCommand is the parent of the store's verbs; cmd/surface.go adds them.
func authCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "auth",
		Short: "store, remove and list the API keys kbase keeps, one encrypted file per provider",
		Long: `Manage the API keys kbase stores: one file per providers.toml entry, under
$XDG_STATE_HOME/kbase/` + auth.DirName + `/ (~/.local/state/kbase/` + auth.DirName + `/ where the variable is
unset or not an absolute path), each encrypted under an encryption key derived
for your OS user. The encryption is obfuscation, not a boundary: anyone running
as your OS user can recover a key, while a reader of the file sees ciphertext.

A key arrives on stdin alone, piped: never as an argument and never typed at
a prompt. These commands are CLI-only; no MCP tool accepts a key.`,
		Args: authArgs(0, toolresult.Item{Check: toolresult.CheckUsage,
			Detail: "kbase auth takes one of its subcommands — add, remove, list — and no other argument"}),
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
}

func authAddCommand() *cobra.Command {
	return authVerbCommand("add <provider>",
		"store the API key piped on stdin for a providers.toml entry",
		`Store the API key piped on stdin for <provider>, a providers.toml entry
name, replacing any key stored for it; a name no providers.toml entry carries,
spelled exactly as the entry spells it, is refused, and `+auth.NameRule+`.

The key is read from stdin alone, never from an argument. Pipe it
from a program that prints it: a key manager, or your system's clipboard paster
for a key copied there by hand or by a program.

  `+config.AuthAddRemedy(authProviderArg)+`
  <your-system-clipboard-paster> | kbase auth add <provider>

A command that spells the key out, such as echo, puts it on a command line.
Surrounding whitespace is trimmed. Stdin that is a terminal is refused: kbase
never prompts for a key, so none is typed where a scrollback or a transcript
keeps it. Input that is empty or whitespace alone is refused, as are input over
64 KiB and a key with whitespace or a control character inside it. A refusal
writes nothing.

The result names the provider, never the key.`,
		authArgs(1, toolresult.Item{Check: toolresult.CheckUsage, Key: authProviderArg, Remedy: config.AuthAddRemedy(authProviderArg),
			Detail: "kbase auth add takes one argument, the provider name; the key is piped on stdin, never given as an argument"}),
		true, authAdd)
}

func authRemoveCommand() *cobra.Command {
	return authVerbCommand("remove <provider>",
		"clear the API key stored for a providers.toml entry",
		`Remove the API key stored for <provider>, a providers.toml entry name. Where
none is stored the outcome is unchanged and nothing is removed.`,
		authArgs(1, toolresult.Item{Check: toolresult.CheckUsage, Key: authProviderArg, Remedy: config.AuthRemoveCommand(authProviderArg),
			Detail: "kbase auth remove takes one argument, the provider name"}),
		false, authRemove)
}

func authListCommand() *cobra.Command {
	return authVerbCommand("list",
		"name the providers that have a stored API key",
		`Name each provider that has a stored key, in name order: names alone,
never a key. Each name is in lowercase, the form its key is stored under,
whatever case add was given.`,
		authArgs(0, toolresult.Item{Check: toolresult.CheckUsage, Remedy: "kbase auth list", Detail: "kbase auth list takes no argument"}),
		false, authList)
}
