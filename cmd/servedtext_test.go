package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"kbase/internal/config"
	"kbase/internal/mcp/protocol"
)

// TestServedTextNamesNoKeyFile: outside show-config's report, no text kbase
// serves an agent names apiKeyFile — every command's help, every MCP
// resource, the MCP instructions, both templates configure --create writes,
// the refusal naming a provider the pool does not hold, and the two key-file
// fault reasons on a configuration-loading verb's stderr. A failure names the
// text, never its content: the stderr leg reads a configuration holding a key.
func TestServedTextNamesNoKeyFile(t *testing.T) {
	const field = "apiKeyFile"
	served := map[string]string{}

	// The MCP server first: it stages its fixture by a path relative to the
	// package directory, which the invocations below leave.
	srv, _ := mcpServer(t)
	served["the MCP instructions"] = srv.Instructions
	var listed struct {
		Resources []protocol.Resource `json:"resources"`
	}
	serveOne(t, srv, "resources/list", nil, &listed)
	for _, r := range listed.Resources {
		served[r.URI] = readResource(t, srv, r.URI)
	}

	dir := realPath(t, t.TempDir())
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		args := append(strings.Fields(c.CommandPath())[1:], "--help")
		out, code := kbase(t, dir, args...)
		if code != 0 || out == "" {
			t.Fatalf("%s --help: exit %d, %d bytes", c.CommandPath(), code, len(out))
		}
		served[c.CommandPath()+" --help"] = out
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)

	created := filepath.Join(dir, "created")
	if _, code := kbase(t, dir, "configure", "--create", "--config-dir", created); code != 0 {
		t.Fatalf("configure --create: exit %d", code)
	}
	for _, name := range []string{config.ProvidersFileName, config.ConfigFileName} {
		b, err := os.ReadFile(filepath.Join(created, name))
		if err != nil {
			t.Fatal(err)
		}
		served["the "+name+" template"] = string(b)
	}

	// One entry whose key file is missing and one whose key file any user
	// can read: the two fault reasons, as warnFaults writes them to a
	// configuration-loading verb's stderr; and models refusing a name the
	// pool does not hold.
	faulty := filepath.Join(dir, "faulty")
	if err := os.Mkdir(faulty, 0o755); err != nil {
		t.Fatal(err)
	}
	writeValues(t, faulty, config.ProvidersFileName,
		"[gone]\nbaseUrl = \"http://127.0.0.1:1/v1\"\napiKeyFile = \"not-here.key\"\n\n"+
			"[open]\nbaseUrl = \"http://127.0.0.1:1/v1\"\napiKeyFile = \"open.key\"\n")
	if err := os.Chmod(writeValues(t, faulty, "open.key", "sk-served-text-sentinel\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts, err := loadConfigContext([]config.Layer{{Path: faulty, Source: config.LayerSourceExplicit}})
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	warnFaults(&stderr, opts.Faults)
	if !strings.Contains(stderr.String(), "not-here.key") || !strings.Contains(stderr.String(), "0644") {
		t.Fatalf("warnFaults wrote %d bytes, not both key-file fault reasons", stderr.Len())
	}
	served["the key-file fault reasons"] = stderr.String()
	refusal, _ := kbase(t, dir, "models", "--provider", "nosuch", "--config-dir", faulty)
	if !strings.Contains(refusal, "nosuch") || !strings.Contains(refusal, "not found") {
		t.Fatalf("models --provider nosuch: stdout (%d bytes) carries no unresolved-provider refusal", len(refusal))
	}
	served["the unresolved-provider refusal"] = refusal

	for label, text := range served {
		if strings.Contains(text, field) {
			t.Errorf("%s names %s", label, field)
		}
	}
}
