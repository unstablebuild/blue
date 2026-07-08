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
