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
