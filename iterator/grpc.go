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
	"io"
)

// Stream abstract a subset of grpc.ClientStream.
type Stream interface {
	// RecvMsg blocks until it receives a message into m or the stream is
	// done. It returns io.EOF when the stream completes successfully. On
	// any other error, the stream is aborted and the error contains the RPC
	// status.
	RecvMsg(m interface{}) error
}

// FromStream takes a Stream which casually resembles a grpc.ClientStream, and
// returns an Iterator of T. The iterator stops when its Close function is returned or
// the underlying stream returns io.EOF or other error. Only non io.EOF errors
// will be surfaced via the returning iterator's Err method.
//
// The given cancel function should be the context canceling function fed to
// the grpc.ClientStream's constructor. This is to ensure that when the
// returned iterator's Close method is called, the stream's resources
// are also cleaned up.
func FromStream[T any](
	ctx context.Context,
	cancel func(),
	stream Stream,
) Iterator[*T] {
	type msg struct {
		data *T
		err  error
	}
	ch := make(chan msg)
	go func() {
		defer close(ch)
		var err error
		for {
			data := new(T)
			err = stream.RecvMsg(data)
			select {
			case ch <- msg{data: data, err: err}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return FromFunc(func(ctx context.Context) (ret *T, ok bool, err error) {
		var m msg
		select {
		case m, ok = <-ch:
			ret = m.data
			err = m.err
			if err == io.EOF {
				ok = false
				err = nil
			}
			return
		case <-ctx.Done():
			err = ctx.Err()
			return
		}
	}, func() error {
		cancel()
		<-ch // wait for clean goroutine to be done
		return nil
	})
}
