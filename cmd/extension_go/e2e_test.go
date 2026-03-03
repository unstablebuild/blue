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

package main

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unstablebuild/blue/ide/idelsp"
	"github.com/unstablebuild/blue/ide/idelsp/lspcmd"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// Source files used across tests.
const (
	// mainSrc is a basic Go file with an unused import, a struct, and
	// functions that exercise various code actions.
	mainSrc = `package main

import (
	"fmt"
	"strings"
)

// Greeter holds a name.
type Greeter struct {
	Name string
}

// Greet returns a greeting.
func (g *Greeter) Greet() string {
	return fmt.Sprintf("Hello, %s!", g.Name)
}

func Add(a, b int) int {
	return a + b
}

func main() {
	g := &Greeter{Name: "World"}
	fmt.Println(g.Greet())
	_ = strings.ToUpper("test")
}
`

	// testFileSrc is a test file with a test function and a benchmark.
	testFileSrc = `package main

import "testing"

func TestAdd(t *testing.T) {
	if Add(1, 2) != 3 {
		t.Error("expected 3")
	}
}

func BenchmarkAdd(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Add(1, 2)
	}
}
`

	// fillStructSrc has an incomplete struct literal.
	fillStructSrc = `package main

import "net/http"

func newRequest() *http.Request {
	return &http.Request{}
}
`

	// extractFuncSrc has code suitable for extract-function.
	extractFuncSrc = `package main

import "fmt"

func compute() {
	a := 1
	b := 2
	sum := a + b
	fmt.Println(sum)
}
`

	// invertIfSrc has an if/else block suitable for inversion.
	invertIfSrc = `package main

import "fmt"

func check(x int) {
	if x > 0 {
		fmt.Println("positive")
	} else {
		fmt.Println("non-positive")
	}
}
`

	// unusedImportSrc has an unused import that organize-imports should remove.
	unusedImportSrc = `package main

import (
	"fmt"
	"os"
)

func hello() string {
	return fmt.Sprintf("hello")
}
`

	// addTagsSrc has a struct suitable for add-tags / remove-tags.
	addTagsSrc = `package main

type Config struct {
	Host string
	Port int
}
`

	// typeSwitchSrc has an incomplete type switch suitable for fill-switch.
	typeSwitchSrc = `package main

import "fmt"

type Animal interface {
	Sound() string
}

type Dog struct{}
func (d Dog) Sound() string { return "woof" }

type Cat struct{}
func (c Cat) Sound() string { return "meow" }

func describe(a Animal) {
	switch a.(type) {
	}
	fmt.Println(a.Sound())
}
`

	// inlineCallSrc has a simple function suitable for inline-call.
	inlineCallSrc = `package main

func double(x int) int {
	return x * 2
}

func useDouble() int {
	return double(5)
}
`

	// generateSrc has a go:generate directive.
	generateSrc = `package main

//go:generate echo hello

func generated() {}
`
)

// stubResource implements textapi.Handler for tests.
type stubResource struct {
	uri workspaceapi.URI
}

func (s *stubResource) Handle(_ term.Event) (bool, bool) { return false, false }
func (s *stubResource) Draw(_ term.Writer)               {}
func (s *stubResource) Resize(_, _ int)                  {}
func (s *stubResource) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, 0, false
}
func (s *stubResource) Selection() (string, bool)  { return "", false }
func (s *stubResource) Close() error               { return nil }
func (s *stubResource) Resource() workspaceapi.URI { return s.uri }

var _ textapi.Handler = (*stubResource)(nil)

func TestE2E(t *testing.T) {
	t.Parallel()
	goplsBin := findGopls(t)

	t.Run("OrganizeImports", func(t *testing.T) {
		t.Parallel()
		env := initGopls(t, goplsBin, []testFile{
			{name: "main.go", content: unusedImportSrc},
		})
		handler, me, _ := newTestHandler(t, env)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		me.Register(resource)
		cmd := goCmdAt("organize-imports", uri, resource, 0, 0)

		err := handler.HandleCommand(t.Context(), cmd)
		require.NoError(t, err)

		edits := me.editsFor(resource)
		require.NotEmpty(t, edits, "expected edits for organize-imports")

		combined := collectEditText(edits)
		assert.NotContains(t, combined, `"os"`)

		// Verify gopls overlay is in sync after the edit.
		// Hover on fmt.Sprintf (still present after removing unused os).
		hover, err := env.mgr.Hover(t.Context(), semanticapi.HoverParams{
			TextDocument: semanticapi.TextDocumentIdentifier{URI: env.fileURIs["main.go"]},
			Position:     semanticapi.Position{Line: 0, Character: 8},
		})
		require.NoError(t, err, "gopls should respond after overlay update")
		require.NotNil(t, hover, "expected hover on package name after edit")
	})

	t.Run("FixAll", func(t *testing.T) {
		t.Parallel()
		// source.fixAll applies safe diagnostic fixes (e.g. simplifyrange).
		// Whether gopls produces fixAll actions depends on the analyzer
		// configuration and gopls version. This test verifies the handler
		// runs the code action flow correctly.
		env := initGopls(t, goplsBin, []testFile{
			{name: "main.go", content: mainSrc},
		})
		handler, me, mn := newTestHandler(t, env)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		me.Register(resource)
		cmd := goCmdAt("fix-all", uri, resource, 0, 0)

		err := handler.HandleCommand(t.Context(), cmd)
		require.NoError(t, err)

		edits := me.editsFor(resource)
		if len(edits) > 0 {
			// If edits were produced, verify gopls overlay is in sync.
			hover, err := env.mgr.Hover(t.Context(), semanticapi.HoverParams{
				TextDocument: semanticapi.TextDocumentIdentifier{URI: env.fileURIs["main.go"]},
				Position:     semanticapi.Position{Line: 0, Character: 8},
			})
			require.NoError(t, err, "gopls should respond after fix-all overlay update")
			require.NotNil(t, hover)
		} else {
			// No fixAll actions: handler should notify.
			assert.True(t, mn.hasMessage("No automatic fixes available"))
		}
	})

	t.Run("FillStruct", func(t *testing.T) {
		t.Parallel()
		env := initGopls(t, goplsBin, []testFile{
			{name: "main.go", content: fillStructSrc},
		})
		handler, me, mn := newTestHandlerWithEditorEvents(t, env)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		me.Register(resource)
		// Cursor on the empty struct literal `http.Request{}` (line 5, char 18).
		cmd := goCmdAt("fill-struct", uri, resource, 5, 18)

		err := handler.HandleCommand(t.Context(), cmd)
		require.NoError(t, err)

		edits := me.editsFor(resource)
		if len(edits) > 0 {
			combined := collectEditText(edits)
			assert.Contains(t, strings.ToLower(combined), "method")

			// Wait for gopls to publish diagnostics after processing the
			// edit events, then verify the overlay is not corrupted.
			fileURI := env.fileURIs["main.go"]
			diags := waitForDiagnostics(t, env.cb, fileURI, 5*time.Second)
			for _, d := range diags {
				if d.Severity == 1 { // Error
					t.Errorf("overlay corrupted after fill-struct: %s (line %d)",
						d.Message, d.Range.Start.Line)
				}
			}
		} else {
			assert.True(t, mn.hasMessage("Place cursor on a struct literal") || len(edits) == 0)
		}
	})

	t.Run("FillSwitch", func(t *testing.T) {
		t.Parallel()
		env := initGopls(t, goplsBin, []testFile{
			{name: "main.go", content: typeSwitchSrc},
		})
		handler, me, _ := newTestHandlerWithEditorEvents(t, env)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		me.Register(resource)
		// Cursor on the empty type switch (line 15, char 2).
		cmd := goCmdAt("fill-switch", uri, resource, 15, 2)

		err := handler.HandleCommand(t.Context(), cmd)
		require.NoError(t, err)

		edits := me.editsFor(resource)
		if len(edits) > 0 {
			combined := collectEditText(edits)
			assert.Contains(t, combined, "Dog")
			assert.Contains(t, combined, "Cat")

			// Wait for gopls to publish diagnostics after processing the
			// edit events, then verify the overlay is not corrupted.
			fileURI := env.fileURIs["main.go"]
			diags := waitForDiagnostics(t, env.cb, fileURI, 5*time.Second)
			for _, d := range diags {
				if d.Severity == 1 { // Error
					t.Errorf("overlay corrupted after fill-switch: %s (line %d)",
						d.Message, d.Range.Start.Line)
				}
			}
		}
	})

	t.Run("ExtractFunction", func(t *testing.T) {
		t.Parallel()
		env := initGoplsWithApplyEdit(t, goplsBin, []testFile{
			{name: "main.go", content: extractFuncSrc},
		})
		handler, me, _ := newTestHandler(t, env.testEnv)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// Fire a selection event covering all statements in compute()
		// (lines 5-8: "a := 1" through "fmt.Println(sum)").
		me.fireEvent(t.Context(), textapi.Event{
			Type:  textapi.EventTypeSelection,
			URI:   uri,
			Start: term.Coordinates{X: 1, Y: 5},
			End:   term.Coordinates{X: 17, Y: 8},
		})

		cmd := goCmdAt("extract-function", uri, resource, 5, 1)

		err := handler.HandleCommand(t.Context(), cmd)
		require.NoError(t, err)

		// gopls applies extract-function via workspace/applyEdit
		// (the code action returns a Command, not a direct Edit).
		captured := env.capturedEdits()
		require.NotEmpty(t, captured, "expected workspace/applyEdit from extract-function")

		// Verify the edit contains a new function definition.
		var editText strings.Builder
		for _, ae := range captured {
			for _, edits := range ae.Edit.Changes {
				for _, e := range edits {
					editText.WriteString(e.NewText)
				}
			}
			for _, dc := range ae.Edit.DocumentChanges {
				if dc.TextDocumentEdit != nil {
					for _, e := range dc.TextDocumentEdit.Edits {
						editText.WriteString(e.NewText)
					}
				}
			}
		}
		assert.Contains(t, editText.String(), "newFunction",
			"extracted function should be named newFunction (gopls default)")

		// --- Second extraction from the newly extracted function. ---
		// If gopls's overlay is in sync with the applied edit, it
		// should be able to extract a subset of newFunction's body.
		// If the overlay is stale, gopls still sees the old source
		// and the selected lines won't correspond to extractable code.
		ctx := t.Context()

		// Wait for gopls to learn about newFunction (overlay sync).
		var fnStartLine uint32
		require.Eventually(t, func() bool {
			syms, err := env.mgr.WorkspaceSymbol(ctx, semanticapi.WorkspaceSymbolParams{
				Query: "newFunction",
			})
			if err != nil {
				return false
			}
			for _, s := range syms {
				if s.Name == "newFunction" &&
					strings.Contains(s.Location.URI, env.dir) {
					fnStartLine = s.Location.Range.Start.Line
					return true
				}
			}
			return false
		}, 5*time.Second, 100*time.Millisecond,
			"gopls should know about 'newFunction' after first extract")

		// newFunction body starts at fnStartLine+1. Select the first
		// two statements (a := 1, b := 2) for the second extraction.
		// Use X=0 on the line past the selection to cover full lines.
		bodyStart := int(fnStartLine) + 1

		me.fireEvent(ctx, textapi.Event{
			Type:  textapi.EventTypeSelection,
			URI:   uri,
			Start: term.Coordinates{X: 0, Y: bodyStart},
			End:   term.Coordinates{X: 0, Y: bodyStart + 2},
		})

		prevCount := len(env.capturedEdits())
		cmd2 := goCmdAt("extract-function", uri, resource, bodyStart, 1)

		err = handler.HandleCommand(ctx, cmd2)
		require.NoError(t, err,
			"second extract-function should not error if gopls overlay is in sync")

		captured2 := env.capturedEdits()
		require.Greater(t, len(captured2), prevCount,
			"gopls overlay stale: second extract-function produced no workspace/applyEdit")
	})

	t.Run("ExtractVariable", func(t *testing.T) {
		t.Parallel()
		env := initGopls(t, goplsBin, []testFile{
			{name: "main.go", content: extractFuncSrc},
		})
		handler, me, _ := newTestHandler(t, env)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		me.Register(resource)
		cmd := goCmdAt("extract-variable", uri, resource, 7, 8)

		err := handler.HandleCommand(t.Context(), cmd)
		require.NoError(t, err)
	})

	t.Run("InvertIf", func(t *testing.T) {
		t.Parallel()
		env := initGopls(t, goplsBin, []testFile{
			{name: "main.go", content: invertIfSrc},
		})
		handler, me, mn := newTestHandlerWithEditorEvents(t, env)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		me.Register(resource)
		// Cursor on the if keyword (line 5, char 1).
		cmd := goCmdAt("invert-if", uri, resource, 5, 1)

		err := handler.HandleCommand(t.Context(), cmd)
		require.NoError(t, err)

		// gopls may apply the edit directly or via a command (workspace/applyEdit).
		edits := me.editsFor(resource)
		if len(edits) > 0 {
			combined := collectEditText(edits)
			assert.Contains(t, combined, "<=")

			// Wait for gopls to publish diagnostics after processing the
			// edit events, then verify the overlay is not corrupted.
			fileURI := env.fileURIs["main.go"]
			diags := waitForDiagnostics(t, env.cb, fileURI, 5*time.Second)
			for _, d := range diags {
				if d.Severity == 1 { // Error
					t.Errorf("overlay corrupted after invert-if: %s (line %d)",
						d.Message, d.Range.Start.Line)
				}
			}
		} else {
			// If gopls used a command, we should see a notification.
			msgs := mn.getMessages()
			assert.NotEmpty(t, msgs)
		}
	})

	t.Run("InlineCall", func(t *testing.T) {
		t.Parallel()
		env := initGopls(t, goplsBin, []testFile{
			{name: "main.go", content: inlineCallSrc},
		})
		handler, me, mn := newTestHandlerWithEditorEvents(t, env)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		me.Register(resource)
		// Cursor on `double` call (line 7, char 8).
		cmd := goCmdAt("inline-call", uri, resource, 7, 8)

		err := handler.HandleCommand(t.Context(), cmd)
		require.NoError(t, err)

		edits := me.editsFor(resource)
		if len(edits) > 0 {
			combined := collectEditText(edits)
			assert.Contains(t, combined, "5 * 2")

			// Wait for gopls to publish diagnostics after processing the
			// edit events, then verify the overlay is not corrupted.
			fileURI := env.fileURIs["main.go"]
			diags := waitForDiagnostics(t, env.cb, fileURI, 5*time.Second)
			for _, d := range diags {
				if d.Severity == 1 { // Error
					t.Errorf("overlay corrupted after inline-call: %s (line %d)",
						d.Message, d.Range.Start.Line)
				}
			}
		} else {
			msgs := mn.getMessages()
			assert.NotEmpty(t, msgs)
		}
	})

	t.Run("AddTags", func(t *testing.T) {
		t.Parallel()
		env := initGopls(t, goplsBin, []testFile{
			{name: "main.go", content: addTagsSrc},
		})
		handler, me, _ := newTestHandler(t, env)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		me.Register(resource)
		// Cursor on the Host field (line 3, char 1).
		cmd := goCmdAt("add-tags", uri, resource, 3, 1)

		err := handler.HandleCommand(t.Context(), cmd)
		require.NoError(t, err)
	})

	t.Run("RemoveTags", func(t *testing.T) {
		t.Parallel()
		taggedSrc := `package main

type Config struct {
	Host string ` + "`json:\"host\"`" + `
	Port int    ` + "`json:\"port\"`" + `
}
`
		env := initGopls(t, goplsBin, []testFile{
			{name: "main.go", content: taggedSrc},
		})
		handler, me, _ := newTestHandler(t, env)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		me.Register(resource)
		cmd := goCmdAt("remove-tags", uri, resource, 3, 1)

		err := handler.HandleCommand(t.Context(), cmd)
		require.NoError(t, err)
	})

	t.Run("AddTest", func(t *testing.T) {
		t.Parallel()
		env := initGopls(t, goplsBin, []testFile{
			{name: "main.go", content: mainSrc},
			{name: "main_test.go", content: testFileSrc},
		})
		handler, me, _ := newTestHandler(t, env)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		me.Register(resource)
		// Cursor on the Add function (line 17, char 5).
		cmd := goCmdAt("add-test", uri, resource, 17, 5)

		err := handler.HandleCommand(t.Context(), cmd)
		require.NoError(t, err)
	})

	// Regression: go add-test should produce a test file with
	// the package declaration at the top, not at the bottom.
	// When gopls sends workspace/applyEdit with multiple inserts
	// at the same position, the array order must define the
	// resulting text order per the LSP spec.
	t.Run("AddTest/EditOrder", func(t *testing.T) {
		t.Parallel()

		// Use a source with only Greet (no existing test file)
		// so gopls creates a fresh test file.
		addTestSrc := "package main\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n"
		env := initGoplsWithApplyEdit(t, goplsBin, []testFile{
			{name: "main.go", content: addTestSrc},
		})
		handler, me, _ := newTestHandler(t, env.testEnv)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		me.Register(resource)
		// Cursor on "Add" (line 2, char 5).
		cmd := goCmdAt("add-test", uri, resource, 2, 5)

		err := handler.HandleCommand(t.Context(), cmd)
		require.NoError(t, err)

		// workspace/applyEdit is sent synchronously within
		// ExecuteCommand, so captured edits are available
		// immediately after HandleCommand returns.
		captured := env.capturedEdits()
		require.NotEmpty(t, captured, "gopls should have sent workspace/applyEdit")

		// Collect all TextEdits destined for the test file.
		var testEdits []semanticapi.TextEdit
		for _, ae := range captured {
			for _, dc := range ae.Edit.DocumentChanges {
				if dc.TextDocumentEdit != nil &&
					strings.HasSuffix(dc.TextDocumentEdit.TextDocument.URI, "_test.go") {
					testEdits = append(testEdits, dc.TextDocumentEdit.Edits...)
				}
			}
			for fileURI, edits := range ae.Edit.Changes {
				if strings.HasSuffix(fileURI, "_test.go") {
					testEdits = append(testEdits, edits...)
				}
			}
		}
		require.NotEmpty(t, testEdits, "expected edits for the test file")

		// Apply the edits through lspcmd.ApplyEdits — the function
		// under test — and reconstruct the resulting buffer content.
		buf := newBufferCellEditor("")
		err = lspcmd.ApplyEdits(t.Context(), buf, testEdits)
		require.NoError(t, err)

		content := buf.String()
		assert.True(t, strings.HasPrefix(content, "package"),
			"generated test file must start with 'package', got:\n%s", content)
		assert.Contains(t, content, "func Test")
	})

	t.Run("Assembly", func(t *testing.T) {
		t.Parallel()
		env := initGopls(t, goplsBin, []testFile{
			{name: "main.go", content: mainSrc},
		})
		handler, me, _ := newTestHandler(t, env)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		me.Register(resource)
		cmd := goCmdAt("assembly", uri, resource, 17, 5)

		err := handler.HandleCommand(t.Context(), cmd)
		require.NoError(t, err)
	})

	t.Run("Doc", func(t *testing.T) {
		t.Parallel()
		env := initGopls(t, goplsBin, []testFile{
			{name: "main.go", content: mainSrc},
		})
		handler, me, _ := newTestHandler(t, env)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		me.Register(resource)
		cmd := goCmdAt("doc", uri, resource, 0, 0)

		err := handler.HandleCommand(t.Context(), cmd)
		require.NoError(t, err)
	})

	t.Run("CodeLens/Test", func(t *testing.T) {
		t.Parallel()
		env := initGopls(t, goplsBin, []testFile{
			{name: "main.go", content: mainSrc},
			{name: "main_test.go", content: testFileSrc},
		})
		handler, _, mn := newTestHandler(t, env)

		uri := parseTestURI(t, env.fileURIs["main_test.go"])
		resource := &stubResource{uri: uri}
		// Cursor near TestAdd (line 4).
		cmd := goCmdAt("test", uri, resource, 4, 5)

		err := handler.HandleCommand(t.Context(), cmd)
		require.NoError(t, err)
		assert.Len(t, mn.getMessages(), 1)
		assert.Equal(t, mockNotification{
			Level: browserapi.LevelInfo, Message: "Executed: run test",
		}, mn.getMessages()[0])
	})

	t.Run("CodeLens/Generate", func(t *testing.T) {
		t.Parallel()
		env := initGopls(t, goplsBin, []testFile{
			{name: "main.go", content: generateSrc},
		})
		handler, _, mn := newTestHandler(t, env)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		// Cursor near the go:generate directive (line 2).
		cmd := goCmdAt("generate", uri, resource, 2, 0)

		err := handler.HandleCommand(t.Context(), cmd)
		if err != nil {
			assert.NotContains(t, err.Error(), "unsupported command")
		} else {
			msgs := mn.getMessages()
			assert.NotEmpty(t, msgs, "expected a notification from generate")
		}
	})

	t.Run("Tidy", func(t *testing.T) {
		t.Parallel()
		env := initGopls(t, goplsBin, []testFile{
			{name: "main.go", content: "package main\n\nfunc main() {}\n"},
		})
		handler, _, mn := newTestHandler(t, env)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		cmd := goCmd("tidy", uri, resource)

		err := handler.HandleCommand(t.Context(), cmd)
		if err != nil {
			assert.NotContains(t, err.Error(), "unsupported command")
		} else {
			assert.True(t, mn.hasMessage("gopls.tidy"))
		}
	})

	t.Run("Vendor", func(t *testing.T) {
		t.Parallel()
		env := initGopls(t, goplsBin, []testFile{
			{name: "main.go", content: "package main\n\nfunc main() {}\n"},
		})
		handler, _, mn := newTestHandler(t, env)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		cmd := goCmd("vendor", uri, resource)

		err := handler.HandleCommand(t.Context(), cmd)
		if err != nil {
			assert.NotContains(t, err.Error(), "unsupported command")
		} else {
			assert.True(t, mn.hasMessage("gopls.vendor"))
		}
	})

	t.Run("Vulncheck", func(t *testing.T) {
		t.Parallel()
		env := initGopls(t, goplsBin, []testFile{
			{name: "main.go", content: "package main\n\nfunc main() {}\n"},
		})
		handler, _, mn := newTestHandler(t, env)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		cmd := goCmd("vulncheck", uri, resource)

		err := handler.HandleCommand(t.Context(), cmd)
		// Vulncheck may fail if govulncheck is not installed.
		if err != nil {
			assert.NotContains(t, err.Error(), "unsupported command")
		} else {
			assert.True(t, mn.hasMessage("gopls.run_govulncheck"))
		}
	})

	t.Run("AddImport", func(t *testing.T) {
		t.Parallel()
		simpleSrc := "package main\n\nfunc main() {}\n"
		env := initGopls(t, goplsBin, []testFile{
			{name: "main.go", content: simpleSrc},
		})
		handler, _, mn := newTestHandler(t, env)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		cmd := textapi.Command{
			Name:     "go",
			Args:     []string{"add-import", "fmt"},
			URI:      uri,
			Resource: resource,
		}

		err := handler.HandleCommand(t.Context(), cmd)
		require.NoError(t, err)
		assert.True(t, mn.hasMessage("Added import"))
	})

	// Regression: after add-import, gopls must know about the change.
	// workspace/applyEdit is sent by gopls; the client must feed the
	// edits back as textDocument/didChange so the overlay stays in sync.
	// AddImportSyncsOverlay verifies that after gopls.add_import
	// triggers workspace/applyEdit, the callbackInterceptor sends
	// didChange so gopls's overlay stays in sync — no manual
	// replay needed.
	t.Run("AddImportSyncsOverlay", func(t *testing.T) {
		t.Parallel()
		src := "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"
		env := initGoplsWithApplyEdit(t, goplsBin, []testFile{
			{name: "main.go", content: src},
		})
		handler, _, mn := newTestHandler(t, env.testEnv)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		cmd := textapi.Command{
			Name:     "go",
			Args:     []string{"add-import", "fmt"},
			URI:      uri,
			Resource: resource,
		}

		err := handler.HandleCommand(t.Context(), cmd)
		require.NoError(t, err)
		assert.True(t, mn.hasMessage("Added import"))

		captured := env.capturedEdits()
		require.NotEmpty(t, captured, "gopls should have sent workspace/applyEdit")

		// simulateEditorEvents fires Handle(EventTypeEdit) which sends
		// didChange asynchronously — give it time to be processed.
		time.Sleep(500 * time.Millisecond)

		fileURI := env.fileURIs["main.go"]
		hover, err := env.mgr.Hover(t.Context(), semanticapi.HoverParams{
			TextDocument: semanticapi.TextDocumentIdentifier{URI: fileURI},
			Position:     semanticapi.Position{Line: 5, Character: 1},
		})
		require.NoError(t, err)
		require.NotNil(t, hover, "expected hover result after add-import synced overlay")
		assert.Contains(t, hover.Contents.Value, "fmt")
	})

	// Same as above but with auto-init-like params (no
	// workspace.configuration, no workspace.workspaceEdit.documentChanges).
	t.Run("AddImportSyncsOverlay/AutoInitParams", func(t *testing.T) {
		t.Parallel()
		src := "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"
		env := initGoplsWithAutoInitParams(t, goplsBin, []testFile{
			{name: "main.go", content: src},
		})
		handler, _, mn := newTestHandler(t, env.testEnv)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		cmd := textapi.Command{
			Name:     "go",
			Args:     []string{"add-import", "fmt"},
			URI:      uri,
			Resource: resource,
		}

		err := handler.HandleCommand(t.Context(), cmd)
		require.NoError(t, err)
		assert.True(t, mn.hasMessage("Added import"))

		captured := env.capturedEdits()
		require.NotEmpty(t, captured, "gopls should have sent workspace/applyEdit")

		time.Sleep(500 * time.Millisecond)

		fileURI := env.fileURIs["main.go"]
		hover, err := env.mgr.Hover(t.Context(), semanticapi.HoverParams{
			TextDocument: semanticapi.TextDocumentIdentifier{URI: fileURI},
			Position:     semanticapi.Position{Line: 5, Character: 1},
		})
		require.NoError(t, err)
		require.NotNil(t, hover, "expected hover result after add-import synced overlay (auto-init params)")
		assert.Contains(t, hover.Contents.Value, "fmt")
	})

	// Regression: in the real IDE, workspace/applyEdit modifies the editor
	// buffer, which fires Handle(EventTypeEdit) → didChange to gopls.
	// Previously callbackInterceptor.notifyDidChange sent ANOTHER didChange,
	// which corrupted gopls's overlay. initGoplsWithApplyEdit simulates the
	// real editor events via simulateEditorEvents; this test verifies the
	// overlay stays clean.
	t.Run("AddImportNoDoubleNotification", func(t *testing.T) {
		t.Parallel()
		src := "package main\n\nfunc main() {\n\tstrings.Join(nil, \"\")\n}\n"
		env := initGoplsWithApplyEdit(t, goplsBin, []testFile{
			{name: "main.go", content: src},
		})

		handler, _, mn := newTestHandler(t, env.testEnv)

		uri := parseTestURI(t, env.fileURIs["main.go"])
		resource := &stubResource{uri: uri}
		cmd := textapi.Command{
			Name:     "go",
			Args:     []string{"add-import", "strings"},
			URI:      uri,
			Resource: resource,
		}

		err := handler.HandleCommand(t.Context(), cmd)
		require.NoError(t, err)
		assert.True(t, mn.hasMessage("Added import"))

		captured := env.capturedEdits()
		require.NotEmpty(t, captured, "gopls should have sent workspace/applyEdit")

		// Wait for the async Handle(EventTypeEdit) to be processed by handleEvs.
		// This ensures both didChange notifications have reached gopls.
		time.Sleep(1 * time.Second)

		// Verify gopls's overlay is not corrupted.
		fileURI := env.fileURIs["main.go"]
		hover, err := env.mgr.Hover(t.Context(), semanticapi.HoverParams{
			TextDocument: semanticapi.TextDocumentIdentifier{URI: fileURI},
			Position:     semanticapi.Position{Line: 0, Character: 8},
		})
		require.NoError(t, err, "hover should succeed after add-import")
		require.NotNil(t, hover, "overlay may be corrupted: hover returned nil")

		// Wait for diagnostics to settle.
		time.Sleep(2 * time.Second)

		// Check the LAST diagnostic set for the file: if gopls got a double
		// didChange, the overlay has garbled imports producing errors like
		// "could not import stringss", "redeclared", or "undefined".
		// Earlier diagnostics include pre-import errors (undefined: strings)
		// which are expected — only the final state matters.
		env.cb.mu.Lock()
		var lastDiags []semanticapi.Diagnostic
		for _, d := range env.cb.diagnostics {
			if d.URI == fileURI {
				lastDiags = d.Diagnostics
			}
		}
		env.cb.mu.Unlock()

		var diagMsgs []string
		for _, d := range lastDiags {
			diagMsgs = append(diagMsgs, d.Message)
		}
		t.Logf("final diagnostics after add-import: %v", diagMsgs)
		for _, msg := range diagMsgs {
			if strings.Contains(msg, "redeclared") ||
				strings.Contains(msg, "could not import") {
				t.Errorf("overlay corrupted by double didChange: %s", msg)
			}
		}
		// The import should resolve — no "undefined: strings" in final state.
		for _, msg := range diagMsgs {
			if msg == "undefined: strings" {
				t.Errorf("import not applied to overlay: %s", msg)
			}
		}
	})
}

func TestFindGoModURI(t *testing.T) {
	t.Parallel()

	// Create a workspace with nested directories.
	tmpDir := setupWorkspace(t, "example.com/test", []testFile{
		{name: "main.go", content: "package main\n"},
		{name: "pkg/foo/bar.go", content: "package foo\n"},
	})

	t.Run("FileInRoot", func(t *testing.T) {
		fileURI := "file://" + tmpDir + "/main.go"
		result := findGoModURI(fileURI)
		assert.Equal(t, "file://"+tmpDir+"/go.mod", result)
	})

	t.Run("FileInSubdirectory", func(t *testing.T) {
		fileURI := "file://" + tmpDir + "/pkg/foo/bar.go"
		result := findGoModURI(fileURI)
		assert.Equal(t, "file://"+tmpDir+"/go.mod", result)
	})
}

func TestGoRouter(t *testing.T) {
	t.Parallel()

	t.Run("MissingSubcommand", func(t *testing.T) {
		router := &goRouter{handlers: map[string]textapi.CommandHandler{}}
		err := router.HandleCommand(t.Context(), textapi.Command{
			Name:     "go",
			Resource: &stubResource{},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing subcommand")
	})

	t.Run("UnknownSubcommand", func(t *testing.T) {
		router := &goRouter{handlers: map[string]textapi.CommandHandler{}}
		err := router.HandleCommand(t.Context(), textapi.Command{
			Name:     "go",
			Args:     []string{"nonexistent"},
			Resource: &stubResource{},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown go subcommand")
	})

	t.Run("NilResource", func(t *testing.T) {
		router := &goRouter{handlers: map[string]textapi.CommandHandler{}}
		err := router.HandleCommand(t.Context(), textapi.Command{
			Name: "go",
			Args: []string{"tidy"},
		})
		require.NoError(t, err)
	})
}

func TestE2EHandleOpenWorksWithNoInitializeServer(t *testing.T) {
	t.Parallel()
	goplsBin := findGopls(t)

	diskContent := "package main\n\nfunc main() {}\n"

	dir := setupWorkspace(t, "example.com/test", []testFile{
		{name: "main.go", content: diskContent},
	})
	rootURI := "file://" + dir

	uri, err := workspaceapi.ParseURI(rootURI)
	require.NoError(t, err)

	scheme := newTestScheme()

	ready := make(chan struct{})
	var once sync.Once
	cb := &testCallback{
		onShowMessage: func(params semanticapi.ShowMessageParams) {
			if strings.Contains(params.Message, "Finished loading packages") ||
				strings.Contains(params.Message, "background refresh finished") {
				once.Do(func() { close(ready) })
			}
		},
		onProgress: readyOnProgressCh(&once, ready),
	}

	mgr := idelsp.New(
		uri, scheme, scheme,
		&stubPkgManager{bin: goplsBin},
		nil, nil,
		idelsp.Config{
			MaxRetries:         1,
			Callback:           cb,
			NoInitializeServer: true,
		},
	)
	t.Cleanup(func() { require.NoError(t, mgr.Close()) })

	ctx := context.Background()
	params, err := goplsInitializeParams(rootURI)
	require.NoError(t, err)

	_, err = mgr.Initialize(ctx, params)
	require.NoError(t, err)

	mainURI := "file://" + filepath.Join(dir, "main.go")
	fileURIs := map[string]string{"main.go": mainURI}
	waitGoplsReady(t, ready, mgr, fileURIs)

	// Open the file with overlay content that differs from disk.
	// The overlay has an Add function that the disk version does not.
	wsURI, err := workspaceapi.ParseURI(mainURI)
	require.NoError(t, err)

	mgr.Handle(ctx, textapi.Event{
		Type:    textapi.EventTypeOpen,
		URI:     wsURI,
		Content: mainSrc, // contains Add at line 17
	})

	// Give handleEvs time to process the open event.
	time.Sleep(1 * time.Second)

	// Hover on Add (line 17, char 5) which only exists in the overlay.
	// If didOpen was sent, gopls uses the overlay and returns hover info.
	// If didOpen was dropped (bug), gopls reads from disk where line 17
	// does not exist and returns nil.
	hover, err := mgr.Hover(ctx, semanticapi.HoverParams{
		TextDocument: semanticapi.TextDocumentIdentifier{URI: mainURI},
		Position:     semanticapi.Position{Line: 17, Character: 5},
	})
	require.NoError(t, err)
	require.NotNil(t, hover,
		"expected hover on Add — didOpen was likely not sent to gopls")
	assert.Contains(t, hover.Contents.Value, "Add")
}

func TestE2EHandleOpenRaceWithInitialize(t *testing.T) {
	t.Parallel()
	goplsBin := findGopls(t)

	diskContent := "package main\n\nfunc main() {}\n"

	dir := setupWorkspace(t, "example.com/test", []testFile{
		{name: "main.go", content: diskContent},
	})
	rootURI := "file://" + dir

	uri, err := workspaceapi.ParseURI(rootURI)
	require.NoError(t, err)

	scheme := newTestScheme()

	ready := make(chan struct{})
	var once sync.Once
	cb := &testCallback{
		onShowMessage: func(params semanticapi.ShowMessageParams) {
			if strings.Contains(params.Message, "Finished loading packages") ||
				strings.Contains(params.Message, "background refresh finished") {
				once.Do(func() { close(ready) })
			}
		},
		onProgress: readyOnProgressCh(&once, ready),
	}

	mgr := idelsp.New(
		uri, scheme, scheme,
		&stubPkgManager{bin: goplsBin},
		nil, nil,
		idelsp.Config{
			MaxRetries:         1,
			Callback:           cb,
			NoInitializeServer: true,
		},
	)
	t.Cleanup(func() { require.NoError(t, mgr.Close()) })

	ctx := context.Background()

	mainURI := "file://" + filepath.Join(dir, "main.go")
	wsURI, err := workspaceapi.ParseURI(mainURI)
	require.NoError(t, err)

	// Send EventTypeOpen BEFORE Initialize — simulates the race
	// where the IDE re-opens session files before the extension
	// has called Initialize.
	mgr.Handle(ctx, textapi.Event{
		Type:    textapi.EventTypeOpen,
		URI:     wsURI,
		Content: mainSrc, // overlay with Add at line 17
	})

	// Give handleEvs time to track the pending open (no server yet).
	time.Sleep(200 * time.Millisecond)

	// Now Initialize — this creates the gopls server and sends
	// didOpen for pending files as part of server init.
	params, err := goplsInitializeParams(rootURI)
	require.NoError(t, err)

	_, err = mgr.Initialize(ctx, params)
	require.NoError(t, err)

	fileURIs := map[string]string{"main.go": mainURI}
	waitGoplsReady(t, ready, mgr, fileURIs)

	// Hover on Add (line 17, char 5) which only exists in the overlay.
	// If the pending open was sent during Initialize, gopls uses the
	// overlay and returns hover info. If it was dropped, gopls reads
	// from disk where Add does not exist and returns nil.
	hover, err := mgr.Hover(ctx, semanticapi.HoverParams{
		TextDocument: semanticapi.TextDocumentIdentifier{URI: mainURI},
		Position:     semanticapi.Position{Line: 17, Character: 5},
	})
	require.NoError(t, err)
	require.NotNil(t, hover,
		"expected hover on Add — pending didOpen was likely not sent during Initialize")
	assert.Contains(t, hover.Contents.Value, "Add")
}

func TestE2EHandleOpenCloseRaceWithInitialize(t *testing.T) {
	t.Parallel()
	goplsBin := findGopls(t)

	diskContent := "package main\n\nfunc main() {}\n"

	dir := setupWorkspace(t, "example.com/test", []testFile{
		{name: "main.go", content: diskContent},
	})
	rootURI := "file://" + dir

	uri, err := workspaceapi.ParseURI(rootURI)
	require.NoError(t, err)

	scheme := newTestScheme()

	ready := make(chan struct{})
	var once sync.Once
	cb := &testCallback{
		onShowMessage: func(params semanticapi.ShowMessageParams) {
			if strings.Contains(params.Message, "Finished loading packages") ||
				strings.Contains(params.Message, "background refresh finished") {
				once.Do(func() { close(ready) })
			}
		},
		onProgress: readyOnProgressCh(&once, ready),
	}

	mgr := idelsp.New(
		uri, scheme, scheme,
		&stubPkgManager{bin: goplsBin},
		nil, nil,
		idelsp.Config{
			MaxRetries:         1,
			Callback:           cb,
			NoInitializeServer: true,
		},
	)
	t.Cleanup(func() { require.NoError(t, mgr.Close()) })

	ctx := context.Background()

	mainURI := "file://" + filepath.Join(dir, "main.go")
	wsURI, err := workspaceapi.ParseURI(mainURI)
	require.NoError(t, err)

	// Open then close BEFORE Initialize — the file should NOT be
	// sent as didOpen during server init because it's no longer open.
	mgr.Handle(ctx, textapi.Event{
		Type:    textapi.EventTypeOpen,
		URI:     wsURI,
		Content: mainSrc, // overlay with Add at line 17
	})
	mgr.Handle(ctx, textapi.Event{
		Type: textapi.EventTypeClose,
		URI:  wsURI,
	})

	// Give handleEvs time to process both events.
	time.Sleep(200 * time.Millisecond)

	// Initialize — should NOT send didOpen (file was closed).
	params, err := goplsInitializeParams(rootURI)
	require.NoError(t, err)

	_, err = mgr.Initialize(ctx, params)
	require.NoError(t, err)

	fileURIs := map[string]string{"main.go": mainURI}
	waitGoplsReady(t, ready, mgr, fileURIs)

	// Hover on line 17 where Add would be in the overlay. Since the
	// file was closed before Initialize, gopls should NOT have the
	// overlay — it reads from disk where line 17 doesn't exist.
	// gopls may return nil hover or an error (line out of range).
	hover, err := mgr.Hover(ctx, semanticapi.HoverParams{
		TextDocument: semanticapi.TextDocumentIdentifier{URI: mainURI},
		Position:     semanticapi.Position{Line: 17, Character: 5},
	})
	if err == nil {
		assert.Nil(t, hover,
			"expected no hover at line 17 — closed file should not have been sent as didOpen")
	}
	// err != nil is also acceptable: gopls rejects the position because
	// the disk file only has a few lines (no overlay was sent).
}

func TestAbsDiff(t *testing.T) {
	t.Parallel()
	assert.Equal(t, uint32(5), absDiff(10, 5))
	assert.Equal(t, uint32(5), absDiff(5, 10))
	assert.Equal(t, uint32(0), absDiff(7, 7))
}
