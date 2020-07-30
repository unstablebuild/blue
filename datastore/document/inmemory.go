package document

import (
	"context"
	"reflect"
	"sync"
)

type inMemoryCache struct {
	m       sync.Mutex
	storage map[string][]byte
}

// NewInMemoryCache returns an instance of Service backed
// by an in-memory map.
func NewInMemoryCache() Service {
	return &inMemoryCache{
		storage: make(map[string][]byte),
	}
}

func (c *inMemoryCache) Set(
	ctx context.Context, ID string, data interface{},
) error {
	return c.set(ctx, ID, data, false)
}

func (c *inMemoryCache) Create(
	ctx context.Context, ID string, data interface{},
) error {
	return c.set(ctx, ID, data, true)
}

func (c *inMemoryCache) set(
	ctx context.Context, ID string, data interface{},
	errAlreadyExists bool,
) (err error) {
	if data == nil {
		panic("invalid nil data argument to Create")
	}
	data, err = derefCreateValue(reflect.ValueOf(data))
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
	c.storage[ID] = encode(data, true)
	return
}

func (c *inMemoryCache) getValue(ID string, doc interface{}) (
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

	return safeDecode(doc, raw)
}

func (c *inMemoryCache) Get(
	ctx context.Context, ID string, to interface{},
) (err error) {
	err = c.getValue(ID, to)
	if err != nil {
		return
	}
	return
}

func (c *inMemoryCache) Update(
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

	updateProto(updates, proto)

	c.m.Lock()
	defer c.m.Unlock()

	c.storage[ID] = encode(proto, false)

	return nil
}

func (c *inMemoryCache) Close() error {
	return nil
}

func (c *inMemoryCache) Delete(ctx context.Context, ID string) error {
	c.m.Lock()
	defer c.m.Unlock()

	delete(c.storage, ID)
	return nil
}

func (c *inMemoryCache) List(ctx context.Context, filters []Filter) (
	it Iterator, err error,
) {
	iter := listIterator{docs: make([][]byte, 0)}

	c.m.Lock()
	defer c.m.Unlock()

	for _, v := range c.storage {
		iter.maybeExtend(filters, v)
	}

	return &iter, nil
}
