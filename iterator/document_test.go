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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/docmarshal/docjson"
	"go.uber.org/goleak"
)

func TestDocumentIterator(t *testing.T) {
	goleak.VerifyNone(t)

	t.Run("unblocks Next if context is canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()
		dit := blockingDocumentIterator{quitCh: make(chan struct{})}
		it := FromDocumentIterator[string](dit)
		_, ok := it.Next(ctx)
		assert.False(t, ok)
		assert.Error(t, it.Err())
	})

	t.Run("exhausts underlying document iterator", func(t *testing.T) {
		type bob struct {
			X string
		}
		a, b, c := bob{X: "a"}, bob{X: "b"}, bob{X: "c"}
		dit := document.NewListIterator(docjson.Marshaler(), a, b, c)
		actual, err := Reduce(context.Background(),
			FromDocumentIterator[bob](dit), func(ret []bob, t bob) ([]bob, error) {
				return append(ret, t), nil
			})
		require.NoError(t, err)
		assert.EqualValues(t, []bob{a, b, c}, actual)
	})

	t.Run("bubbles up underlying document iterator errors", func(t *testing.T) {
		dit := FromDocumentIterator[string](&errorDocumentIterator{})
		_, ok := dit.Next(context.Background())
		assert.False(t, ok)
		assert.EqualError(t, dit.Err(), "1 error occurred: kaboom")
	})
}

type blockingDocumentIterator struct {
	quitCh chan struct{}
}

func (b blockingDocumentIterator) HasNext() bool {
	return true
}

func (b blockingDocumentIterator) NextTo(doc interface{}) error {
	<-b.quitCh
	return nil
}

func (b blockingDocumentIterator) Close() error {
	close(b.quitCh)
	return nil
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
