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
	"testing"
	"time"

	"github.com/unstablebuild/blue/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testStruct struct {
	Queries   []string
	UpdatedAt time.Time
}

func TestConsistentUpdate(t *testing.T) {
	ctx := context.Background()
	svc := NewInMemoryService()
	a := testStruct{Queries: []string{"A"}, UpdatedAt: time.Now()}
	require.NoError(t, svc.Create(ctx, "docID", &a))

	t1 := time.Now().Add(1 * time.Minute)
	b := testStruct{Queries: []string{"B"}, UpdatedAt: t1}

	err := ConsistentUpdate(ctx, svc, "docID", &b, retry.LimitStrategy(2),
		func() ([]Update, []Precondition) {
			return []Update{
					{
						FieldPath: []string{"Queries"},
						Value:     append(b.Queries, "C"),
					},
				}, []Precondition{
					{
						FieldPath: []string{"UpdatedAt"},
						Value:     b.UpdatedAt,
					},
				}
		},
	)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"A", "C"}, b.Queries)

}
