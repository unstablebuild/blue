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
