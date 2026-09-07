// Copyright 2018-2026 Unstable Build, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
