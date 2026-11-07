#### Wire and resources

`internal/mcp/protocol` holds newline-delimited JSON-RPC over stdio, `initialize`, `ping`,
`tools/list` and `tools/call`, the `Tool`, `Schema` and `Property` types, and the frame checks
(`jsonrpc`, `id`, `method`, known tool, object arguments). Arguments failing the schema (missing,
wrong type, a string outside its `enum`, unknown key) are JSON-RPC `-32602` errors, not
refusals; `Schema.Faults` is that check as data, which a family tool runs on its `arguments`. A
fault inside an object argument declaring its properties is reported at its dotted path
(`edge.relation`).
`Serve(in, out, version, instructions, tools, resources, lg)` takes the server version, the
instructions, the tool table and the resources from its caller, and imports `log` only.

`resources/list` and `resources/read` serve the resources `internal/mcp` supplies: the sheets, under
`kbase://sheet/<kb-root-relative path>`; the model guidance at `kbase://model-hints`, which is
`internal/config`'s rows rendered as markdown and needs no KB; and the help of each family
(`kbase://help/<family>`) and of each command exposed as a tool (`kbase://help/<command>`,
`mcp.HelpResources`), composed in `internal/mcp/help.go`, which needs no KB. An unknown URI is
`-32002` and a missing `uri` `-32602`. `initialize` advertises `resources` with no `subscribe`, and
the protocol answers no subscribe method (`TestNoSubscription`).

