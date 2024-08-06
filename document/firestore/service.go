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

package firestore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/document"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fireStore struct {
	collID string
	client *firestore.Client
}

// New returns an instance of Service backed by GC's FireStore.
//
// The returned service's Update method's preconditions only work with
// document.DefaultUpdatedAtField or document.LowerUpdatedAtField fields.
// See https://cloud.google.com/firestore/docs/reference/rest/v1/Precondition
// for more details.
func New(projectID, collectionID, credsFile string) (
	s document.Service, err error,
) {
	ctx := context.Background()

	// in a GCP runtime or VM, the SDK knows how to fetch credentials.
	// For local development, we need to pass a file manually
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
	if err != nil {
		err = convertError(err)
		return
	}

	doc, err = document.DerefUpdateValue(reflect.ValueOf(doc))
	if err != nil {
		return fmt.Errorf("dereference value for updating UpdatedAt: %w", err)
	}
	// best effort needed for updateTime precondition to
	// work with default updated at fields
	document.UpdateDefaultUpdatedAtField(doc, snapshot.UpdateTime, false)
	document.UpdateDefaultUpdatedAtField(doc, snapshot.UpdateTime, true)
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
		log.Debugf("unknown firestore err %v: code %v", err, status.Code(err))
	}
	return err
}
