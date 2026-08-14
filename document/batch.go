// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2026 Unstable Build, All Rights Reserved.
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


package document

import "context"

// BatchOpType selects which write a BatchOp performs.
type BatchOpType int

const (
	// BatchCreate mirrors Service.Create.
	BatchCreate BatchOpType = iota
	// BatchSet mirrors Service.Set.
	BatchSet
	// BatchUpdate mirrors Service.Update.
	BatchUpdate
	// BatchDelete mirrors Service.Delete.
	BatchDelete
)

// BatchOp is a single write of a batch. Doc is only read by BatchCreate
// and BatchSet; Updates and Preconditions only by BatchUpdate.
type BatchOp struct {
	Type          BatchOpType
	ID            string
	Doc           any
	Updates       []Update
	Preconditions []Precondition
}

// BatchOpResult reports the outcome of the BatchOp at the same index.
type BatchOpResult struct {
	// Err is one of ErrAlreadyExists, ErrNotFound or
	// ErrPreconditionFailed when the operation was rejected by the
	// store, and nil when it was applied.
	Err error
}

// BatchWriter is implemented by services that can apply several writes
// in a single atomic round trip.
type BatchWriter interface {
	// ApplyBatch applies every op to the same collection. Operations
	// rejected for document-level reasons (a Create over an existing
	// document, an Update whose preconditions fail) do not abort the
	// batch: their error is reported in the result at the same index
	// and the remaining ops are still applied. Any other failure aborts
	// the batch, leaves the store untouched, and is returned as the
	// call error.
	ApplyBatch(ctx context.Context, ops []BatchOp) ([]BatchOpResult, error)
}
