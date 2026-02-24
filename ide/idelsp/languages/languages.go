// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package languages

import (
	"errors"
	"path/filepath"
)

// this is for exceptions to the rule of languageID => file_extension[1:]
var extensionToLanguageID = map[string]string{
	// Ada
	".adb": "ada",
	".ads": "ada",

	// Apex
	".cls":     "apex",
	".trigger": "apex",

	// Arduino
	".ino": "arduino",

	// Assembly
	".s": "asm",

	// Bash/Shell
	".sh":           "bash",
	".bash":         "bash",
	".bashrc":       "bash",
	".bash_profile": "bash",
	".profile":      "bash",
	".localrc":      "bash",

	// BibTeX
	".bib": "bibtex",

	// BitBake
	".bb":       "bitbake",
	".bbappend": "bitbake",
	".bbclass":  "bitbake",

	// Blade (Laravel)
	".blade.php": "blade",

	// BPFtrace
	".bt": "bpftrace",

	// BrightScript
	".brs": "brightscript",

	// C
	".c": "c",
	".h": "c",

	// C#
	".cs": "c_sharp",

	// Clojure
	".clj":  "clojure",
	".cljs": "clojure",
	".cljc": "clojure",
	".edn":  "clojure",

	// CMake
	".cmake": "cmake",

	// Comment
	// (no unique extension)

	// Common Lisp
	".lisp": "commonlisp",
	".lsp":  "commonlisp",
	".cl":   "commonlisp",

	// Cooklang
	".cook": "cooklang",

	// C++
	".cpp": "cpp",
	".cxx": "cpp",
	".cc":  "cpp",
	".hpp": "cpp",
	".hxx": "cpp",
	".hh":  "cpp",
	".h++": "cpp",

	// CSS
	".css": "css",

	// CSV
	".csv": "csv",

	// CUDA
	".cu":  "cuda",
	".cuh": "cuda",

	// Device Tree
	".dts":  "devicetree",
	".dtsi": "devicetree",

	// Diff
	".diff":  "diff",
	".patch": "diff",

	// DOT (Graphviz)
	".gv": "dot",

	// Earthfile
	// (no extension, just filename)

	// Elixir
	".ex":  "elixir",
	".exs": "elixir",

	// Elvish
	".elv": "elvish",

	// Embedded Template (ERB/EJS)
	".erb": "embedded_template",
	".ejs": "embedded_template",

	// Enforce
	".enf": "enforce",

	// Erlang
	".erl": "erlang",
	".hrl": "erlang",

	// Facility
	".fsd": "facility",

	// Faust
	".dsp": "faust",

	// Fennel
	".fnl": "fennel",

	// FIRRTL
	".fir": "firrtl",

	// Foam
	// (no unique extension)

	// Forth
	".fth": "forth",
	".4th": "forth",

	// Fortran
	".f":   "fortran",
	".for": "fortran",
	".f90": "fortran",
	".f95": "fortran",
	".f03": "fortran",
	".f08": "fortran",

	// F#
	".fs":  "fsharp",
	".fsi": "fsharp",
	".fsx": "fsharp",

	// FunC
	".fc": "func",

	// GAP
	".g":  "gap",
	".gi": "gap",

	// GAP test
	".tst": "gaptst",

	// GDScript (Godot)
	".gd": "gdscript",

	// GDShader (Godot)
	".gdshader": "gdshader",

	// Git config
	".gitconfig": "git_config",

	// Git attributes
	".gitattributes": "gitattributes",

	// Git ignore
	".gitignore": "gitignore",

	// Glimmer (Handlebars)
	".hbs": "glimmer",

	// Glimmer JS
	".gjs": "glimmer_javascript",

	// Glimmer TS
	".gts": "glimmer_typescript",

	// GLSL
	".glsl": "glsl",
	".vert": "glsl",
	".frag": "glsl",
	".geom": "glsl",
	".comp": "glsl",

	// GN
	".gn":  "gn",
	".gni": "gn",

	// Gnuplot
	".gp": "gnuplot",

	// Goctl
	".api": "goctl",

	// Godot Resource
	".tres": "godot_resource",
	".tscn": "godot_resource",

	// Go template
	".tmpl": "gotmpl",

	// GraphQL
	".gql": "graphql",

	// Groovy
	".gradle": "groovy",

	// Hare
	".ha": "hare",

	// Haskell
	".hs":  "haskell",
	".lhs": "haskell",

	// Haskell Persistent
	".persistentmodels": "haskell_persistent",

	// HCL
	".hcl": "hcl",

	// Helm
	// (uses .yaml with gotmpl)

	// HLS Playlist
	".m3u":  "hlsplaylist",
	".m3u8": "hlsplaylist",

	// HOCON
	".conf": "hocon",

	// HTML
	".htm": "html",

	// HTML Django
	// (uses .html)

	// Hyprlang
	".hl": "hyprlang",

	// Idris
	".idr": "idris",

	// Janet
	".janet": "janet_simple",

	// JavaScript
	".js":  "javascript",
	".mjs": "javascript",
	".cjs": "javascript",

	// Jinja
	".jinja":  "jinja",
	".jinja2": "jinja",
	".j2":     "jinja",

	// JSDoc
	// (embedded in .js)

	// Jsonnet
	".jsonnet":   "jsonnet",
	".libsonnet": "jsonnet",

	// Julia
	".jl": "julia",

	// KCL
	".k": "kcl",

	// Kconfig
	// (filename based)

	// Kitty
	// (filename based)

	// Kotlin
	".kt":  "kotlin",
	".kts": "kotlin",

	// Kusto
	".kql": "kusto",
	".csl": "kusto",

	// LaTeX
	".tex": "latex",
	".ltx": "latex",
	".sty": "latex",

	// Ledger
	".journal": "ledger",

	// Linker Script
	".ld":  "linkerscript",
	".lds": "linkerscript",

	// Liquidsoap
	".liq": "liquidsoap",

	// LLVM IR
	".ll": "llvm",

	// Luadoc
	// (embedded in .lua)

	// Luap
	// (embedded patterns)

	// M68K Assembly
	".m68k": "m68k",

	// Make
	".mk": "make",

	// Markdown
	".md":       "markdown",
	".markdown": "markdown",

	// MATLAB/Octave
	".m": "matlab",

	// Menhir
	".mly": "menhir",

	// Mermaid
	".mmd": "mermaid",

	// Meson
	// (filename based: meson.build)

	// Muttrc
	".neomuttrc": "muttrc",

	// Nginx
	// (filename based)

	// Nickel
	".ncl": "nickel",

	// Nim
	".nim":  "nim",
	".nims": "nim",

	// Nim format string
	// (embedded)

	// Objective-C
	// ".m": "objc", // conflicts with matlab

	// Objdump
	// (no standard extension)

	// OCaml
	".ml":  "ocaml",
	".mli": "ocaml_interface",
	".mll": "ocamllex",

	// Pascal
	".pas": "pascal",
	".pp":  "pascal",
	".inc": "pascal",

	// Passwd
	// (filename based)

	// Perl
	".pl": "perl",
	".pm": "perl",

	// PHPDoc
	// (embedded in .php)

	// PIO Assembly
	".pio": "pioasm",

	// PO (Gettext)
	".po":  "po",
	".pot": "po",

	// Path of Exile filter
	".filter": "poe_filter",

	// PowerShell
	".ps1":  "powershell",
	".psm1": "powershell",
	".psd1": "powershell",

	// Problog/Prolog
	".pro": "prolog",
	// ".pl":  "prolog", // conflicts with perl

	// Pug
	".pug":  "pug",
	".jade": "pug",

	// PureScript
	".purs": "purescript",

	// Python
	".py":  "python",
	".pyw": "python",
	".pyi": "python",

	// QL (CodeQL)
	".ql":  "ql",
	".qll": "ql",

	// QML
	".qml": "qmljs",

	// Query (tree-sitter)
	".scm": "query",

	// R
	".r": "r",
	".R": "r",

	// Racket
	".rkt": "racket",

	// Ralph
	".ral": "ralph",

	// Rasi (Rofi)
	".rasi": "rasi",

	// Razor
	".razor":  "razor",
	".cshtml": "razor",

	// RBS (Ruby type)
	".rbs": "rbs",

	// re2c
	".re": "re2c",

	// Readline
	".inputrc": "readline",

	// Regex
	// (no standard extension)

	// Requirements
	// (filename based)

	// ReScript
	".res":  "rescript",
	".resi": "rescript",

	// Rifleconf
	".rifle": "rifleconf",

	// Rnoweb
	".Rnw": "rnoweb",
	".rnw": "rnoweb",

	// Robots.txt
	// (filename based)

	// Ruby
	".rb":      "ruby",
	".rake":    "ruby",
	".gemspec": "ruby",

	// RuneScript
	".rs2": "runescript",

	// Rust
	".rs": "rust",

	// Scala
	".scala": "scala",
	".sc":    "scala",

	// Scheme
	".ss": "scheme",

	// Sflog (Salesforce)
	".log": "sflog",

	// Snakemake
	".smk": "snakemake",

	// Solidity
	".sol": "solidity",

	// SourcePawn
	".sp": "sourcepawn",

	// SPARQL
	".rq": "sparql",

	// Squirrel
	".nut": "squirrel",

	// SSH config
	".ssh/config": "ssh_config",

	// Starlark
	".star": "starlark",
	".bzl":  "starlark",

	// Strace
	// (no standard extension)

	// Styled
	// (embedded in .js/.ts)

	// SuperCollider
	".scd": "supercollider",

	// Superhtml
	".shtml": "superhtml",

	// Surface (Phoenix)
	".sface": "surface",

	// Sway
	".sw": "sway",

	// sxhkdrc
	// (filename based)

	// SystemTap
	".stp": "systemtap",

	// SystemVerilog
	".sv":  "systemverilog",
	".svh": "systemverilog",

	// T32
	".cmm": "t32",

	// TableGen
	".td": "tablegen",

	// Teal
	".tl": "teal",

	// Terraform
	".tf":     "terraform",
	".tfvars": "terraform",

	// Text Proto
	".pbtxt": "textproto",

	// Tiger
	".tig": "tiger",

	// TLA+
	".tla": "tlaplus",

	// Tmux
	// (filename based)

	// Todotxt
	// (filename based)

	// Turtle (RDF)
	".ttl": "turtle",

	// TypeScript
	".ts":  "typescript",
	".mts": "typescript",
	".cts": "typescript",

	// TypeSpec
	".tsp": "typespec",

	// Typst
	".typ": "typst",

	// udev rules
	".rules": "udev",

	// Ungrammar
	".ungram": "ungrammar",

	// Unison
	".u": "unison",

	// USD (Universal Scene Description)
	".usd":  "usd",
	".usda": "usd",
	".usdc": "usd",

	// Uxntal
	".tal": "uxntal",

	// V
	".v": "v",

	// Vala
	".vapi": "vala",

	// Vento
	".vto": "vento",

	// VHDL
	".vhd":  "vhdl",
	".vhdl": "vhdl",

	// VHS
	".tape": "vhs",

	// Vim
	".vim":   "vim",
	".vimrc": "vim",

	// Vimdoc
	// (no standard extension)

	// WGSL Bevy
	// (uses .wgsl)

	// Wing
	".w": "wing",

	// XCompose
	".XCompose": "xcompose",

	// Xresources
	".Xresources": "xresources",

	// YAML
	".yaml":   "yaml",
	".yml":    "yaml",
	".runerc": "yaml",

	// Ziggy Schema
	".ziggy-schema": "ziggy_schema",

	// Zsh
	".zsh":    "zsh",
	".zshrc":  "zsh",
	".zshenv": "zsh",
}

// special filenames that don't rely on extensions
var filenameToLanguageID = map[string]string{
	"CMakeLists.txt":   "cmake",
	"Caddyfile":        "caddy",
	"Dockerfile":       "dockerfile",
	"Earthfile":        "earthfile",
	"Gemfile":          "ruby",
	"Justfile":         "just",
	"justfile":         "just",
	"Kconfig":          "kconfig",
	"Makefile":         "make",
	"makefile":         "make",
	"GNUmakefile":      "make",
	"meson.build":      "meson",
	"go.mod":           "gomod",
	"go.sum":           "gosum",
	"go.work":          "gowork",
	"BUILD":            "starlark",
	"BUILD.bazel":      "starlark",
	"WORKSPACE":        "starlark",
	"WORKSPACE.bazel":  "starlark",
	"Snakefile":        "snakemake",
	"Rakefile":         "ruby",
	"qmldir":           "qmldir",
	"COMMIT_EDITMSG":   "gitcommit",
	"MANIFEST.in":      "pymanifest",
	"requirements.txt": "requirements",
	"robots.txt":       "robots_txt",
	"todo.txt":         "todotxt",
	".inputrc":         "readline",
	".gitconfig":       "git_config",
	".gitignore":       "gitignore",
	".gitattributes":   "gitattributes",
	".editorconfig":    "editorconfig",
	"nginx.conf":       "nginx",
	"sxhkdrc":          "sxhkdrc",
	".zathurarc":       "zathurarc",
}

// LanguageForFile returns the language id for the given file,
// or an error if it couldn't be determined.
func LanguageForFile(filename string) (string, error) {
	id, ok := filenameToLanguageID[filename]
	if ok {
		return id, nil
	}
	ext := filepath.Ext(filename)
	if ext == "" {
		return "", errors.New("file does not have an extension " +
			"and it's not a recognized file")
	}
	id, ok = extensionToLanguageID[ext]
	if !ok {
		id = ext[1:]
	}
	return id, nil
}
