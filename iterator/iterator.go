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

import "io"

// Iterator provides a convenient interface for iterating over
// chunks of structured or unstructured data such as
// a file of newline-delimited lines of text or a set of datastore documents.
type Iterator[T any] interface {
	// Next returns the next element and true or nil and false
	// if there's no more elements in this Iterator. Err should be
	// checked for any errors incurred during the lifecycle of this Iterator.
	// Note that if an error is found, it's up to the implementation as to
	// whether to return false and stop iteration or aggregate errors
	// and return at the end.
	Next() (T, bool)
	// Err returns the first error or an aggreation of the errors
	// encountered by the Iterator.
	Err() error

	io.Closer
}

// Empty returns an empty iterator.
func Empty[T any]() Iterator[T] {
	return FromSlice[T](nil)
}
