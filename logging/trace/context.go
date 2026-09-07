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
