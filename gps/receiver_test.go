package gps

import (
	"context"
	"sync"
)

type testingReceiver struct {
	lock      sync.Mutex
	onReceive func(context.Context, Coordinates) error
}

func (r *testingReceiver) Receive(ctx context.Context, pos Coordinates) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	return r.onReceive(ctx, pos)
}
