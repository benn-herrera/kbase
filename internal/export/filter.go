package export

import _ "embed"

// LuaFilter is the export's pandoc Lua filter, written into gen-tex/ as kbase-export.lua.
//
//go:embed kbase-export.lua
var LuaFilter []byte

// LaTeXTemplate is the export's pandoc LaTeX template, written into gen-tex/ as
// kbase-export.latex.
//
//go:embed kbase-export.latex
var LaTeXTemplate []byte

// preambleStarter is the commented preamble.tex WriteOut writes where the
// author has none; it loads nothing.
//
//go:embed preamble.tex
var preambleStarter []byte
