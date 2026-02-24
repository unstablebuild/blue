# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Go module that provides utilities and packages for extending the Rune IDE.

## Common Commands

```bash
# Build
make              # Build
make debug        # Build with race detection

# Testing
make test         # Run all tests with race detector (120s timeout)
make coverage     # Generate HTML coverage report

# Code Quality
make lint         # Run golangci-lint (600s timeout)
make format       # Format code with go fmt

# Code Generation & License
make generate     # Regenerate generated files
make license

# Pre-commit validation (run before committing)
make generate && make lint && make test
```

## How to implement a tui.Component or tui.Handler

### Component-Handler-TUI Model

The UI elements of this module are built upon github.com/unstablebuild/rune-go-sdk.
This SDK is built around three core abstractions:

1. **Component** - Basic UI element that can be drawn (via term.Writer) and be resized.
2. **Handler** - Component that can handle events (keyboard, mouse) and manage cursor/selection
3. **Event Loop** - Polls tcell events, routes to handlers, redraws, and flushes to terminal

### Interface flavors

#### component.Responsive and handler.Responsive
Designed to allow collection components (component.List, component.FrameUnion, component.Container, component.ResponsiveList, etc.)
to vertically compose their responsive children, given the collection's width during a call to Resize.
- `Height(width int) int` must return a **hint** — the number
  of lines needed to render content at the given width.
- The component must **not assume** that `Resize` will be called
  with the same height returned by `Height()`.
- If the component can receive less vertical space than it
  needs, it must implement vertical scrolling or truncation.
- Text wrapping must happen at the width boundary (no
  horizontal scrolling for text content).

#### component.Scrollable and handler.Scrollable
Designed to allow the TUI runtime to show a scroll bar next to
the handler/component. It should be implemented by components/handlers
that vertically scroll their content, or collections of Responsive
component/handlers.

#### component.Floating and handler.Floating
Designed to allow components that are inherently capable of calculating
their ideal width and height, because their content size and shape is known
ahead of time via Dimensions, which should not return the width/height passed
in the last Resize, but rather the ideal width and height for this component to
render the entire content. This should be implemented by all components/handlers with
easy to calculate dimensions or when they're collections of Floating components/handlers.

## Rune MCP Tools

When the Rune MCP server is connected (`runectl mcp`), prefer
these tools over the built-in alternatives. They provide
precise and semantically accurate results from Rune's
language servers and tree-sitter parser, which are more
precise than text-based search.

### Code Navigation (prefer over Grep)

Instead of using Grep to find where something is defined or
used, use the suite LSP tools which understand the code semantically:

- **`lsp_definition`** — Jump to where a symbol is defined.
  Use instead of grepping for a function or type name.
- **`lsp_references`** — Find all usages of a symbol across
  the workspace. Use instead of grepping for callers or
  consumers.
- **`lsp_declaration`** — Find the interface or forward
  declaration of a symbol.
- **`lsp_type_definition`** — Find the type behind a value
  (e.g., the struct that a variable holds).
- **`lsp_implementation`** — Find concrete types implementing
  an interface. Use instead of grepping for implementors.

### Code Structure (prefer over Glob + Grep)

Instead of globbing for files and grepping for patterns to
understand code structure, use these tools:

- **`lsp_document_symbols`** — List all functions, types, and
  variables in a single file. Use instead of reading an entire
  file to understand its structure.
- **`lsp_workspace_symbols`** — Search for symbols by name
  across the entire workspace. Use instead of Glob + Grep to
  locate a function or type.
- **`syntax_search_node`** — Find all functions, methods,
  types, or variables workspace-wide. Use when you need
  "all functions in the project"
  without knowing exact names.
- **`syntax_query_node`** — Same as above but scoped to a
  single file.

### Code Searching (prefer over regex Grep)

When you need to find variables, functions, methods, references,
namespaces or types, use rune's search node tools:

- **`syntax_search_node`** — Run a tree-sitter query across all
  workspace files. Use for searching variables, functions, methods
  references, namespaces, or types across the workspace.
- **`syntax_query_node`** — Same as above but scoped to a single
  known file.

When you need to search for a custom node you can provide
your own tree-sitter query to find it use rune's search syntax tools:

- **`syntax_search`** — Run a tree-sitter query across all
  workspace files. Use for structural patterns like "all
  function calls with two arguments" or "all composite literals
  of type X". More precise than regex.
- **`syntax_query`** — Same as above but scoped to a single
  known file.

### Understanding Code (prefer over reading whole files)

- **`lsp_hover`** — Get type info and documentation for any
  symbol at a position. Use instead of reading source to
  understand what a symbol is.
- **`lsp_signature_help`** — Get function parameter names and
  types. Use when you need to know a function's signature
  without reading its definition.
- **`lsp_completion`** — Discover available methods and fields
  on a type at a cursor position.

### Error Checking (prefer over go build / go vet)

- **`lsp_workspace_diagnostics`** — Get compilation errors, warnings,
  and linter diagnostics for the whole workspace. Use instead of running
  `go build` or `go vet` to check for errors.

- **`lsp_diagnostics`** — Get compilation errors, warnings,
  and linter diagnostics for a file. Use instead of running
  `go build` or `go vet` to check for errors.

### Refactoring (prefer over manual find-and-replace)

- **`lsp_rename`** — Safely rename a symbol across the entire
  workspace, updating all references. Use instead of
  Grep + Edit for renaming.
- **`lsp_prepare_rename`** — Check if a rename is valid before
  performing it.
- **`lsp_code_actions`** — Discover available refactorings and
  quick fixes at a position (extract variable, organize
  imports, etc.).
- **`lsp_formatting`** — Format an entire file using the
  language server. Use instead of running `go fmt`.
- **`lsp_range_formatting`** — Format a specific range within
  a file.
