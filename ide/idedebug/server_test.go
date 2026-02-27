// Copyright 2026 Unstable Build, LLC.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package idedebug

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-dap"
	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

func TestCloseConnLeavesAliveTrue(t *testing.T) {
	t.Parallel()

	c1, c2 := net.Pipe()
	defer c2.Close() //nolint:errcheck

	srv := &debugServer{
		conn:    c1,
		alive:   true,
		pending: make(map[int]chan dap.Message),
	}

	srv.closeConn()

	assert.False(t, srv.alive,
		"closeConn should set alive = false to prevent writes to a closed connection")
}

func TestWatchServerNotRetainsDeadServer(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr := &Manager{
		cfg: Config{
			MaxRetries:        1,
			InitializeTimeout: 500 * time.Millisecond,
			CloseTimeout:      time.Second,
		},
		servers:  make(map[string]*debugServer),
		starting: make(map[string]chan struct{}),
		eventSub: nopEventSubscriber{},
		executor: failingExecutor{},
		ctx:      ctx,
		cancel:   cancel,
		log:      slog.Default(),
	}

	watchCh := make(chan error, 1)
	c1, c2 := net.Pipe()
	defer c2.Close() //nolint:errcheck

	srvCtx, srvCancel := context.WithCancel(ctx)
	defer srvCancel()

	srv := &debugServer{
		ctx:     srvCtx,
		cancel:  srvCancel,
		conn:    c1,
		alive:   true,
		watcher: watchCh,
		cfg:     debugConfig{id: "test-adapter"},
		pending: make(map[int]chan dap.Message),
		log:     slog.With("test", true),
	}

	mgr.mu.Lock()
	mgr.servers["test-adapter"] = srv
	mgr.mu.Unlock()

	// Simulate a server crash.
	watchCh <- errors.New("process crashed")

	done := make(chan struct{})
	go func() {
		mgr.watchServer(
			&debugConfig{id: "test-adapter"},
			srv,
		)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("watchServer did not complete")
	}

	// Bug: watchServer logs the failure but doesn't remove
	// the dead server from m.servers. getOrCreateServer sees
	// the entry and returns it without any health check.
	mgr.mu.Lock()
	_, exists := mgr.servers["test-adapter"]
	mgr.mu.Unlock()

	assert.False(t, exists,
		"dead server should be removed from m.servers after failed retries")
}

// failingExecutor always fails to start commands.
// Used to force retry failure in watchServer tests.
type failingExecutor struct{}

var _ schemeapi.Executor = failingExecutor{}

func (failingExecutor) StartCommand(
	context.Context, workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	return 0, errors.New("executor: forced failure")
}

func (failingExecutor) Signal(
	workspaceapi.Pid, syscall.Signal,
) error {
	return errors.New("not implemented")
}

func (failingExecutor) Close() error {
	return nil
}
