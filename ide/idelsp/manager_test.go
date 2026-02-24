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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

func TestHandleDoesNotBlockOnInitializeTimeout(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(tmpDir)
	})

	// Create a fake binary that sleeps forever (never speaks LSP).
	fakeBin := filepath.Join(tmpDir, "fake-gopls")
	require.NoError(t, os.WriteFile(fakeBin, []byte("#!/bin/sh\nsleep 3600\n"), 0755))

	uri := makeURI(t, "file://"+tmpDir)
	scheme := newTestScheme()

	mgr := New(
		uri, scheme, scheme,
		&stubPkgManager{bin: fakeBin},
		nil, nil,
		Config{
			MaxRetries:         1,
			InitializeTimeout:  200 * time.Millisecond,
			CloseTimeout:       200 * time.Millisecond,
			EventHandleTimeout: 500 * time.Millisecond,
			NoInitializeServer: true,
		},
	)
	t.Cleanup(func() { _ = mgr.Close() })

	// Initialize with the fake binary via InitializeOptions.
	// This exercises the start → initialize timeout → Close path
	// without touching the global langConfigs map.
	params := autoInitParams(uri.String())
	initOpts, err := json.Marshal(map[string]any{
		"langID":  "go",
		"command": fakeBin,
	})
	require.NoError(t, err)
	params.InitializeOptions = initOpts

	timeout, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	initDone := make(chan error, 1)
	go func() {
		_, err := mgr.Initialize(t.Context(), params)
		initDone <- err
	}()

	select {
	case err := <-initDone:
		require.ErrorContains(t, err, "context deadline exceeded")
	case <-timeout.Done():
		t.Fatal("Initialize blocked: deadlock detected")
	}

	// Verify Handle still drains events without blocking
	// (handleEvs goroutine remains alive after the failed init).
	mainPath := filepath.Join(tmpDir, "main.go")
	require.NoError(t, os.WriteFile(mainPath, []byte("package main\n"), 0644))
	mainURI, err := workspaceapi.ParseURI("file://" + mainPath)
	require.NoError(t, err)

	handleDone := make(chan struct{})
	go func() {
		defer close(handleDone)
		for i := 0; i < 10; i++ {
			shutdown := mgr.Handle(t.Context(), textapi.Event{
				Type:    textapi.EventTypeOpen,
				URI:     mainURI,
				Content: "package main\n",
			})
			assert.False(t, shutdown)
		}
	}()

	select {
	case <-handleDone:
	case <-timeout.Done():
		t.Fatal("Handle blocked: deadlock detected")
	}
}
