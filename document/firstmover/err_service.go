package firstmover

import (
	"context"

	"github.com/unstablebuild/blue/document"
)

type errService struct {
	err error
}

func (e errService) Create(ctx context.Context, ID string, doc interface{}) error {
	return e.err
}

func (e errService) Set(ctx context.Context, ID string, doc interface{}) error {
	return e.err
}

func (e errService) Update(ctx context.Context, ID string, updates []document.Update) error {
	return e.err
}

func (e errService) Get(ctx context.Context, ID string, doc interface{}) error {
	return e.err
}

func (e errService) Delete(ctx context.Context, ID string) error {
	return e.err
}

func (e errService) List(ctx context.Context, filters []document.Filter) (document.Iterator, error) {
	return nil, e.err
}

func (e errService) Close() error {
	return nil
}
