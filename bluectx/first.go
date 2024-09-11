// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
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

package bluectx

import (
	"context"
	"sync"
	"time"
)

// First returns a new context that combines parent with ctxs.
// The returned context's Done channel is closed the first Done channel
// of any of the passed ctx closes first. Deadline returns the
// earliest of deadlines if there's any set. Value returns the first value
// found, following the passed order, for the given key.
// Callers must make sure that at least one of the passed contexts is eventually canceled,
// or else this function could leak goroutines.
func First(parent context.Context, ctx ...context.Context) context.Context {
	ctxs := make([]context.Context, 0, 1+len(ctx))
	ctxs = append(ctxs, parent)
	ctxs = append(ctxs, ctx...)

	ret := &first{
		ctxs: ctxs,
		ch:   make(chan struct{}),
	}

	// we don't need to return a cancel function because we rely on
	// the other channels' cancel/deadlines
	ret.wg.Add(len(ret.ctxs))
	for _, ctx := range ret.ctxs {
		// best effort check to prevent a goroutine leak if none of the contexts
		// passed are ever canceled. This doesn't work if any of the contexts are
		// non background nor TODO (i.e. implement special Value setting, etc.)
		if ctx == context.Background() || ctx == context.TODO() {
			ret.wg.Done()
			continue
		}
		go func(ctx context.Context) {
			defer ret.wg.Done()
			select {
			case <-ctx.Done():
				// if somehow to context closed at once, make sure that
				// we do not close ch twice
				ret.mu.Lock()
				defer ret.mu.Unlock()
				if ret.done {
					return
				}
				ret.done = true
				ret.err = ctx.Err()
				close(ret.ch)
			case <-ret.ch:
			}
		}(ctx)
	}
	return ret
}

type first struct {
	mu   sync.Mutex
	wg   sync.WaitGroup
	ctxs []context.Context
	err  error
	done bool
	ch   chan struct{}
}

func (f *first) Value(key any) any {
	for _, ctx := range f.ctxs {
		ret := ctx.Value(key)
		if ret != nil {
			return ret
		}
	}
	return nil
}

func (f *first) Deadline() (deadline time.Time, ok bool) {
	for _, ctx := range f.ctxs {
		d, dok := ctx.Deadline()
		if !dok {
			continue
		}
		if d.Before(deadline) || !ok {
			ok = true
			deadline = d
		}
	}
	return
}

func (f *first) Done() <-chan struct{} {
	// make sure that if cancel is called in the calling goroutine
	// we return a valid result.
	for _, ctx := range f.ctxs {
		select {
		case <-ctx.Done():
			// wait for the f.ch to be closed
			f.wg.Wait()
			return f.ch
		default:
		}
	}
	return f.ch
}

func (f *first) Err() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}
