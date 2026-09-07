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

package iterator

import (
	"context"

	"github.com/unstablebuild/blue/document"
)

// FromDocumentIterator maps a document.Iterator to an Iterator of type T.
// Items are pulled lazily — nothing is read from the underlying
// document.Iterator until the consumer calls Next.
func FromDocumentIterator[T any](it document.Iterator) Iterator[T] {
	return FromFunc(func(_ context.Context) (ret T, ok bool, err error) {
		if !it.HasNext() {
			return
		}
		err = it.NextTo(&ret)
		if err != nil {
			return
		}
		ok = true
		return
	}, it.Close)
}
