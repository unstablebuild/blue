package document

import (
	"context"
	"testing"
	"time"

	"github.com/ernestrc/blue/retry"
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
