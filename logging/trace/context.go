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
