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

package semanticrpc

import (
	"context"
	"sync"
	"time"
)

// joinedCtx is a context whose Done channel closes when either of
// two parent contexts is done first.  Deadline returns the earliest
// deadline (if any), and Value returns the first value found.
type joinedCtx struct {
	a, b context.Context
	done chan struct{}
	once sync.Once
	err  error
}

// joinContexts returns a context that is done when the first of a or b
// is done. The returned cancel function must be called to release
// resources even if one of the parent contexts is already done.
func joinContexts(a, b context.Context) (context.Context, context.CancelFunc) {
	j := &joinedCtx{
		a:    a,
		b:    b,
		done: make(chan struct{}),
	}
	stop := context.AfterFunc(a, func() {
		j.once.Do(func() {
			j.err = a.Err()
			close(j.done)
		})
	})
	stop2 := context.AfterFunc(b, func() {
		j.once.Do(func() {
			j.err = b.Err()
			close(j.done)
		})
	})
	cancel := func() {
		stop()
		stop2()
		j.once.Do(func() {
			j.err = context.Canceled
			close(j.done)
		})
	}
	return j, cancel
}

func (j *joinedCtx) Deadline() (time.Time, bool) {
	da, oka := j.a.Deadline()
	db, okb := j.b.Deadline()
	switch {
	case oka && okb:
		if da.Before(db) {
			return da, true
		}
		return db, true
	case oka:
		return da, true
	case okb:
		return db, true
	default:
		return time.Time{}, false
	}
}

func (j *joinedCtx) Done() <-chan struct{} {
	return j.done
}

func (j *joinedCtx) Err() error {
	select {
	case <-j.done:
		return j.err
	default:
		return nil
	}
}

func (j *joinedCtx) Value(key any) any {
	if v := j.a.Value(key); v != nil {
		return v
	}
	return j.b.Value(key)
}
