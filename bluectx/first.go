// Copyright 2018-2026 Unstable Build, LLC
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

package bluectx

import (
	"context"
)

// First returns a new context that combines parent with ctxs.
// The returned context's Done channel is closed the first Done channel
// of any of the passed ctx closes first. Deadline returns the
// earliest of deadlines if there's any set. Value returns the first value
// found, following the passed order, for the given key.
// Callers must make sure that the returned cancel function is eventually called.
func First(parent context.Context, ctxs ...context.Context) (
	context.Context, context.CancelFunc,
) {
	deadline, hasDeadline := parent.Deadline()
	for _, c := range ctxs {
		if d, ok := c.Deadline(); ok {
			if !hasDeadline || d.Before(deadline) {
				deadline, hasDeadline = d, true
			}
		}
	}

	var ctx context.Context
	var cancel context.CancelFunc
	if hasDeadline {
		ctx, cancel = context.WithDeadline(parent, deadline)
	} else {
		ctx, cancel = context.WithCancel(parent)
	}

	stops := make([]func() bool, len(ctxs))
	for i, c := range ctxs {
		stops[i] = context.AfterFunc(c, cancel)
	}

	return mergedCtx{ctx, ctxs}, func() {
		cancel()
		for _, stop := range stops {
			stop()
		}
	}
}

type mergedCtx struct {
	context.Context
	ctxs []context.Context
}

func (m mergedCtx) Value(key any) any {
	if v := m.Context.Value(key); v != nil {
		return v
	}
	for _, c := range m.ctxs {
		if v := c.Value(key); v != nil {
			return v
		}
	}
	return nil
}
