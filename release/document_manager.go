package release

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/ernestrc/blue/debug"
	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/go-multierror"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// ErrDataIntegrity is returned when downloaded release data is compromised.
var ErrDataIntegrity = errors.New("data integrity check failed: artifact " +
	"is corrupted or communication channel is compromised")

type documentType int32

const (
	// https://firebase.google.com/docs/firestore/quotas#limits
	maxDocSizeBytes = 1048487 - 64

	documentTypeData documentType = iota
	// package bundle document
	documentTypeReleaseBundle
	// package document
	documentTypePackage
	// panic report document
	documentTypePanicReport

	// PanicReportMetadataIDField represents the name of the PanicReport.Metadata field used
	// to store the document ID.
	PanicReportMetadataIDField = "id"
)

type documentManager struct {
	db document.Service
}

type releaseDocument struct {
	Type       documentType
	Package    string
	Bundle     Bundle
	DataChunks []string
	Checksum   string
}

type packageDocument struct {
	Type    documentType
	Package Package
}

type releaseData struct {
	Type documentType
	Data []byte
}

type panicReportDocument struct {
	Type        documentType
	PanicReport debug.PanicReport
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

func makeChunkID(pkg string, ver Version, i int) string {
	return fmt.Sprintf("%s:%s:chunk:%d", pkg, ver, i)
}

func (d *documentManager) createDataChunks(
	ctx context.Context, m Bundle, r ProgressReader,
) ([]string, string, error) {
	ids := make([]string, 0)
	buffer := make([]byte, maxDocSizeBytes)
	docID := makeReleaseDocID(m.Package, m.Version)

	hasher := sha256.New()
	var totalSize int64
	var totalRead int64
	// progress is best effort
	if stater, ok := r.(interface{ Stat() (os.FileInfo, error) }); ok {
		fi, err := stater.Stat()
		if err == nil {
			totalSize = fi.Size()
		}
	}
	r.Progress(0, totalSize, "bytes")
	for i := 0; ; i++ {
		read, rerr := r.Read(buffer)
		if rerr != nil && rerr != io.EOF {
			rerr = fmt.Errorf("failed to read release data: %v", rerr)
			return nil, "", d.forceRemoveChunks(rerr, docID, ids)
		}
		if read == 0 {
			break
		}
		totalRead += int64(read)
		r.Progress(totalRead, totalSize, "bytes")

		// hash.Hash impls never return an error
		_, _ = hasher.Write(buffer[:read])

		chunkID := makeChunkID(m.Package, m.Version, i)
		chunk := releaseData{
			Type: documentTypeData,
			Data: buffer[:read],
		}
		err := d.db.Set(ctx, chunkID, chunk)
		if err != nil {
			return nil, "", d.forceRemoveChunks(err, docID, ids)
		}
		ids = append(ids, chunkID)

		if rerr == io.EOF {
			break
		}
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))
	return ids, checksum, nil
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
	ctx context.Context, m Package,
) error {
	p := packageDocument{
		Type:    documentTypePackage,
		Package: m,
	}
	packageDocID := makePackageDocID(m.Name)
	err := d.db.Set(ctx, packageDocID, p)
	if err != nil {
		return err
	}
	return nil
}

func makeReleaseDocID(pkg string, ver Version) string {
	return fmt.Sprintf("release:%s:%s", pkg, ver)
}

func makePackageDocID(pkg string) string {
	return fmt.Sprintf("package:%s", pkg)
}

func (d *documentManager) Upload(
	ctx context.Context, m Bundle, r ProgressReader,
) error {
	id := makeReleaseDocID(m.Package, m.Version)
	doc := releaseDocument{
		Type:       documentTypeReleaseBundle,
		Package:    m.Package,
		Bundle:     m,
		DataChunks: nil,
	}
	err := d.db.Create(ctx, id, &doc)
	if err != nil {
		return err
	}

	ids, checksum, err := d.createDataChunks(ctx, m, r)
	if err != nil {
		return d.forceDelete(err, id)
	}

	doc.DataChunks = ids
	doc.Checksum = checksum
	err = d.db.Set(ctx, id, &doc)
	if err != nil {
		err = d.forceDelete(err, id)
		return d.forceRemoveChunks(err, id, ids)
	}

	// update latest version of package structure
	// if it doesn't exist, then create one
	p := packageDocument{
		Type: documentTypePackage,
		Package: Package{
			Name:   m.Package,
			Latest: m.Version,
		},
	}
	packageDocID := makePackageDocID(m.Package)
	err = d.db.Set(ctx, packageDocID, p)
	if err != nil {
		return d.forceDelete(err, id)
	}

	return nil
}

func (d *documentManager) writeChunks(
	ctx context.Context, doc releaseDocument, out ProgressWriter,
) error {
	var dataDoc releaseData
	hasher := sha256.New()

	out.Progress(0, int64(len(doc.DataChunks)), "chunks")
	for i, chunkID := range doc.DataChunks {
		err := d.db.Get(ctx, chunkID, &dataDoc)
		if err != nil {
			return fmt.Errorf("failed to read release data chunk: %v", err)
		}
		_, err = out.Write(dataDoc.Data)
		if err != nil {
			return fmt.Errorf("failed to write release data: %v", err)
		}
		_, _ = hasher.Write(dataDoc.Data)
		out.Progress(int64(i+1), int64(len(doc.DataChunks)), "chunks")
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))
	if checksum != doc.Checksum {
		return ErrDataIntegrity
	}
	return nil
}

func (d *documentManager) Get(
	ctx context.Context, pkg string,
	ver Version, out ProgressWriter,
) (Bundle, error) {
	var doc releaseDocument

	id := makeReleaseDocID(pkg, ver)
	err := d.db.Get(ctx, id, &doc)
	if err != nil {
		return Bundle{}, err
	}

	if out == ioutil.Discard {
		return doc.Bundle, nil
	}

	err = d.writeChunks(ctx, doc, out)
	if err != nil {
		return Bundle{}, err
	}

	return doc.Bundle, nil
}

func (d *documentManager) Delete(
	ctx context.Context, pkg string, ver Version,
) error {
	var doc releaseDocument

	id := makeReleaseDocID(pkg, ver)
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

func makeDocumentBundleFilter(
	pkg string, userFilters map[string]string,
) []document.Filter {
	ret := []document.Filter{
		{
			Field: document.Field{
				FieldPath: []string{"Type"},
				Value:     documentTypeReleaseBundle,
			},
			Op: document.OpEqual,
		},
		{
			Field: document.Field{
				FieldPath: []string{"Package"},
				Value:     pkg,
			},
			Op: document.OpEqual,
		},
	}
	for k, v := range userFilters {
		ret = append(ret,
			document.Filter{
				Field: document.Field{
					FieldPath: []string{"Bundle", "Metadata", k},
					Value:     v,
				},
				Op: document.OpEqual,
			})
	}
	return ret
}

func (d *documentManager) List(
	ctx context.Context, pkg string,
	filters map[string]string,
) (ret []Bundle, err error) {
	var it document.Iterator
	ret = make([]Bundle, 0)

	it, err = d.db.List(ctx, makeDocumentBundleFilter(pkg, filters))
	if err != nil {
		return
	}

	var doc releaseDocument
	for it.HasNext() {
		err = it.NextTo(&doc)
		if err != nil {
			return
		}
		ret = append(ret, doc.Bundle)
	}

	return
}

func makeDocumentPackageFilter(
	userFilters map[string]string,
) []document.Filter {
	ret := []document.Filter{
		{
			Field: document.Field{
				FieldPath: []string{"Type"},
				Value:     documentTypePackage,
			},
			Op: document.OpEqual,
		},
	}
	for k, v := range userFilters {
		ret = append(ret,
			document.Filter{
				Field: document.Field{
					FieldPath: []string{"Bundle", "Metadata", k},
					Value:     v,
				},
				Op: document.OpEqual,
			})
	}
	return ret
}

func (d *documentManager) ListPackages(
	ctx context.Context, filters map[string]string,
) (ret []Package, err error) {
	ret = make([]Package, 0)

	var it document.Iterator
	it, err = d.db.List(ctx, makeDocumentPackageFilter(filters))
	if err != nil {
		return
	}

	var doc packageDocument
	for it.HasNext() {
		err = it.NextTo(&doc)
		if err != nil {
			return
		}
		ret = append(ret, doc.Package)
	}

	return
}

func (d *documentManager) DeletePackage(
	ctx context.Context, pkg string,
) error {
	id := makePackageDocID(pkg)
	_, err := d.GetPackage(ctx, pkg)
	if err != nil {
		return err
	}
	return d.db.Delete(ctx, id)
}

func (d *documentManager) GetPackage(
	ctx context.Context, pkg string,
) (Package, error) {
	var doc packageDocument

	id := makePackageDocID(pkg)
	err := d.db.Get(ctx, id, &doc)
	if err != nil {
		return Package{}, err
	}

	return doc.Package, nil
}

// AddPanicReport stores the given report into the underlying document.Service. It also appends
// it to the underlyin Package and Bundle Reports field.
func (d *documentManager) AddPanicReport(ctx context.Context, report debug.PanicReport) error {
	if report.Metadata == nil {
		report.Metadata = make(map[string]string)
	}
	id := uuid.New().String()
	report.Metadata[PanicReportMetadataIDField] = id

	p := panicReportDocument{
		Type:        documentTypePanicReport,
		PanicReport: report,
	}
	err := d.db.Create(ctx, id, p)
	if err != nil {
		return fmt.Errorf("document.Service.Create: %v", err)
	}
	return nil
}

// ListPanicReports returns all the reports stored for a given release.
func (d *documentManager) ListPanicReports(
	ctx context.Context, pkg, ver string, filters map[string]string,
) ([]debug.PanicReport, error) {
	it, err := d.db.List(ctx, makePanicReportFilters(pkg, ver, filters))
	if err != nil {
		return nil, fmt.Errorf("document.Service.Create: %v", err)
	}
	defer it.Close()

	var ret []debug.PanicReport
	var temp panicReportDocument
	for it.HasNext() {
		if nextErr := it.NextTo(&temp); nextErr != nil {
			err = multierror.Append(err, nextErr)
			continue
		}
		ret = append(ret, temp.PanicReport)
	}
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (m *documentManager) GetPanicReport(ctx context.Context, id string) (debug.PanicReport, error) {
	var doc panicReportDocument
	err := m.db.Get(ctx, id, &doc)
	if err != nil {
		return debug.PanicReport{}, err
	}
	return doc.PanicReport, nil
}

func makePanicReportFilters(
	pkg, ver string, userFilters map[string]string,
) []document.Filter {
	ret := []document.Filter{
		{
			Field: document.Field{
				FieldPath: []string{"Type"},
				Value:     documentTypePanicReport,
			},
			Op: document.OpEqual,
		},
		{
			Field: document.Field{
				FieldPath: []string{"PanicReport", "Package"},
				Value:     pkg,
			},
			Op: document.OpEqual,
		},
		{
			Field: document.Field{
				FieldPath: []string{"PanicReport", "Version"},
				Value:     ver,
			},
			Op: document.OpEqual,
		},
	}
	for k, v := range userFilters {
		ret = append(ret,
			document.Filter{
				Field: document.Field{
					FieldPath: []string{"PanicReport", k},
					Value:     v,
				},
				Op: document.OpEqual,
			})
	}
	return ret
}
