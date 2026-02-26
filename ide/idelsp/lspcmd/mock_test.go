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

package lspcmd

import (
	"context"
	"os"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
)

// mockEditor implements textapi.Editor for testing.
// No embedding to avoid field/method name conflict
// with the Editor() method.
type mockEditor struct {
	setLocationListFn func(textapi.Handler, textapi.LocationPriority, string, textapi.LocationList) error
	setCursorFn       func(textapi.Handler, term.Coordinates) error
	cellEditorFn      func(textapi.Handler) textapi.CellEditor
	editorFn          func(workspaceapi.URI) (textapi.Handler, error)
	subscribeEventsFn func([]textapi.EventType, textapi.EventHandler) error
	cellViewFn        func(textapi.Handler) textapi.CellView
}

func (m *mockEditor) SubscribeEvents(
	types []textapi.EventType,
	h textapi.EventHandler,
) error {
	if m.subscribeEventsFn != nil {
		return m.subscribeEventsFn(types, h)
	}
	return nil
}

func (m *mockEditor) SetLocationList(h textapi.Handler, p textapi.LocationPriority, id string, list textapi.LocationList) error {
	if m.setLocationListFn != nil {
		return m.setLocationListFn(h, p, id, list)
	}
	return nil
}

func (m *mockEditor) SetCursor(h textapi.Handler, c term.Coordinates) error {
	if m.setCursorFn != nil {
		return m.setCursorFn(h, c)
	}
	return nil
}

func (m *mockEditor) CellEditor(h textapi.Handler) textapi.CellEditor {
	if m.cellEditorFn != nil {
		return m.cellEditorFn(h)
	}
	return nil
}

func (m *mockEditor) Editor(uri workspaceapi.URI) (textapi.Handler, error) {
	if m.editorFn != nil {
		return m.editorFn(uri)
	}
	return nil, nil
}

func (m *mockEditor) MoveToNextLocation(_ textapi.Handler, _ string) error {
	return nil
}

func (m *mockEditor) MoveToPrevLocation(_ textapi.Handler, _ string) error {
	return nil
}

func (m *mockEditor) Cursor(_ textapi.Handler) (term.Coordinates, error) {
	return term.Coordinates{}, nil
}

func (m *mockEditor) CellView(h textapi.Handler) textapi.CellView {
	if m.cellViewFn != nil {
		return m.cellViewFn(h)
	}
	return nil
}

func (m *mockEditor) SetDefaultAttributes(_ textapi.Handler, _ term.Attributes) error {
	return nil
}

// mockCellEditor implements textapi.CellEditor.
type mockCellEditor struct {
	editFn func(context.Context, term.Coordinates, term.Coordinates, string,
	) (term.Coordinates, term.Coordinates, string, error)
}

func (m *mockCellEditor) Edit(ctx context.Context, start, end term.Coordinates, str string) (
	term.Coordinates, term.Coordinates, string, error,
) {
	return m.editFn(ctx, start, end, str)
}

// mockCellView implements textapi.CellView.
type mockCellView struct {
	rawCellsFn func() ([][]term.Cell, error)
}

func (m *mockCellView) RawCells() ([][]term.Cell, error) {
	return m.rawCellsFn()
}

// mockWindow implements browserapi.Window for testing.
type mockWindow struct {
	id uint64
}

func (m *mockWindow) WindowID() uint64 { return m.id }

// mockWindowManager implements
// browserapi.WindowManager for testing.
type mockWindowManager struct {
	focusFn            func() (browserapi.Window, error)
	floatingFn         func(browserapi.Floating, browserapi.FloatingConfig) (browserapi.Window, error)
	setWindowContentFn func(browserapi.Window, browserapi.Handler) error
}

func (m *mockWindowManager) Focus() (browserapi.Window, error) {
	if m.focusFn != nil {
		return m.focusFn()
	}
	return nil, nil
}

func (m *mockWindowManager) Split(
	_ browserapi.Orientation, _ browserapi.Window, _ browserapi.Handler,
) (browserapi.Window, error) {
	return nil, nil
}

func (m *mockWindowManager) Floating(h browserapi.Floating, cfg browserapi.FloatingConfig) (browserapi.Window, error) {
	if m.floatingFn != nil {
		return m.floatingFn(h, cfg)
	}
	return nil, nil
}

func (m *mockWindowManager) Bar(_ browserapi.BarConfig, _ tui.Handler) error {
	return nil
}

func (m *mockWindowManager) Tab(_ workspaceapi.URI, _ rune, _ string, _ browserapi.Handler) (browserapi.Handler, error) {
	return nil, nil
}

func (m *mockWindowManager) SetWindowContent(w browserapi.Window, h browserapi.Handler) error {
	if m.setWindowContentFn != nil {
		return m.setWindowContentFn(w, h)
	}
	return nil
}

func (m *mockWindowManager) CloseWindow(_ browserapi.Window) error {
	return nil
}

// mockResourceOpener implements
// browserapi.ResourceOpener.
type mockResourceOpener struct {
	openFn func(workspaceapi.URI) (browserapi.Handler, error)
}

func (m *mockResourceOpener) Open(uri workspaceapi.URI) (browserapi.Handler, error) {
	if m.openFn != nil {
		return m.openFn(uri)
	}
	return nil, nil
}

// mockNotifications implements browserapi.Notifications.
type mockNotifications struct {
	notifyFn func(browserapi.NotificationLevel, string, ...any) (string, error)
}

func (m *mockNotifications) Notify(level browserapi.NotificationLevel, msg string, args ...any) (string, error) {
	if m.notifyFn != nil {
		return m.notifyFn(level, msg, args...)
	}
	return "", nil
}

func (m *mockNotifications) NotifyOnce(level browserapi.NotificationLevel, msg string, args ...any) (string, error) {
	return m.Notify(level, msg, args...)
}

func (m *mockNotifications) UpdateNotificationProgress(_, _ string, _, _ int64) error {
	return nil
}

// mockFileSystem implements workspaceapi.FileSystem.
type mockFileSystem struct {
	openFileFn func(string, int, os.FileMode) (workspaceapi.File, error)
}

func (m *mockFileSystem) URI(path string) (workspaceapi.URI, error) {
	return workspaceapi.ParseURI("file://" + path)
}

func (m *mockFileSystem) OpenFile(path string, flag int, mode os.FileMode) (workspaceapi.File, error) {
	if m.openFileFn != nil {
		return m.openFileFn(path, flag, mode)
	}
	return nil, os.ErrNotExist
}

func (m *mockFileSystem) Remove(_ string) error                   { return nil }
func (m *mockFileSystem) Stat(_ string) (os.FileInfo, error)      { return nil, os.ErrNotExist }
func (m *mockFileSystem) ReadDir(_ string) ([]os.DirEntry, error) { return nil, nil }
func (m *mockFileSystem) MkdirAll(_ string, _ os.FileMode) error  { return nil }

// mockHandler implements textapi.Handler.
type mockHandler struct {
	uri workspaceapi.URI
	sel string
	has bool
}

func (m *mockHandler) Resize(_, _ int)    {}
func (m *mockHandler) Draw(_ term.Writer) {}
func (m *mockHandler) Close() error {
	return nil
}

func (m *mockHandler) Handle(_ term.Event) (bool, bool) {
	return false, false
}

func (m *mockHandler) Cursor() (
	term.Coordinates, term.CursorStyle, bool,
) {
	return term.Coordinates{},
		term.CursorStyleDefault, false
}

func (m *mockHandler) Selection() (
	string, bool,
) {
	return m.sel, m.has
}

func (m *mockHandler) Resource() workspaceapi.URI {
	return m.uri
}

// stubLSP implements semanticapi.LSP with no-op stubs
// for all methods. Embed in mockLSP to override only
// the methods needed for a given test.
type stubLSP struct{}

func (stubLSP) Initialize(_ context.Context, _ semanticapi.InitializeParams) (semanticapi.InitializeResult, error) {
	return semanticapi.InitializeResult{}, nil
}
func (stubLSP) Initialized(_ context.Context) error { return nil }
func (stubLSP) Shutdown(_ context.Context) error    { return nil }
func (stubLSP) Exit(_ context.Context) error         { return nil }
func (stubLSP) DidOpen(_ context.Context, _ semanticapi.DidOpenTextDocumentParams) error {
	return nil
}
func (stubLSP) DidChange(_ context.Context, _ semanticapi.DidChangeTextDocumentParams) error {
	return nil
}
func (stubLSP) DidClose(_ context.Context, _ semanticapi.DidCloseTextDocumentParams) error {
	return nil
}
func (stubLSP) DidSave(_ context.Context, _ semanticapi.DidSaveTextDocumentParams) error {
	return nil
}
func (stubLSP) Completion(_ context.Context, _ semanticapi.CompletionParams) (semanticapi.CompletionResult, error) {
	return semanticapi.CompletionResult{}, nil
}
func (stubLSP) Hover(_ context.Context, _ semanticapi.HoverParams) (*semanticapi.Hover, error) {
	return nil, nil
}
func (stubLSP) SignatureHelp(_ context.Context, _ semanticapi.SignatureHelpParams) (*semanticapi.SignatureHelp, error) {
	return nil, nil
}
func (stubLSP) Definition(_ context.Context, _ semanticapi.DefinitionParams) (semanticapi.LocationResult, error) {
	return semanticapi.LocationResult{}, nil
}
func (stubLSP) Declaration(_ context.Context, _ semanticapi.DeclarationParams) (semanticapi.LocationResult, error) {
	return semanticapi.LocationResult{}, nil
}
func (stubLSP) TypeDefinition(_ context.Context, _ semanticapi.TypeDefinitionParams) (semanticapi.LocationResult, error) {
	return semanticapi.LocationResult{}, nil
}
func (stubLSP) Implementation(_ context.Context, _ semanticapi.ImplementationParams) (semanticapi.LocationResult, error) {
	return semanticapi.LocationResult{}, nil
}
func (stubLSP) References(_ context.Context, _ semanticapi.ReferenceParams) ([]semanticapi.Location, error) {
	return nil, nil
}
func (stubLSP) DocumentHighlight(_ context.Context, _ semanticapi.DocumentHighlightParams) ([]semanticapi.DocumentHighlight, error) {
	return nil, nil
}
func (stubLSP) DocumentSymbol(_ context.Context, _ semanticapi.DocumentSymbolParams) (semanticapi.DocumentSymbolResult, error) {
	return semanticapi.DocumentSymbolResult{}, nil
}
func (stubLSP) CodeAction(_ context.Context, _ semanticapi.CodeActionParams) ([]semanticapi.CodeActionResult, error) {
	return nil, nil
}
func (stubLSP) CodeLens(_ context.Context, _ semanticapi.CodeLensParams) ([]semanticapi.CodeLens, error) {
	return nil, nil
}
func (stubLSP) Formatting(_ context.Context, _ semanticapi.DocumentFormattingParams) ([]semanticapi.TextEdit, error) {
	return nil, nil
}
func (stubLSP) RangeFormatting(_ context.Context, _ semanticapi.DocumentRangeFormattingParams) ([]semanticapi.TextEdit, error) {
	return nil, nil
}
func (stubLSP) Rename(_ context.Context, _ semanticapi.RenameParams) (*semanticapi.WorkspaceEdit, error) {
	return nil, nil
}
func (stubLSP) PrepareRename(_ context.Context, _ semanticapi.PrepareRenameParams) (*semanticapi.PrepareRenameResult, error) {
	return nil, nil
}
func (stubLSP) FoldingRange(_ context.Context, _ semanticapi.FoldingRangeParams) ([]semanticapi.FoldingRange, error) {
	return nil, nil
}
func (stubLSP) SelectionRange(_ context.Context, _ semanticapi.SelectionRangeParams) ([]semanticapi.SelectionRange, error) {
	return nil, nil
}
func (stubLSP) SemanticTokensFull(_ context.Context, _ semanticapi.SemanticTokensParams) (*semanticapi.SemanticTokens, error) {
	return nil, nil
}
func (stubLSP) SemanticTokensRange(_ context.Context, _ semanticapi.SemanticTokensRangeParams) (*semanticapi.SemanticTokens, error) {
	return nil, nil
}
func (stubLSP) Diagnostic(_ context.Context, _ semanticapi.DocumentDiagnosticParams) (semanticapi.DocumentDiagnosticReport, error) {
	return semanticapi.DocumentDiagnosticReport{}, nil
}
func (stubLSP) WorkspaceDiagnostic(_ context.Context, _ semanticapi.WorkspaceDiagnosticParams) (semanticapi.WorkspaceDiagnosticReport, error) {
	return semanticapi.WorkspaceDiagnosticReport{}, nil
}
func (stubLSP) WorkspaceSymbol(_ context.Context, _ semanticapi.WorkspaceSymbolParams) ([]semanticapi.SymbolInformation, error) {
	return nil, nil
}
func (stubLSP) ExecuteCommand(_ context.Context, _ semanticapi.ExecuteCommandParams) (string, error) {
	return "", nil
}
func (stubLSP) PrepareCallHierarchy(_ context.Context, _ semanticapi.CallHierarchyPrepareParams) ([]semanticapi.CallHierarchyItem, error) {
	return nil, nil
}
func (stubLSP) CallHierarchyIncomingCalls(_ context.Context, _ semanticapi.CallHierarchyIncomingCallsParams) ([]semanticapi.CallHierarchyIncomingCall, error) {
	return nil, nil
}
func (stubLSP) CallHierarchyOutgoingCalls(_ context.Context, _ semanticapi.CallHierarchyOutgoingCallsParams) ([]semanticapi.CallHierarchyOutgoingCall, error) {
	return nil, nil
}
func (stubLSP) CompletionResolve(_ context.Context, item semanticapi.CompletionItem) (semanticapi.CompletionItem, error) {
	return item, nil
}
func (stubLSP) CodeLensResolve(_ context.Context, lens semanticapi.CodeLens) (semanticapi.CodeLens, error) {
	return lens, nil
}
func (stubLSP) DocumentColor(_ context.Context, _ semanticapi.DocumentColorParams) ([]semanticapi.ColorInformation, error) {
	return nil, nil
}
func (stubLSP) ColorPresentation(_ context.Context, _ semanticapi.ColorPresentationParams) ([]semanticapi.ColorPresentation, error) {
	return nil, nil
}
func (stubLSP) DocumentLink(_ context.Context, _ semanticapi.DocumentLinkParams) ([]semanticapi.DocumentLink, error) {
	return nil, nil
}
func (stubLSP) DocumentLinkResolve(_ context.Context, link semanticapi.DocumentLink) (semanticapi.DocumentLink, error) {
	return link, nil
}
func (stubLSP) OnTypeFormatting(_ context.Context, _ semanticapi.DocumentOnTypeFormattingParams) ([]semanticapi.TextEdit, error) {
	return nil, nil
}
func (stubLSP) LinkedEditingRange(_ context.Context, _ semanticapi.LinkedEditingRangeParams) (*semanticapi.LinkedEditingRanges, error) {
	return nil, nil
}
func (stubLSP) Moniker(_ context.Context, _ semanticapi.MonikerParams) ([]semanticapi.Moniker, error) {
	return nil, nil
}
func (stubLSP) WillSaveWaitUntil(_ context.Context, _ semanticapi.WillSaveTextDocumentParams) ([]semanticapi.TextEdit, error) {
	return nil, nil
}
func (stubLSP) SemanticTokensFullDelta(_ context.Context, _ semanticapi.SemanticTokensDeltaParams) (*semanticapi.SemanticTokensDelta, error) {
	return nil, nil
}
func (stubLSP) PrepareTypeHierarchy(_ context.Context, _ semanticapi.TypeHierarchyPrepareParams) ([]semanticapi.TypeHierarchyItem, error) {
	return nil, nil
}
func (stubLSP) TypeHierarchySupertypes(_ context.Context, _ semanticapi.TypeHierarchySupertypesParams) ([]semanticapi.TypeHierarchyItem, error) {
	return nil, nil
}
func (stubLSP) TypeHierarchySubtypes(_ context.Context, _ semanticapi.TypeHierarchySubtypesParams) ([]semanticapi.TypeHierarchyItem, error) {
	return nil, nil
}
func (stubLSP) InlayHint(_ context.Context, _ semanticapi.InlayHintParams) ([]semanticapi.InlayHint, error) {
	return nil, nil
}
func (stubLSP) InlayHintResolve(_ context.Context, hint semanticapi.InlayHint) (semanticapi.InlayHint, error) {
	return hint, nil
}
func (stubLSP) InlineValue(_ context.Context, _ semanticapi.InlineValueParams) ([]semanticapi.InlineValue, error) {
	return nil, nil
}
func (stubLSP) WillCreateFiles(_ context.Context, _ semanticapi.CreateFilesParams) (*semanticapi.WorkspaceEdit, error) {
	return nil, nil
}
func (stubLSP) WillRenameFiles(_ context.Context, _ semanticapi.RenameFilesParams) (*semanticapi.WorkspaceEdit, error) {
	return nil, nil
}
func (stubLSP) WillDeleteFiles(_ context.Context, _ semanticapi.DeleteFilesParams) (*semanticapi.WorkspaceEdit, error) {
	return nil, nil
}
func (stubLSP) WillSave(_ context.Context, _ semanticapi.WillSaveTextDocumentParams) error {
	return nil
}
func (stubLSP) DidChangeConfiguration(_ context.Context, _ semanticapi.DidChangeConfigurationParams) error {
	return nil
}
func (stubLSP) DidChangeWatchedFiles(_ context.Context, _ semanticapi.DidChangeWatchedFilesParams) error {
	return nil
}
func (stubLSP) DidChangeWorkspaceFolders(_ context.Context, _ semanticapi.DidChangeWorkspaceFoldersParams) error {
	return nil
}
func (stubLSP) WorkDoneProgressCancel(_ context.Context, _ semanticapi.WorkDoneProgressCancelParams) error {
	return nil
}
func (stubLSP) SetTrace(_ context.Context, _ semanticapi.SetTraceParams) error { return nil }
func (stubLSP) DidCreateFiles(_ context.Context, _ semanticapi.CreateFilesParams) error {
	return nil
}
func (stubLSP) DidRenameFiles(_ context.Context, _ semanticapi.RenameFilesParams) error {
	return nil
}
func (stubLSP) DidDeleteFiles(_ context.Context, _ semanticapi.DeleteFilesParams) error {
	return nil
}

// mockLSP embeds stubLSP and overrides specific
// methods for testing.
type mockLSP struct {
	stubLSP
	completionFn         func(context.Context, semanticapi.CompletionParams) (semanticapi.CompletionResult, error)
	definitionFn         func(context.Context, semanticapi.DefinitionParams) (semanticapi.LocationResult, error)
	declarationFn        func(context.Context, semanticapi.DeclarationParams) (semanticapi.LocationResult, error)
	typeDefinitionFn     func(context.Context, semanticapi.TypeDefinitionParams) (semanticapi.LocationResult, error)
	implementationFn     func(context.Context, semanticapi.ImplementationParams) (semanticapi.LocationResult, error)
	referencesFn         func(context.Context, semanticapi.ReferenceParams) ([]semanticapi.Location, error)
	documentHighlightFn  func(context.Context, semanticapi.DocumentHighlightParams) ([]semanticapi.DocumentHighlight, error)
}

func (m *mockLSP) Completion(
	ctx context.Context,
	params semanticapi.CompletionParams,
) (semanticapi.CompletionResult, error) {
	if m.completionFn != nil {
		return m.completionFn(ctx, params)
	}
	return semanticapi.CompletionResult{}, nil
}

func (m *mockLSP) Definition(
	ctx context.Context,
	params semanticapi.DefinitionParams,
) (semanticapi.LocationResult, error) {
	if m.definitionFn != nil {
		return m.definitionFn(ctx, params)
	}
	return semanticapi.LocationResult{}, nil
}

func (m *mockLSP) Declaration(
	ctx context.Context,
	params semanticapi.DeclarationParams,
) (semanticapi.LocationResult, error) {
	if m.declarationFn != nil {
		return m.declarationFn(ctx, params)
	}
	return semanticapi.LocationResult{}, nil
}

func (m *mockLSP) TypeDefinition(
	ctx context.Context,
	params semanticapi.TypeDefinitionParams,
) (semanticapi.LocationResult, error) {
	if m.typeDefinitionFn != nil {
		return m.typeDefinitionFn(ctx, params)
	}
	return semanticapi.LocationResult{}, nil
}

func (m *mockLSP) Implementation(
	ctx context.Context,
	params semanticapi.ImplementationParams,
) (semanticapi.LocationResult, error) {
	if m.implementationFn != nil {
		return m.implementationFn(ctx, params)
	}
	return semanticapi.LocationResult{}, nil
}

func (m *mockLSP) References(
	ctx context.Context,
	params semanticapi.ReferenceParams,
) ([]semanticapi.Location, error) {
	if m.referencesFn != nil {
		return m.referencesFn(ctx, params)
	}
	return nil, nil
}

func (m *mockLSP) DocumentHighlight(
	ctx context.Context,
	params semanticapi.DocumentHighlightParams,
) ([]semanticapi.DocumentHighlight, error) {
	if m.documentHighlightFn != nil {
		return m.documentHighlightFn(ctx, params)
	}
	return nil, nil
}
