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
