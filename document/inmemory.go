package document

import (
	"context"
	"reflect"
	"sync"
)

type inMemoryService struct {
	m       sync.Locker
	storage map[string][]byte
}

// NewInMemoryService returns an instance of Service backed
// by an in-memory map.
func NewInMemoryService() DroppableService {
	return &inMemoryService{
		m:       new(sync.Mutex),
		storage: make(map[string][]byte),
	}
}

func (c *inMemoryService) Set(
	ctx context.Context, ID string, data interface{},
) error {
	return c.set(ctx, ID, data, false)
}

func (c *inMemoryService) Create(
	ctx context.Context, ID string, data interface{},
) error {
	return c.set(ctx, ID, data, true)
}

func (c *inMemoryService) set(
	ctx context.Context, ID string, data interface{},
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
	c.storage[ID] = Encode(data, true)
	return
}

func (c *inMemoryService) getValue(ID string, doc interface{}) (
	err error,
) {
	var ok bool

	c.m.Lock()
	defer c.m.Unlock()

	var raw []byte
	raw, ok = c.storage[ID]
	if !ok {
		err = ErrNotFound
		return
	}

	return SafeDecode(doc, raw)
}

func (c *inMemoryService) Get(
	ctx context.Context, ID string, to interface{},
) (err error) {
	err = c.getValue(ID, to)
	if err != nil {
		return
	}
	return
}

func (c *inMemoryService) Update(
	ctx context.Context, ID string, updates []Update,
) error {
	if len(updates) == 0 {
		panic("Update: no paths to update")
	}

	var proto map[string]interface{}
	err := c.getValue(ID, &proto)
	if err != nil {
		return err
	}

	UpdateProto(updates, proto)

	c.m.Lock()
	defer c.m.Unlock()

	c.storage[ID] = Encode(proto, false)

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
	iter := NewListIterator()

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
