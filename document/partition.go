package document

import (
	"context"
	"fmt"
	"runtime"
)

// WithPartition wraps a service and creates a partition with the given name.
// All the records created, stored, listed, etc. won't be seen by other partitions
// over the same Service, created by this function.
func WithPartition(other Service, partition string) Service {
	c := new(partitionedService)
	c.other = other
	c.partition = partition
	runtime.SetFinalizer(c, func(c *partitionedService) { c.Close() })
	return c
}

const partitionField = "__partition"

type partitionedService struct {
	other     Service
	partition string
}

func (c *partitionedService) makePartitionID(ID string) string {
	return fmt.Sprintf("%s.%s", c.partition, ID)
}

func (c *partitionedService) setPartitionField(ctx context.Context, ID string) error {
	// enable list to filter document of this partition only
	updates := []Update{{FieldPath: []string{partitionField}, Value: c.partition}}
	if err := c.other.Update(ctx, ID, updates); err != nil {
		_ = c.other.Delete(ctx, ID) // best effort
		return err
	}
	return nil
}

func (c *partitionedService) Create(ctx context.Context, ID string, doc interface{}) error {
	ID = c.makePartitionID(ID)
	if err := c.other.Create(ctx, ID, doc); err != nil {
		return err
	}
	return c.setPartitionField(ctx, ID)
}

func (c *partitionedService) Set(ctx context.Context, ID string, doc interface{}) error {
	ID = c.makePartitionID(ID)
	if err := c.other.Set(ctx, ID, doc); err != nil {
		return err
	}
	return c.setPartitionField(ctx, ID)
}

func (c *partitionedService) Update(ctx context.Context, ID string,
	updates []Update, precond ...Precondition) error {
	ID = c.makePartitionID(ID)
	return c.other.Update(ctx, ID, updates, precond...)
}

func (c *partitionedService) Get(ctx context.Context, ID string, doc interface{}) error {
	ID = c.makePartitionID(ID)
	return c.other.Get(ctx, ID, doc)
}

func (c *partitionedService) Delete(ctx context.Context, ID string) error {
	ID = c.makePartitionID(ID)
	return c.other.Delete(ctx, ID)
}

func (c *partitionedService) List(
	ctx context.Context, filters []Filter,
) (Iterator, error) {
	partFilter := Filter{
		Op: OpEqual,
		Field: Field{
			FieldPath: []string{partitionField},
			Value:     c.partition,
		},
	}
	filters = append(filters, partFilter)
	return c.other.List(ctx, filters)
}

func (c *partitionedService) Close() error {
	return c.other.Close()
}
