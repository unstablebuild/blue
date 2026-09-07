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

package cliformat

import (
	"context"
	"encoding/json"
	"io"

	"github.com/unstablebuild/blue/iterator"
)

// JSON returns an IteratorFormatter that formats elements into JSON objects.
func JSON[T any]() IteratorFormatter[T] {
	return jsonFormatter[T]{}
}

type jsonFormatter[T any] struct {
}

func (j jsonFormatter[T]) Format(
	ctx context.Context, w io.Writer, it iterator.Iterator[T],
) error {
	e := json.NewEncoder(w)
	for {
		t, ok := it.Next(ctx)
		if !ok {
			if err := it.Err(); err != nil {
				return err
			}
			return nil
		}
		err := e.Encode(t)
		if err != nil {
			return err
		}
	}
}
