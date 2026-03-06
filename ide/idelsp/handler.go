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

package idelsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/unstablebuild/blue/tui/handler/html"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

var errCouldNotSchedule = errors.New(
	"could not schedule operation",
)

// Refresher handles LSP server refresh requests.
type Refresher interface {
	RefreshCodeLens(ctx context.Context) error
	RefreshSemanticTokens(ctx context.Context) error
	RefreshInlayHints(ctx context.Context) error
	RefreshDiagnostics(ctx context.Context) error
}

// WindowManager provides floating window management for
// LSP callback prompts.
type WindowManager interface {
	Floating(h browserapi.Floating, cfg browserapi.FloatingConfig) (browserapi.Window, error)
	CloseWindow(browserapi.Window) error
}

// Editor provides text editing capabilities needed by
// LSP callbacks.
type Editor interface {
	Editor(resource workspaceapi.URI) (textapi.Handler, error)
	SetLocationList(textapi.Handler, textapi.LocationPriority, string, textapi.LocationList) error
	SetCursor(textapi.Handler, term.Coordinates) error
	CellEditor(textapi.Handler) textapi.CellEditor
}

// CallbackHandlerConfig holds optional dependencies for
// CallbackHandler.
type CallbackHandlerConfig struct {
	Config           config.Config
	Refresher        Refresher
	Interrupter      term.Interrupter
	ScheduleNextTick func(fn func()) bool
}

// CallbackHandler implements semanticapi.LSPCallback by
// wiring LSP server-to-client callbacks to the Rune IDE.
type CallbackHandler struct {
	notifications    browserapi.Notifications
	windowManager    WindowManager
	resourceOpener   browserapi.ResourceOpener
	editor           Editor
	fileSystem       schemeapi.FileSystem
	rootURI          string
	config           config.Config
	refresher        Refresher
	interrupter      term.Interrupter
	scheduleNextTick func(fn func()) bool

	mu       sync.Mutex
	progress map[string]string
}

var _ semanticapi.LSPCallback = (*CallbackHandler)(nil)

// NewCallbackHandler creates a new CallbackHandler.
func NewCallbackHandler(
	notifications browserapi.Notifications,
	windowManager WindowManager,
	resourceOpener browserapi.ResourceOpener,
	editor Editor,
	fileSystem schemeapi.FileSystem,
	rootURI string,
	cfg CallbackHandlerConfig,
) *CallbackHandler {
	r := cfg.Refresher
	if r == nil {
		r = nopRefresher{}
	}
	interrupter := cfg.Interrupter
	if interrupter == nil {
		interrupter = term.NopInterrupter()
	}
	sched := cfg.ScheduleNextTick
	if sched == nil {
		sched = func(fn func()) bool {
			fn()
			return true
		}
	}
	return &CallbackHandler{
		notifications:    notifications,
		windowManager:    windowManager,
		resourceOpener:   resourceOpener,
		editor:           editor,
		fileSystem:       fileSystem,
		rootURI:          rootURI,
		config:           cfg.Config,
		refresher:        r,
		interrupter:      interrupter,
		scheduleNextTick: sched,
		progress:         make(map[string]string),
	}
}

// ShowMessage displays a message notification.
func (h *CallbackHandler) ShowMessage(
	ctx context.Context,
	params semanticapi.ShowMessageParams,
) error {
	level := messageTypeToNotificationLevel(params.Type)
	msg := params.Message
	md, ok := metadataFromContext(ctx)
	if ok && md.ServerName != "" {
		msg = md.ServerName + ": " + msg
	}
	ok = h.scheduleNextTick(func() {
		h.notifications.Notify(level, msg) //nolint:errcheck
	})
	if !ok {
		return errCouldNotSchedule
	}
	return nil
}

// LogMessage logs a message at the appropriate slog level.
func (h *CallbackHandler) LogMessage(
	ctx context.Context, params semanticapi.LogMessageParams,
) error {
	level := messageTypeToSlogLevel(params.Type)
	slog.Log(ctx, level, params.Message)
	return nil
}

// PublishDiagnostics publishes diagnostics for a document.
func (h *CallbackHandler) PublishDiagnostics(
	_ context.Context,
	params semanticapi.PublishDiagnosticsParams,
) error {
	uri, err := workspaceapi.ParseURI(params.URI)
	if err != nil {
		return fmt.Errorf("parse URI: %w", err)
	}

	locs := make([]textapi.Location, 0, len(params.Diagnostics))
	highest := textapi.LocationPriorityInfo
	for _, diag := range params.Diagnostics {
		p := diagnosticSeverityToLocationPriority(diag.Severity)
		if p > highest {
			highest = p
		}

		msg := diag.Message
		attr := diagnosticSeverityToAttr(diag.Severity)
		icon := ""
		if (diag.Source == "compiler" || diag.Source == "optimizer details") &&
			diag.Severity != semanticapi.DiagnosticSeverityError &&
			diag.Severity != semanticapi.DiagnosticSeverityWarning {
			icon, msg, attr = classifyCompilerDiagnostic(diag.Message)
		}

		locs = append(locs, textapi.Location{
			From: term.Coordinates{
				X: int(diag.Range.Start.Character),
				Y: int(diag.Range.Start.Line),
			},
			To: term.Coordinates{
				X: int(diag.Range.End.Character),
				Y: int(diag.Range.End.Line),
			},
			Message: msg,
			Attr:    attr,
			Icon:    icon,
		})
	}
	ll := textapi.LocationSlice(locs)

	ok := h.scheduleNextTick(func() {
		eh, err := h.editor.Editor(uri)
		if err != nil {
			slog.Warn("editor for diagnostics", "uri", uri.Name(), "err", err)
			return
		}
		err = h.editor.SetLocationList(eh, highest, "lsp-diagnostics", ll)
		if err != nil {
			slog.Warn("set diagnostics", "err", err)
		}
	})
	if !ok {
		return errCouldNotSchedule
	}
	return nil
}

// Progress handles $/progress notifications.
func (h *CallbackHandler) Progress(
	_ context.Context, params semanticapi.ProgressParams,
) error {
	var kind struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(params.Value, &kind); err != nil {
		return fmt.Errorf("unmarshal progress kind: %w", err)
	}

	key := progressTokenKey(params.Token)

	switch kind.Kind {
	case "begin":
		var begin struct {
			Title   string `json:"title"`
			Message string `json:"message,omitempty"`
		}
		err := json.Unmarshal(params.Value, &begin)
		if err != nil {
			return fmt.Errorf("unmarshal progress begin: %w", err)
		}
		msg := begin.Title
		if begin.Message != "" {
			msg = begin.Title + ": " + begin.Message
		}
		ok := h.scheduleNextTick(func() {
			id, err := h.notifications.Notify(browserapi.LevelInfo, msg)
			if err != nil {
				slog.Warn("progress begin notify", "err", err)
				return
			}
			h.mu.Lock()
			h.progress[key] = id
			h.mu.Unlock()
		})
		if !ok {
			return errCouldNotSchedule
		}

	case "report":
		var report struct {
			Message    string `json:"message,omitempty"`
			Percentage *int64 `json:"percentage,omitempty"`
		}
		err := json.Unmarshal(params.Value, &report)
		if err != nil {
			return fmt.Errorf("unmarshal progress report: %w", err)
		}
		h.mu.Lock()
		id, ok := h.progress[key]
		h.mu.Unlock()
		if !ok {
			return nil
		}
		var pct int64
		if report.Percentage != nil {
			pct = *report.Percentage
		}
		scheduled := h.scheduleNextTick(func() {
			_ = h.notifications.UpdateNotificationProgress(id, report.Message, pct, 100)
		})
		if !scheduled {
			return errCouldNotSchedule
		}

	case "end":
		h.mu.Lock()
		id, ok := h.progress[key]
		delete(h.progress, key)
		h.mu.Unlock()
		if !ok {
			return nil
		}
		scheduled := h.scheduleNextTick(func() {
			_ = h.notifications.UpdateNotificationProgress(id, "", 100, 100)
		})
		if !scheduled {
			return errCouldNotSchedule
		}
	}
	return nil
}

// LogTrace logs a trace message at debug level.
func (h *CallbackHandler) LogTrace(ctx context.Context, params semanticapi.LogTraceParams) error {
	slog.Log(ctx, slog.LevelDebug, params.Message, "verbose", params.Verbose)
	return nil
}

// ShowDocument requests the client to display a document.
func (h *CallbackHandler) ShowDocument(
	ctx context.Context, params semanticapi.ShowDocumentParams,
) (semanticapi.ShowDocumentResult, error) {
	if strings.HasPrefix(params.URI, "http://") || strings.HasPrefix(params.URI, "https://") {
		return h.showHTTPDocument(params.URI)
	}

	uri, err := workspaceapi.ParseURI(params.URI)
	if err != nil {
		return semanticapi.ShowDocumentResult{Success: false}, fmt.Errorf("parse URI: %w", err)
	}
	prefix := "LSP"
	md, ok := metadataFromContext(ctx)
	if ok && md.ServerName != "" {
		prefix = md.ServerName
	}
	sel := params.Selection
	resultCh := make(chan semanticapi.ShowDocumentResult, 1)
	errCh := make(chan error, 1)
	ok = h.scheduleNextTick(func() {
		if _, err := h.resourceOpener.Open(uri); err != nil {
			errCh <- fmt.Errorf("open resource: %w", err)
			return
		}
		//nolint:errcheck
		h.notifications.Notify(browserapi.LevelInfo, "%s: see %s", prefix, uri.Name())
		if sel != nil {
			eh, err := h.editor.Editor(uri)
			if err == nil {
				cursor := term.Coordinates{X: int(sel.Start.Character), Y: int(sel.Start.Line)}
				_ = h.editor.SetCursor(eh, cursor)
			}
		}
		resultCh <- semanticapi.ShowDocumentResult{Success: true}
	})
	if !ok {
		return semanticapi.ShowDocumentResult{}, errCouldNotSchedule
	}

	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return semanticapi.ShowDocumentResult{Success: false}, err
	case <-ctx.Done():
		return semanticapi.ShowDocumentResult{Success: false}, ctx.Err()
	}
}

func (h *CallbackHandler) showHTTPDocument(rawURL string) (
	semanticapi.ShowDocumentResult, error,
) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return semanticapi.ShowDocumentResult{Success: false}, fmt.Errorf("parse URL: %w", err)
	}
	resultCh := make(chan semanticapi.ShowDocumentResult, 1)
	errCh := make(chan error, 1)
	handler := html.New(h.interrupter, parsed)
	ok := h.scheduleNextTick(func() {
		_, err := h.windowManager.Floating(handler, browserapi.FloatingConfig{
			Alignment: component.AlignmentCentered,
		})
		if err != nil {
			errCh <- fmt.Errorf("show floating: %w", err)
			return
		}
		resultCh <- semanticapi.ShowDocumentResult{Success: true}
	})
	if !ok {
		return semanticapi.ShowDocumentResult{}, errCouldNotSchedule
	}

	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return semanticapi.ShowDocumentResult{Success: false}, err
	}
}

// ShowMessageRequest shows a message with action items.
func (h *CallbackHandler) ShowMessageRequest(
	ctx context.Context,
	params semanticapi.ShowMessageRequestParams,
) (*semanticapi.MessageActionItem, error) {
	titles := make([]string, len(params.Actions))
	for i, a := range params.Actions {
		titles[i] = a.Title
	}

	ch := make(chan *semanticapi.MessageActionItem, 1)
	prompt := handler.NewPrompt(handler.PromptConfig{
		PromptConfig: component.PromptConfig{
			Message: params.Message,
			Options: titles,
		},
		PromptHandler: handler.FuncPromptHandler(
			func(idx int, _ string) {
				if idx >= 0 && idx < len(params.Actions) {
					ch <- &params.Actions[idx]
				} else {
					ch <- nil
				}
			},
			func() error {
				ch <- nil
				return nil
			},
		),
	})

	winCh := make(chan browserapi.Window, 1)
	errCh := make(chan error, 1)
	ok := h.scheduleNextTick(func() {
		win, err := h.windowManager.Floating(
			prompt, browserapi.FloatingConfig{Alignment: component.AlignmentCentered},
		)
		if err != nil {
			errCh <- fmt.Errorf("show floating: %w", err)
			return
		}
		winCh <- win
	})
	if !ok {
		return nil, errCouldNotSchedule
	}

	var win browserapi.Window
	select {
	case win = <-winCh:
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	defer func() {
		h.scheduleNextTick(func() { //nolint:errcheck
			h.windowManager.CloseWindow(win) //nolint:errcheck
		})
	}()

	select {
	case result := <-ch:
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// WorkDoneProgressCreate pre-allocates a progress token.
func (h *CallbackHandler) WorkDoneProgressCreate(
	_ context.Context,
	params semanticapi.WorkDoneProgressCreateParams,
) error {
	key := progressTokenKey(params.Token)
	h.mu.Lock()
	h.progress[key] = ""
	h.mu.Unlock()
	return nil
}

// ApplyEdit applies a workspace edit.
func (h *CallbackHandler) ApplyEdit(
	ctx context.Context,
	params semanticapi.ApplyWorkspaceEditParams,
) (semanticapi.ApplyWorkspaceEditResult, error) {
	edit := params.Edit

	if len(edit.DocumentChanges) > 0 {
		ok := h.scheduleNextTick(func() {
			err := h.applyDocumentChanges(ctx, edit.DocumentChanges)
			if err != nil {
				slog.Warn("apply document changes", "err", err)
			}
		})
		if !ok {
			return semanticapi.ApplyWorkspaceEditResult{}, errCouldNotSchedule
		}
		return semanticapi.ApplyWorkspaceEditResult{Applied: true}, nil
	}

	if len(edit.Changes) > 0 {
		ok := h.scheduleNextTick(func() {
			err := h.applyChanges(ctx, edit.Changes)
			if err != nil {
				slog.Warn("apply changes",
					"err", err)
			}
		})
		if !ok {
			return semanticapi.ApplyWorkspaceEditResult{}, errCouldNotSchedule
		}
		return semanticapi.ApplyWorkspaceEditResult{Applied: true}, nil
	}

	return semanticapi.ApplyWorkspaceEditResult{Applied: true}, nil
}

// WorkspaceFolders returns the workspace folders.
func (h *CallbackHandler) WorkspaceFolders(
	_ context.Context,
) ([]semanticapi.WorkspaceFolder, error) {
	name := filepath.Base(h.rootURI)
	return []semanticapi.WorkspaceFolder{
		{URI: h.rootURI, Name: name},
	}, nil
}

// Configuration fetches configuration from the client.
func (h *CallbackHandler) Configuration(
	_ context.Context,
	params semanticapi.ConfigurationParams,
) ([]json.RawMessage, error) {
	results := make(
		[]json.RawMessage, len(params.Items),
	)
	for i, item := range params.Items {
		results[i] = h.configForItem(item)
	}
	return results, nil
}

// RegisterCapability is a no-op.
func (h *CallbackHandler) RegisterCapability(
	_ context.Context, _ semanticapi.RegistrationParams,
) error {
	return nil
}

// UnregisterCapability is a no-op.
func (h *CallbackHandler) UnregisterCapability(
	_ context.Context, _ semanticapi.UnregistrationParams,
) error {
	return nil
}

// CodeLensRefresh delegates to the Refresher.
func (h *CallbackHandler) CodeLensRefresh(
	ctx context.Context,
) error {
	return h.refresher.RefreshCodeLens(ctx)
}

// SemanticTokensRefresh delegates to the Refresher.
func (h *CallbackHandler) SemanticTokensRefresh(
	ctx context.Context,
) error {
	return h.refresher.RefreshSemanticTokens(ctx)
}

// InlayHintRefresh delegates to the Refresher.
func (h *CallbackHandler) InlayHintRefresh(
	ctx context.Context,
) error {
	return h.refresher.RefreshInlayHints(ctx)
}

// DiagnosticRefresh delegates to the Refresher.
func (h *CallbackHandler) DiagnosticRefresh(
	ctx context.Context,
) error {
	return h.refresher.RefreshDiagnostics(ctx)
}

type nopRefresher struct{}

func (nopRefresher) RefreshCodeLens(
	_ context.Context,
) error {
	return nil
}

func (nopRefresher) RefreshSemanticTokens(
	_ context.Context,
) error {
	return nil
}

func (nopRefresher) RefreshInlayHints(
	_ context.Context,
) error {
	return nil
}

func (nopRefresher) RefreshDiagnostics(
	_ context.Context,
) error {
	return nil
}

func progressTokenKey(
	token semanticapi.ProgressToken,
) string {
	if token.IsInteger {
		return fmt.Sprintf("%d", token.IntegerValue)
	}
	return token.StringValue
}

func messageTypeToNotificationLevel(
	mt semanticapi.MessageType,
) browserapi.NotificationLevel {
	switch mt {
	case semanticapi.MessageTypeError:
		return browserapi.LevelError
	case semanticapi.MessageTypeWarning:
		return browserapi.LevelWarn
	default:
		return browserapi.LevelInfo
	}
}

func messageTypeToSlogLevel(
	mt semanticapi.MessageType,
) slog.Level {
	switch mt {
	case semanticapi.MessageTypeError:
		return slog.LevelError
	case semanticapi.MessageTypeWarning:
		return slog.LevelWarn
	case semanticapi.MessageTypeInfo:
		return slog.LevelInfo
	default:
		return slog.LevelDebug
	}
}

func diagnosticSeverityToLocationPriority(
	s semanticapi.DiagnosticSeverity,
) textapi.LocationPriority {
	switch s {
	case semanticapi.DiagnosticSeverityError:
		return textapi.LocationPriorityError
	case semanticapi.DiagnosticSeverityWarning:
		return textapi.LocationPriorityWarning
	default:
		return textapi.LocationPriorityInfo
	}
}

func diagnosticSeverityToAttr(
	s semanticapi.DiagnosticSeverity,
) term.Attributes {
	switch s {
	case semanticapi.DiagnosticSeverityError:
		return term.Attributes(tcell.Style{
			Bg: tcell.ColorRed,
		})
	case semanticapi.DiagnosticSeverityWarning:
		return term.Attributes(tcell.Style{
			Bg: tcell.ColorYellow,
		})
	case semanticapi.DiagnosticSeverityInformation:
		return term.Attributes(tcell.Style{
			Bg: tcell.ColorBlue,
		})
	default:
		return term.Attributes(tcell.Style{
			Bg: tcell.ColorGray,
		})
	}
}

// classifyCompilerDiagnostic categorizes a compiler optimization
// diagnostic (gc_details) by its message content and returns a
// descriptive icon, a prefixed message, and a category-specific color.
func classifyCompilerDiagnostic(msg string) (icon, enhanced string, attr term.Attributes) {
	switch {
	case strings.Contains(msg, "inline") || strings.Contains(msg, "inlining"):
		return "⇒", "Inline: " + msg, term.Attributes(tcell.Style{
			Bg: tcell.ColorIndigo,
		})
	case strings.Contains(msg, "escape") ||
		strings.Contains(msg, "heap") ||
		strings.Contains(msg, "leaking"):
		return "↗", "Escape: " + msg, term.Attributes(tcell.Style{
			Bg: tcell.ColorDarkMagenta,
		})
	case strings.Contains(msg, "Bounds"):
		return "⊞", "Bounds: " + msg, term.Attributes(tcell.Style{
			Bg: tcell.ColorRebeccaPurple,
		})
	case strings.Contains(msg, "nilcheck") ||
		strings.Contains(msg, "nil check"):
		return "∅", "Nilcheck: " + msg, term.Attributes(tcell.Style{
			Bg: tcell.ColorBlueViolet,
		})
	default:
		return "⚙", msg, term.Attributes(tcell.Style{
			Bg: tcell.ColorDarkSlateBlue,
		})
	}
}

func (h *CallbackHandler) applyDocumentChanges(
	ctx context.Context,
	changes []semanticapi.DocumentChange,
) error {
	for _, change := range changes {
		switch {
		case change.TextDocumentEdit != nil:
			if err := h.applyTextDocumentEdit(ctx, change.TextDocumentEdit); err != nil {
				return err
			}
		case change.CreateFile != nil:
			if err := h.applyCreateFile(change.CreateFile); err != nil {
				return err
			}
		case change.RenameFile != nil:
			if err := h.applyRenameFile(change.RenameFile); err != nil {
				return err
			}
		case change.DeleteFile != nil:
			if err := h.applyDeleteFile(change.DeleteFile); err != nil {
				return err
			}
		}
	}
	return nil
}

// editorForURI returns the editor handler for the given URI.
// If the editor is not available, it opens the resource via
// the resource opener and retries.
func (h *CallbackHandler) editorForURI(
	uri workspaceapi.URI,
) (textapi.Handler, error) {
	eh, err := h.editor.Editor(uri)
	if err == nil {
		return eh, nil
	}
	if h.resourceOpener == nil {
		return nil, err
	}
	if _, openErr := h.resourceOpener.Open(uri); openErr != nil {
		return nil, err
	}
	return h.editor.Editor(uri)
}

func (h *CallbackHandler) applyTextDocumentEdit(
	ctx context.Context,
	edit *semanticapi.TextDocumentEdit,
) error {
	uri, err := workspaceapi.ParseURI(edit.TextDocument.URI)
	if err != nil {
		return fmt.Errorf("parse URI: %w", err)
	}
	editorHandler, err := h.editorForURI(uri)
	if err != nil {
		return fmt.Errorf("editor for %s: %w", uri.Name(), err)
	}
	return applyTextEdits(ctx, h.editor, editorHandler, edit.Edits)
}

func applyTextEdits(
	ctx context.Context,
	editor Editor,
	editorHandler textapi.Handler,
	edits []semanticapi.TextEdit,
) error {
	sorted := make([]semanticapi.TextEdit, len(edits))
	copy(sorted, edits)
	sort.SliceStable(sorted, func(i, j int) bool {
		a := sorted[i].Range.Start
		b := sorted[j].Range.Start
		if a.Line != b.Line {
			return a.Line > b.Line
		}
		return a.Character > b.Character
	})
	// Per the LSP spec, when multiple inserts share the same
	// position, the array order defines the resulting text order.
	// Since we apply bottom-to-top, same-position inserts must be
	// reversed so the first-in-array insert ends up first in text.
	reverseSameStartEdits(sorted)

	cellEditor := editor.CellEditor(editorHandler)
	for _, edit := range sorted {
		start := term.Coordinates{
			X: int(edit.Range.Start.Character),
			Y: int(edit.Range.Start.Line),
		}
		end := term.Coordinates{
			X: int(edit.Range.End.Character),
			Y: int(edit.Range.End.Line),
		}
		if _, _, _, err := cellEditor.Edit(
			ctx, start, end, edit.NewText,
		); err != nil {
			return fmt.Errorf("cell edit: %w", err)
		}
	}
	return nil
}

// reverseSameStartEdits reverses each contiguous group of edits
// sharing the same start position so that applying them bottom-to-top
// produces the correct array-order text per the LSP spec.
func reverseSameStartEdits(edits []semanticapi.TextEdit) {
	for i := 0; i < len(edits); {
		j := i + 1
		for j < len(edits) &&
			edits[j].Range.Start == edits[i].Range.Start {
			j++
		}
		if j-i > 1 {
			for l, r := i, j-1; l < r; l, r = l+1, r-1 {
				edits[l], edits[r] = edits[r], edits[l]
			}
		}
		i = j
	}
}

func (h *CallbackHandler) applyCreateFile(
	cf *semanticapi.CreateFile,
) error {
	uri, err := workspaceapi.ParseURI(cf.URI)
	if err != nil {
		return fmt.Errorf("parse URI: %w", err)
	}
	path := uri.Path()

	if cf.Options != nil && cf.Options.IgnoreIfExists {
		if _, err := h.fileSystem.Stat(path); err == nil {
			return nil
		}
	}

	flag := os.O_CREATE | os.O_WRONLY
	if cf.Options != nil && cf.Options.Overwrite {
		flag |= os.O_TRUNC
	} else {
		flag |= os.O_EXCL
	}

	f, err := h.fileSystem.OpenFile(path, flag, 0644)
	if err != nil {
		return fmt.Errorf("create file %s: %w", path, err)
	}
	return f.Close()
}

func (h *CallbackHandler) applyRenameFile(
	rf *semanticapi.RenameFile,
) error {
	oldURI, err := workspaceapi.ParseURI(rf.OldURI)
	if err != nil {
		return fmt.Errorf("parse old URI: %w", err)
	}
	newURI, err := workspaceapi.ParseURI(rf.NewURI)
	if err != nil {
		return fmt.Errorf("parse new URI: %w", err)
	}

	oldPath := oldURI.Path()
	newPath := newURI.Path()

	if rf.Options != nil && rf.Options.IgnoreIfExists {
		if _, err := h.fileSystem.Stat(newPath); err == nil {
			return nil
		}
	}

	return h.fileSystem.Rename(oldPath, newPath)
}

func (h *CallbackHandler) applyDeleteFile(
	df *semanticapi.DeleteFile,
) error {
	uri, err := workspaceapi.ParseURI(df.URI)
	if err != nil {
		return fmt.Errorf("parse URI: %w", err)
	}
	path := uri.Path()

	if df.Options != nil && df.Options.IgnoreIfNotExists {
		if _, err := h.fileSystem.Stat(path); err != nil {
			return nil
		}
	}

	return h.fileSystem.Remove(path)
}

func (h *CallbackHandler) applyChanges(
	ctx context.Context,
	changes map[string][]semanticapi.TextEdit,
) error {
	for uriStr, edits := range changes {
		uri, err := workspaceapi.ParseURI(uriStr)
		if err != nil {
			return fmt.Errorf("parse URI: %w", err)
		}
		editorHandler, err := h.editorForURI(uri)
		if err != nil {
			return fmt.Errorf("editor for %s: %w", uri.Name(), err)
		}
		if err := applyTextEdits(ctx, h.editor, editorHandler, edits); err != nil {
			return err
		}
	}
	return nil
}

func (h *CallbackHandler) configForItem(
	item semanticapi.ConfigurationItem,
) json.RawMessage {
	if h.config == nil {
		return json.RawMessage("null")
	}
	cfg := h.config
	segments := []string{"lsp", "servers"}
	if item.Section != "" {
		segments = append(segments, item.Section)
	}
	for i, seg := range segments {
		if i == len(segments)-1 {
			m, err := cfg.GetMap(seg)
			if err != nil {
				return json.RawMessage("null")
			}
			data, err := json.Marshal(m)
			if err != nil {
				return json.RawMessage("null")
			}
			return data
		}
		next, err := cfg.GetConfig(seg)
		if err != nil {
			return json.RawMessage("null")
		}
		cfg = next
	}
	return json.RawMessage("null")
}
