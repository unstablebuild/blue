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

package auth

import "context"

// CombineKeys combines a set of Keys, tipically used in calls to Verify.
// Sign will return the key in k.
func CombineKeys(k Keys, extra ...Keys) Keys {
	keys := make([]Keys, 0, len(extra)+1)
	keys = append(keys, k)
	keys = append(keys, extra...)
	return multiKeys{keys: keys}
}

type multiKeys struct {
	keys []Keys
}

func (m multiKeys) Sign(ctx context.Context) (Key, error) {
	return m.keys[0].Sign(ctx)
}

func (m multiKeys) Verify(ctx context.Context) (ret []Key, err error) {
	for _, keys := range m.keys {
		other, err := keys.Verify(ctx)
		if err != nil {
			return nil, err
		}
		ret = append(ret, other...)
	}
	return
}
