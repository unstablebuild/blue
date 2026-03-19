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

## Builtin Skills

Six skills are available for semantic code navigation via `runectl`.
They use the language server and tree-sitter parser — prefer them over
text-based alternatives when applicable.

- **`code-navigation`** — Jump to definitions, find all references,
  locate declarations, resolve type definitions, find implementations.
  Prefer over Grep when navigating to where a symbol is defined or used.
- **`code-structure`** — List all symbols in a file or search for
  functions, types, and methods across the workspace. Prefer over
  reading an entire file to understand its structure.
- **`code-search`** — Structural search using tree-sitter queries.
  Use for AST patterns (e.g. "all composite literals of type X").
  More precise than regex for structural patterns.
- **`code-understanding`** — Get type info, documentation, and
  function signatures for any symbol without reading source files.
- **`code-diagnostics`** — Get compilation errors and linter
  diagnostics from the language server. Prefer over `go vet`/`go build`.
- **`code-refactoring`** — Rename symbols across the workspace,
  discover code actions, format files. Prefer over find-and-replace
  or `go fmt`.
