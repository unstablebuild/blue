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

package iterator

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/docmarshal/docjson"
	"go.uber.org/goleak"
)

func TestDocumentIterator(t *testing.T) {
	t.Run("double Close is a no-op", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		it := FromDocumentIterator[string](&errorDocumentIterator{})
		_, ok := it.Next(context.Background())
		assert.False(t, ok)
		require.NoError(t, it.Close())
		require.NoError(t, it.Close())
	})

	t.Run("exhausts underlying document iterator", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		type bob struct {
			X string
		}
		a, b, c := bob{X: "a"}, bob{X: "b"}, bob{X: "c"}
		dit := document.NewListIterator(docjson.Marshaler(), a, b, c)
		it := FromDocumentIterator[bob](dit)
		actual, err := Reduce(context.Background(),
			it, func(ret []bob, t bob) ([]bob, error) {
				return append(ret, t), nil
			})
		require.NoError(t, err)
		assert.EqualValues(t, []bob{a, b, c}, actual)
		require.NoError(t, it.Close())
	})

	t.Run("bubbles up underlying document iterator errors", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		it := FromDocumentIterator[string](&errorDocumentIterator{})
		_, ok := it.Next(context.Background())
		assert.False(t, ok)
		assert.EqualError(t, it.Err(), "1 error occurred: kaboom")
		require.NoError(t, it.Close())
	})

	t.Run("does not leak goroutine if data was read, "+
		"but Close was called before Next", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		type bob struct {
			X string
		}
		a := bob{X: "a"}
		dit := document.NewListIterator(docjson.Marshaler(), a)
		it := FromDocumentIterator[bob](dit)
		require.NoError(t, it.Close())
	})
}

type errorDocumentIterator struct {
	closed bool
}

func (b *errorDocumentIterator) HasNext() bool {
	return true
}

func (b *errorDocumentIterator) NextTo(doc interface{}) error {
	return errors.New("kaboom")
}

func (b *errorDocumentIterator) Close() error {
	b.closed = true
	return nil
}
