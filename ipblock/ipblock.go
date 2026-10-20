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

// Package ipblock blocks IP addresses and address ranges.
//
// A blocklist has two sides, and callers depend only on the one they
// use. The command side, Writer, adds and lifts blocks. The query
// side, Reader, answers whether an address is blocked and lists the
// blocks in force. A List is both sides over one store:
//
//   - NewService keeps blocks in a document.Service, such as
//     a Firestore collection, so every process using the collection
//     shares them.
//   - NewMemoryList keeps them in process memory.
//
// Checking a shared store for every address would cost a round trip
// each, so NewCache serves a Reader from memory, reloading it from
// another Reader once per TTL. That is the query side a hot path
// should hold.
//
// A block covers a prefix; a single address is a /32 or /128 prefix.
// IPv4-mapped IPv6 addresses are the IPv4 addresses they map. A block
// may lapse on its own at Block.ExpiresAt, after which it is neither
// matched nor listed, whether or not its storage has been cleaned up.
package ipblock

import (
	"context"
	"errors"
	"net/netip"
	"time"
)

// Block blocks every address in a prefix.
type Block struct {
	// Prefix is the blocked range. Writers store it with its host bits
	// masked off, and IPv4-mapped prefixes as IPv4.
	Prefix netip.Prefix
	// Reason says why the prefix is blocked, for whoever reads it later.
	Reason string
	// BlockedBy identifies who blocked it.
	BlockedBy string
	// BlockedAt is when it was blocked. Writers set it to the current
	// time when it is zero.
	BlockedAt time.Time
	// ExpiresAt is when the block lapses. The zero time means never.
	ExpiresAt time.Time
}

// Writer is the command side of a blocklist.
type Writer interface {
	// Block blocks b.Prefix until b.ExpiresAt. It replaces any block on
	// the same prefix, so blocking again is how a block is extended or
	// reworded. It fails when the prefix is invalid or ExpiresAt is set
	// but not in the future.
	Block(ctx context.Context, b Block) error

	// Unblock lifts the block on exactly p. Blocks on prefixes that
	// contain p, or that p contains, are left alone. It returns
	// ErrNotBlocked when p has no block in force.
	Unblock(ctx context.Context, p netip.Prefix) error
}

// Reader is the query side of a blocklist.
type Reader interface {
	// Check returns the block covering addr, and false when there is
	// none. When several blocks cover addr the narrowest one is
	// returned.
	Check(ctx context.Context, addr netip.Addr) (Block, bool, error)

	// List returns the blocks in force, ordered by prefix.
	List(ctx context.Context) ([]Block, error)
}

// List is a blocklist: both sides of one store of blocks.
type List interface {
	Writer
	Reader
}

// ErrNotBlocked is returned by Unblock when there is no block to lift.
var ErrNotBlocked = errors.New("ipblock: prefix is not blocked")
