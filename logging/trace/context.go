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
package trace

import "context"

// key is an unexported type for keys defined in this package.
// This prevents collisions with keys defined in other packages.
type key int

// idKey is the key for trace.ID values in Contexts. It is
// unexported; clients use user.NewContext and trace.FromContext
// instead of using this key directly.
var idKey key

// NewContext returns a new Context that carries value u.
func NewContext(ctx context.Context, u ID) context.Context {
	return context.WithValue(ctx, idKey, u)
}

// FromContext returns the ID value stored in ctx, if any.
func FromContext(ctx context.Context) (id ID, ok bool) {
	id, ok = ctx.Value(idKey).(ID)
	return
}

// FromContextOrNew returns the ID value stored in ctx.
// If it fails to do so, it will add a new ID to the context.
func FromContextOrNew(ctx context.Context) (ID, context.Context) {
	traceID, ok := FromContext(ctx)
	if !ok {
		traceID = New()
		ctx = NewContext(ctx, traceID)
	}
	return traceID, ctx
}
