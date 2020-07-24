package gps

import "sync"

type testingReceiver struct {
	lock      sync.Mutex
	onClose   func(ConnectionMetadata)
	onOpen    func(ConnectionMetadata)
	onReceive func(ConnectionMetadata, Coordinates)
}

func (r *testingReceiver) OnOpen(meta ConnectionMetadata) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.onOpen(meta)
}

func (r *testingReceiver) Receive(meta ConnectionMetadata, pos Coordinates) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.onReceive(meta, pos)
}

func (r *testingReceiver) OnClose(meta ConnectionMetadata) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.onClose(meta)
}
