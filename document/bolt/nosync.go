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

package bolt

import "context"

type noSyncKey struct{}

// ContextWithNoSync returns a context that asks bolt-backed stores to
// commit the writes it accompanies without waiting for fsync.
//
// Relaxed commits trade durability for throughput: a synced commit
// costs an fsync (milliseconds on hosts where it maps to a full flush,
// such as macOS), a relaxed one costs microseconds. Process death
// loses nothing — pages sit in the OS cache — but an OS crash or power
// loss may discard commits made since the last synced write, and, as
// with any bbolt NoSync usage, may require recovery from the previous
// meta page. Reserve it for bulk-loading data that can be rebuilt.
//
// Durability is restored by the next write on the same database path
// whose context does not carry the request: its commit fsyncs
// everything before it. Bulk loaders should therefore write their
// completion marker with a plain context, so the marker's durability
// implies the durability of the data it vouches for.
//
// The request is honored per write operation, applies to every store
// sharing the database path, and is invisible to non-bolt
// implementations of document.Service.
func ContextWithNoSync(ctx context.Context) context.Context {
	return context.WithValue(ctx, noSyncKey{}, true)
}

// NoSyncRequested reports whether ctx carries the relaxed-durability
// request installed by ContextWithNoSync.
func NoSyncRequested(ctx context.Context) bool {
	requested, _ := ctx.Value(noSyncKey{}).(bool)
	return requested
}
