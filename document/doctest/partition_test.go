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

package doctest

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/ernestrc/go-multierror"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/docmarshal"
	"github.com/unstablebuild/blue/document/docmarshal/docbson"
	"github.com/unstablebuild/blue/document/docmarshal/docjson"
	"github.com/unstablebuild/blue/document/docmarshal/doctoml"
)

func TestPartitionService(t *testing.T) {
	tsuite := []struct {
		encoding  string
		marshaler docmarshal.Marshaler
	}{
		{"bson", docbson.Marshaler()},
		{"json", docjson.Marshaler()},
		{"toml", doctoml.Marshaler()},
	}
	for _, tcase := range tsuite {
		t.Run("a single, default partition", func(t *testing.T) {
			t.Run(tcase.encoding, func(t *testing.T) {
				TestDocumentService(t, func(t *testing.T) document.Service {
					other := document.NewInMemoryServiceWithMarshaler(tcase.marshaler)
					return document.WithPartition(other, "default")
				})
			})
		})

		t.Run("multiple partitions", func(t *testing.T) {
			t.Run(tcase.encoding, func(t *testing.T) {
				other := document.NewInMemoryServiceWithMarshaler(tcase.marshaler)
				var i int
				TestDocumentService(t, func(t *testing.T) document.Service {
					i++
					return document.WithPartition(other, fmt.Sprintf("%dth", i))
				})
			})
		})

		t.Run("nested partitions", func(t *testing.T) {
			t.Run(tcase.encoding, func(t *testing.T) {
				TestDocumentService(t, func(t *testing.T) document.Service {
					other := document.NewInMemoryServiceWithMarshaler(tcase.marshaler)
					return document.WithPartition(
						document.WithPartition(other, "lower"),
						"upper")
				})
			})
		})

		t.Run("same partition different instances", func(t *testing.T) {
			t.Run(tcase.encoding, func(t *testing.T) {
				TestDocumentService(t, func(t *testing.T) document.Service {
					underlying := document.NewInMemoryServiceWithMarshaler(tcase.marshaler)
					a := document.WithPartition(underlying, "default")
					b := document.WithPartition(underlying, "default")
					return &abTester{a: a, b: b}
				})
			})
		})
	}

	// simple tests to make debuggin easier, but complete test is above in "multiple partitions"
	t.Run("records created by one are not seend by another", func(t *testing.T) {
		other := document.NewInMemoryService()
		one := document.WithPartition(other, "one")
		two := document.WithPartition(other, "two")

		type myStruct struct {
			A string
		}
		ctx := context.Background()
		require.NoError(t, one.Create(ctx, "myID", myStruct{A: "a"}))
		require.NoError(t, two.Create(ctx, "myID", myStruct{A: "a"}))

		it, err := one.List(ctx, nil)
		require.NoError(t, err)
		assertListResults(t, it, 1)

		it2, err := two.List(ctx, nil)
		require.NoError(t, err)
		assertListResults(t, it2, 1)
	})
}

type abTester struct {
	a document.Service
	b document.Service
	i atomic.Int64
}

func (ab *abTester) Create(ctx context.Context, ID string, doc interface{}) error {
	if ab.i.Add(1)%2 == 0 {
		return ab.a.Create(ctx, ID, doc)
	}
	return ab.b.Create(ctx, ID, doc)
}

func (ab *abTester) Set(ctx context.Context, ID string, doc interface{}) error {
	if ab.i.Add(1)%2 == 0 {
		return ab.a.Set(ctx, ID, doc)
	}
	return ab.b.Set(ctx, ID, doc)
}

func (ab *abTester) Update(ctx context.Context, ID string,
	updates []document.Update, precond ...document.Precondition) error {
	if ab.i.Add(1)%2 == 0 {
		return ab.a.Update(ctx, ID, updates, precond...)
	}
	return ab.b.Update(ctx, ID, updates, precond...)
}

func (ab *abTester) Get(ctx context.Context, ID string, doc interface{}) error {
	if ab.i.Add(1)%2 == 0 {
		return ab.a.Get(ctx, ID, doc)
	}
	return ab.b.Get(ctx, ID, doc)
}

func (ab *abTester) Delete(ctx context.Context, ID string) error {
	if ab.i.Add(1)%2 == 0 {
		return ab.a.Delete(ctx, ID)
	}
	return ab.b.Delete(ctx, ID)
}

func (ab *abTester) List(
	ctx context.Context, filters []document.Filter,
) (document.Iterator, error) {
	if ab.i.Add(1)%2 == 0 {
		return ab.a.List(ctx, filters)
	}
	return ab.b.List(ctx, filters)
}

func (ab *abTester) Close() (ret error) {
	if err := ab.a.Close(); err != nil {
		ret = multierror.Append(ret, err)
	}
	if err := ab.b.Close(); err != nil {
		ret = multierror.Append(ret, err)
	}
	return
}
