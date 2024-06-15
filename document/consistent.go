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
package document

import (
	"context"
	"errors"

	"github.com/unstablebuild/blue/retry"
)

// ConsistentUpdate calls Update on the given service and tries to update
// the underlying document with the provided retry strategy and the
// callback function which gets called before attempting to update
// the document in storage. This can be used to scan the updated
// doc for any data that has been updated in the process of performing
// this operation and to update preconditions with the latest values.
//
// The given doc value will be updated so it should be a valid value as
// defined by Service.Get.
func ConsistentUpdate(
	ctx context.Context, svc Service, ID string, doc interface{},
	retryStrategy retry.Strategy, callback func() ([]Update, []Precondition),
) error {
	return retry.Retry(ctx, retryStrategy, func(ctx context.Context) (bool, error) {
		if err := svc.Get(ctx, ID, doc); err != nil {
			return false, err
		}
		updates, preconditions := callback()
		err := svc.Update(ctx, ID, updates, preconditions...)
		if err == nil {
			return false, svc.Get(ctx, ID, doc)
		}
		return errors.Is(err, ErrPreconditionFailed), err
	})
}
