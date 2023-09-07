package firestore

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/logging/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fireStore struct {
	collID string
	client *firestore.Client
}

// New returns an instance of Service backed by GC's FireStore.
func New(projectID, collectionID, credsFile string) (
	s document.Service, err error,
) {
	ctx := context.Background()

	// in a GC runtime, the SDK knows how to fetch credentials
	// for local development, we need to pass a file manually
	if credsFile != "" {
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credsFile)
	}

	var client *firestore.Client
	client, err = firestore.NewClient(ctx, projectID)
	if err != nil {
		return
	}
	s = &fireStore{collID: collectionID, client: client}
	return
}

func (f *fireStore) Set(
	ctx context.Context, docID string, data interface{},
) (err error) {
	if data == nil {
		panic("invalid nil data argument to Set")
	}
	coll := f.client.Collection(f.collID)

	_, err = coll.Doc(docID).Set(ctx, data)
	if err != nil {
		err = convertError(err)
	}
	return
}

func (f *fireStore) Create(
	ctx context.Context, docID string, data interface{},
) (err error) {
	if data == nil {
		panic("invalid nil data argument to Create")
	}
	coll := f.client.Collection(f.collID)

	_, err = coll.Doc(docID).Create(ctx, data)
	if err != nil {
		err = convertError(err)
	}
	return
}

func (f *fireStore) Update(
	ctx context.Context, docID string, updates []document.Update,
	preconds ...document.Precondition,
) (err error) {
	if len(updates) == 0 {
		panic("Update: no paths to update")
	}
	coll := f.client.Collection(f.collID)

	updatedAtField := document.DefaultUpdatedAtField

	fUpdates := make([]firestore.Update, 0, len(updates))
	for _, u := range updates {
		if len(u.FieldPath) == 0 {
			panic("empty field path")
		}

		// use firestore.ServerTimestamp to update updated_at
		if u.FieldPath[0] == document.DefaultUpdatedAtField ||
			u.FieldPath[0] == document.LowerUpdatedAtField {
			// use whichever updated at key is used. This best-effort improves
			// integration between rpc and this document.Service,
			// at the same time it's compatible with using firestore directly.
			updatedAtField = u.FieldPath[0]
			continue
		}

		fUpdates = append(fUpdates, firestore.Update{
			FieldPath: firestore.FieldPath(u.FieldPath),
			Value:     u.Value,
		})
	}

	fUpdates = append(fUpdates,
		firestore.Update{
			FieldPath: firestore.FieldPath{updatedAtField},
			Value:     firestore.ServerTimestamp,
		})

	if len(preconds) == 0 {
		_, err = coll.Doc(docID).Update(ctx, fUpdates)
		if err != nil {
			err = convertError(err)
		}
		return
	}

	updatedAt, ok := preconds[0].Value.(time.Time)
	if len(preconds) != 1 || !ok {
		return errors.New("firestore only supports an updated time precondition")
	}

	// timestamps cannot have more than microsecond precision or firestore
	// throws an InvalidArgument
	precond := firestore.LastUpdateTime(updatedAt.Truncate(time.Microsecond))
	_, err = coll.Doc(docID).Update(ctx, fUpdates, precond)
	if err != nil {
		err = convertError(err)
	}
	return
}

func (f *fireStore) Get(
	ctx context.Context, docID string, doc interface{},
) (err error) {
	coll := f.client.Collection(f.collID)

	var snapshot *firestore.DocumentSnapshot
	snapshot, err = coll.Doc(docID).Get(ctx)
	if err != nil {
		err = convertError(err)
		return
	}

	err = snapshot.DataTo(doc)
	return
}

func (f *fireStore) Delete(ctx context.Context, docID string) (
	err error,
) {
	coll := f.client.Collection(f.collID)

	_, err = coll.Doc(docID).Delete(ctx)
	if err != nil {
		err = convertError(err)
	}
	return
}

type fireStoreIterator struct {
	it   *firestore.DocumentIterator
	next *firestore.DocumentSnapshot
}

func (f *fireStoreIterator) HasNext() bool {
	return f.next != nil
}

func (f *fireStoreIterator) NextTo(doc interface{}) error {
	err := f.next.DataTo(doc)
	if err != nil {
		return err
	}
	f.next, err = f.it.Next()
	if err == iterator.Done {
		f.next = nil
		err = nil
	}
	return err
}

func (f *fireStoreIterator) Close() error {
	f.it.Stop()
	return nil
}

func (f *fireStore) List(ctx context.Context, filters []document.Filter) (
	document.Iterator, error,
) {
	coll := f.client.Collection(f.collID)

	query := coll.Offset(0)
	for _, f := range filters {
		if len(f.FieldPath) == 0 || f.Op == "" {
			panic("invalid filter")
		}
		query = query.Where(strings.Join(f.FieldPath, "."), string(f.Op), f.Value)
	}

	firestoreIter := query.Documents(ctx)
	next, err := firestoreIter.Next()
	if err != nil && err != iterator.Done {
		return nil, err
	}

	return &fireStoreIterator{
		it:   firestoreIter,
		next: next,
	}, nil
}

func (f *fireStore) Close() error {
	return f.client.Close()
}

func convertError(err error) error {
	switch status.Code(err) {
	case codes.NotFound:
		err = document.ErrNotFound
	case codes.FailedPrecondition:
		err = document.ErrPreconditionFailed
	case codes.AlreadyExists:
		err = document.ErrAlreadyExists
	case codes.PermissionDenied:
		err = document.ErrPermissionDenied
	default:
		logrus.Warningf("err %s: code %v", err, status.Code(err))
	}
	return err
}

func waitForEmulator(addr string) (err error) {
	const tries = 20
	const backoff = 500 * time.Millisecond
	var conn net.Conn
	for i := 0; i < tries; i++ {
		conn, err = net.Dial("tcp", addr)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(backoff)
	}
	return
}

// RunFirestoreEmulator runs firestore emulator as a subprocess.
//
// It sets FIRESTORE_EMULATOR_HOST so calls to firestore.NewClient
// know that emulator is to be used instead of real Firestore.
//
// Gloud SDK and the emulator need to be installed first.
func RunFirestoreEmulator() (teardown func() error, err error) {
	const addr = "0.0.0.0:8045"
	const callType = "StartFirestoreEmulator"

	cmd := exec.Command("gcloud", "beta", "emulators", "firestore", "start", fmt.Sprintf("--host-port=%s", addr))
	err = cmd.Start()
	if err != nil {
		return
	}

	traceID := trace.New()
	field := logging.Field{Key: "address", Value: addr}
	start := logging.LogAttempt(traceID, callType, field)

	teardown = cmd.Process.Kill
	err = waitForEmulator(addr)
	if err != nil {
		teardown()
	} else {
		os.Setenv("FIRESTORE_EMULATOR_HOST", addr)
	}

	logging.LogResult(err, start, traceID, callType, field)
	return
}
