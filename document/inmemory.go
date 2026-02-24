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
	"reflect"
	"sync"

	"github.com/unstablebuild/blue/document/docmarshal"
	"github.com/unstablebuild/blue/document/docmarshal/docbson"
)

type inMemoryService struct {
	marshaler docmarshal.Marshaler
	m         sync.Locker
	storage   map[string][]byte
}

// NewInMemoryService returns an instance of Service backed
// by an in-memory map.
func NewInMemoryService() DroppableService {
	return NewInMemoryServiceWithMarshaler(docbson.Marshaler())
}

func NewInMemoryServiceWithMarshaler(m docmarshal.Marshaler) DroppableService {
	return &inMemoryService{
		marshaler: m,
		m:         new(sync.Mutex),
		storage:   make(map[string][]byte),
	}
}

func (c *inMemoryService) Set(
	ctx context.Context, ID string, data any,
) error {
	return c.set(ctx, ID, data, false)
}

func (c *inMemoryService) Create(
	ctx context.Context, ID string, data any,
) error {
	return c.set(ctx, ID, data, true)
}

func (c *inMemoryService) set(
	_ context.Context, ID string, data any,
	errAlreadyExists bool,
) (err error) {
	if data == nil {
		panic("invalid nil data argument to Create/Set")
	}
	data, err = DerefCreateValue(reflect.ValueOf(data))
	if err != nil {
		return
	}

	c.m.Lock()
	defer c.m.Unlock()

	if errAlreadyExists {
		_, ok := c.storage[ID]
		if ok {
			return ErrAlreadyExists
		}
	}
	c.storage[ID] = Encode(c.marshaler, data, true)
	return
}

func (c *inMemoryService) getValue(ID string, doc any) (
	err error,
) {
	var ok bool

	var raw []byte
	raw, ok = c.storage[ID]
	if !ok {
		err = ErrNotFound
		return
	}

	return SafeDecode(c.marshaler, doc, raw)
}

func (c *inMemoryService) Get(
	ctx context.Context, ID string, to any,
) (err error) {
	c.m.Lock()
	defer c.m.Unlock()

	err = c.getValue(ID, to)
	return
}

func (c *inMemoryService) Update(
	ctx context.Context, ID string, updates []Update,
	preconds ...Precondition,
) error {
	if len(updates) == 0 {
		panic("Update: no paths to update")
	}

	c.m.Lock()
	defer c.m.Unlock()

	var proto map[string]any
	err := c.getValue(ID, &proto)
	if err != nil {
		return err
	}

	err = UpdateProto(c.marshaler, updates, proto, preconds...)
	if err != nil {
		return err
	}

	c.storage[ID] = Encode(c.marshaler, proto, false)

	return nil
}

func (c *inMemoryService) Close() error {
	return nil
}

func (c *inMemoryService) Delete(ctx context.Context, ID string) error {
	c.m.Lock()
	defer c.m.Unlock()

	delete(c.storage, ID)
	return nil
}

func (c *inMemoryService) List(ctx context.Context, filters []Filter) (
	it Iterator, err error,
) {
	iter := NewListIterator(c.marshaler)

	c.m.Lock()
	defer c.m.Unlock()

	for _, v := range c.storage {
		iter.Extend(filters, v)
	}

	return iter, nil
}

func (c *inMemoryService) Drop(ctx context.Context) error {
	c.m.Lock()
	defer c.m.Unlock()

	c.storage = make(map[string][]byte)
	return nil
}
