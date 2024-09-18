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

	"github.com/unstablebuild/blue/document"
)

// FromDocumentIterator maps a document.Iterator to an Iterator of type T.
func FromDocumentIterator[T any](it document.Iterator) Iterator[T] {
	type msg struct {
		data T
		err  error
	}
	ch := make(chan msg)
	quitCh := make(chan struct{})
	go func() {
		defer close(ch) // signal ok = false below
		var err error
		var data T
		for {
			if !it.HasNext() {
				return
			}
			err = it.NextTo(&data)
			select {
			case ch <- msg{data: data, err: err}:
			case <-quitCh:
				return
			}
		}
	}()
	return FromFunc(func(ctx context.Context) (ret T, ok bool, err error) {
		var m msg
		select {
		case m, ok = <-ch:
			ret = m.data
			err = m.err
			return
		case <-ctx.Done():
			err = ctx.Err()
			return
		}
	}, func() error {
		close(quitCh)
		err := it.Close()
		<-ch // wait for goroutine to be done
		return err
	})
}
