package release

import (
	"context"
	"fmt"
	"io"

	"github.com/ernestrc/blue/datastore/document"
	"github.com/sirupsen/logrus"
)

type documentType int32

const (
	// https://firebase.google.com/docs/firestore/quotas#limits
	maxDocSizeBytes = 1048487 - 64

	documentTypeData documentType = iota
	documentTypeManifest
)

type documentManager struct {
	db document.Service
}

type releaseDocument struct {
	Type       documentType
	Manifest   Manifest
	DataChunks []string
}

type releaseData struct {
	Type documentType
	Data []byte
}

// NewDocumentManager returns a Manager backed by a document.Service.
func NewDocumentManager(db document.Service) Manager {
	ret := new(documentManager)
	ret.db = db
	return ret
}

func makeLeakError(releaseID string, err, rerr error) error {
	return fmt.Errorf("db.Delete error when trying to handle error; leaking release %s in db: %v: %v",
		releaseID, err, rerr)
}

func (d *documentManager) forceDelete(err error, id string) error {
	rerr := d.db.Delete(context.Background(), id)
	if rerr != nil {
		err = makeLeakError(id, err, rerr)
		logrus.WithFields(logrus.Fields{"release": id}).Error(err)
	}
	return err
}

func (d *documentManager) forceRemoveChunks(err error, releaseID string, ids []string) error {
	rerr := d.removeChunks(context.Background(), ids)
	if rerr != nil {
		err = makeLeakError(releaseID, err, rerr)
		logrus.WithFields(logrus.Fields{"release": releaseID}).Error(err)
	}
	return err
}

func (d *documentManager) createDataChunks(
	ctx context.Context, m Manifest, r io.Reader,
) ([]string, error) {
	ids := make([]string, 0)
	buffer := make([]byte, maxDocSizeBytes)

	for i := 0; ; i++ {
		read, rerr := r.Read(buffer)
		if rerr != nil && rerr != io.EOF {
			rerr = fmt.Errorf("failed to read release data: %v", rerr)
			return nil, d.forceRemoveChunks(rerr, m.ID, ids)
		}

		if read == 0 {
			break
		}

		chunkID := fmt.Sprintf("%s:chunk:%d", m.ID, i)
		chunk := releaseData{
			Type: documentTypeData,
			Data: buffer[:read],
		}
		err := d.db.Set(ctx, chunkID, chunk)
		if err != nil {
			return nil, d.forceRemoveChunks(err, m.ID, ids)
		}
		ids = append(ids, chunkID)

		if rerr == io.EOF {
			break
		}
	}

	return ids, nil
}

func (d *documentManager) removeChunks(ctx context.Context, ids []string) error {
	errs := make([]error, 0)
	for _, id := range ids {
		err := d.db.Delete(ctx, id)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return errs[0]
	}

	return nil
}

func (d *documentManager) Create(
	ctx context.Context, m Manifest, r io.Reader,
) error {
	id := m.ID
	doc := releaseDocument{
		Type:       documentTypeManifest,
		Manifest:   m,
		DataChunks: nil,
	}
	err := d.db.Create(ctx, id, &doc)
	if err != nil {
		return err
	}

	ids, err := d.createDataChunks(ctx, m, r)
	if err != nil {
		return d.forceDelete(err, id)
	}

	doc.DataChunks = ids
	err = d.db.Set(ctx, id, &doc)
	if err != nil {
		err = d.forceDelete(err, id)
		return d.forceRemoveChunks(err, id, ids)
	}

	return nil
}

func (d *documentManager) writeChunks(
	ctx context.Context, doc releaseDocument, out io.Writer,
) error {
	var dataDoc releaseData
	for _, chunkID := range doc.DataChunks {
		err := d.db.Get(ctx, chunkID, &dataDoc)
		if err != nil {
			return fmt.Errorf("failed to read release data chunk: %v", err)
		}
		_, err = out.Write(dataDoc.Data)
		if err != nil {
			return fmt.Errorf("failed to write release data: %v", err)
		}
	}
	return nil
}

func (d *documentManager) Get(
	ctx context.Context, id string, out io.Writer,
) (Manifest, error) {
	var doc releaseDocument

	err := d.db.Get(ctx, id, &doc)
	if err != nil {
		return Manifest{}, err
	}

	err = d.writeChunks(ctx, doc, out)
	if err != nil {
		return Manifest{}, err
	}

	return doc.Manifest, nil
}

func (d *documentManager) Delete(ctx context.Context, id string) error {
	var doc releaseDocument

	err := d.db.Get(ctx, id, &doc)
	if err != nil {
		return err
	}

	err = d.removeChunks(ctx, doc.DataChunks)
	if err != nil {
		return err
	}

	return d.db.Delete(ctx, id)
}

func makeDocumentManifestFilter() []document.Filter {
	return []document.Filter{
		document.Filter{
			Field: document.Field{
				FieldPath: []string{"Type"},
				Value:     documentTypeManifest,
			},
			Op: document.OpEqual,
		}}
}

func (d *documentManager) List(ctx context.Context) (ret []Manifest, err error) {
	var it document.Iterator
	ret = make([]Manifest, 0)

	it, err = d.db.List(ctx, makeDocumentManifestFilter())
	if err != nil {
		return
	}

	var doc releaseDocument
	for it.HasNext() {
		err = it.NextTo(&doc)
		if err != nil {
			return
		}
		ret = append(ret, doc.Manifest)
	}

	return
}
