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
	"log/slog"
	"strings"

	"github.com/gobwas/glob"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
)

// resolveGlobPattern extracts a pattern string from an LSP GlobPattern,
// which per LSP 3.17 can be either a plain JSON string or a RelativePattern object.
func resolveGlobPattern(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var rp semanticapi.RelativePattern
	if err := json.Unmarshal(raw, &rp); err != nil {
		return "", err
	}
	return rp.Pattern, nil
}

// matchGlob matches a file path against an LSP glob pattern.
// It handles **, *, ?, [...] and {a,b} via github.com/gobwas/glob.
func matchGlob(pattern, filePath string) bool {
	g, err := glob.Compile(pattern, '/')
	if err != nil {
		return false
	}
	if g.Match(filePath) {
		return true
	}
	// LSP spec: ** matches zero or more path segments, but gobwas/glob
	// with '/' separator requires at least one. Handle zero segments by
	// stripping leading **/ and retrying against the bare filename.
	if strings.HasPrefix(pattern, "**/") {
		return matchGlob(pattern[3:], filePath)
	}
	return false
}

// nopLSPCallback is a no-op implementation of
// semanticapi.LSPCallback used when no callback is provided.
type nopLSPCallback struct{}

func (nopLSPCallback) ShowMessage(
	_ context.Context, _ semanticapi.ShowMessageParams,
) error {
	return nil
}

func (nopLSPCallback) LogMessage(
	_ context.Context, _ semanticapi.LogMessageParams,
) error {
	return nil
}

func (nopLSPCallback) PublishDiagnostics(
	_ context.Context,
	_ semanticapi.PublishDiagnosticsParams,
) error {
	return nil
}

func (nopLSPCallback) Progress(
	_ context.Context, _ semanticapi.ProgressParams,
) error {
	return nil
}

func (nopLSPCallback) LogTrace(
	_ context.Context, _ semanticapi.LogTraceParams,
) error {
	return nil
}

func (nopLSPCallback) ShowDocument(
	_ context.Context, _ semanticapi.ShowDocumentParams,
) (semanticapi.ShowDocumentResult, error) {
	return semanticapi.ShowDocumentResult{}, nil
}

func (nopLSPCallback) ShowMessageRequest(
	_ context.Context,
	_ semanticapi.ShowMessageRequestParams,
) (*semanticapi.MessageActionItem, error) {
	return nil, nil
}

func (nopLSPCallback) WorkDoneProgressCreate(
	_ context.Context,
	_ semanticapi.WorkDoneProgressCreateParams,
) error {
	return nil
}

func (nopLSPCallback) ApplyEdit(
	_ context.Context,
	_ semanticapi.ApplyWorkspaceEditParams,
) (semanticapi.ApplyWorkspaceEditResult, error) {
	return semanticapi.ApplyWorkspaceEditResult{}, nil
}

func (nopLSPCallback) WorkspaceFolders(
	_ context.Context,
) ([]semanticapi.WorkspaceFolder, error) {
	return nil, nil
}

func (nopLSPCallback) Configuration(
	_ context.Context,
	_ semanticapi.ConfigurationParams,
) ([]json.RawMessage, error) {
	return nil, nil
}

func (nopLSPCallback) RegisterCapability(
	_ context.Context, _ semanticapi.RegistrationParams,
) error {
	return nil
}

func (nopLSPCallback) UnregisterCapability(
	_ context.Context, _ semanticapi.UnregistrationParams,
) error {
	return nil
}

func (nopLSPCallback) CodeLensRefresh(
	_ context.Context,
) error {
	return nil
}

func (nopLSPCallback) SemanticTokensRefresh(
	_ context.Context,
) error {
	return nil
}

func (nopLSPCallback) InlayHintRefresh(
	_ context.Context,
) error {
	return nil
}

func (nopLSPCallback) DiagnosticRefresh(
	_ context.Context,
) error {
	return nil
}

// callbackInterceptor wraps an LSPCallback to intercept
// RegisterCapability and UnregisterCapability for file watchers.
type callbackInterceptor struct {
	semanticapi.LSPCallback
	manager  *Manager
	serverID string
	log      *slog.Logger
}

func (c *callbackInterceptor) RegisterCapability(
	ctx context.Context, params semanticapi.RegistrationParams,
) error {
	for _, reg := range params.Registrations {
		if reg.Method != "workspace/didChangeWatchedFiles" {
			continue
		}
		var opts semanticapi.DidChangeWatchedFilesRegistrationOptions
		if err := json.Unmarshal(reg.RegisterOptions, &opts); err != nil {
			c.log.Warn("unmarshal watcher registration options",
				"id", reg.ID, "error", err)
			continue
		}
		c.manager.addWatchers(c.serverID, reg.ID, opts.Watchers)
		c.log.Debug("registered file watchers",
			"server", c.serverID, "id", reg.ID,
			"count", len(opts.Watchers))
	}
	return c.LSPCallback.RegisterCapability(ctx, params)
}

func (c *callbackInterceptor) UnregisterCapability(
	ctx context.Context, params semanticapi.UnregistrationParams,
) error {
	for _, unreg := range params.Unregistrations {
		if unreg.Method != "workspace/didChangeWatchedFiles" {
			continue
		}
		c.manager.removeWatchers(c.serverID, unreg.ID)
		c.log.Debug("unregistered file watchers",
			"server", c.serverID, "id", unreg.ID)
	}
	return c.LSPCallback.UnregisterCapability(ctx, params)
}
