## Dependency Policy

Stdlib-first; a new module requires a ruling. Sanctioned:

- `github.com/spf13/cobra` and `github.com/spf13/pflag` — the CLI.
- `github.com/BurntSushi/toml` — configuration files.
- `go.yaml.in/yaml/v3` — YAML; not `gopkg.in/yaml.v3`.
- `golang.org/x/text` — sanctioned only while a package imports it.

Add with `just add-dependency <module>@<version>`, only after the import exists — `go mod tidy`
drops a dependency nothing imports.

---

