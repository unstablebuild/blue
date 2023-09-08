package issue

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/iterator"
	"github.com/ernestrc/blue/retry"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

type documentType uint8

const (
	// this can be used for versioning.
	documentTypeReport documentType = iota

	// ReportMetadataIDField represents the name of the debug.Report.Metadata field used
	// to store the document ID.
	ReportMetadataIDField          = "_id"
	reportMetadataIssueNumberField = "_in"

	autoIncrementMaxRetries uint = 1000
)

var (
	// something must be wrong if we try 10 times and fail
	autoIncrementRetryStrategy = retry.LimitStrategy(autoIncrementMaxRetries)
)

type documentTracker struct {
	db              document.Service
	lastIssueNumber map[string]int
}

// ReportDocument is the in-storage representation of a Report.
type ReportDocument struct {
	Type   documentType
	Report Report
}

func (r ReportDocument) UpdatedTime() time.Time {
	return r.Report.UpdatedAt
}

func (r ReportDocument) WithUpdatedTime(now time.Time) ReportDocument {
	r.Report.UpdatedAt = now
	return r
}

func (r ReportDocument) ID() string {
	id, _ := r.Report.Metadata[ReportMetadataIDField]
	return id
}

func (r ReportDocument) WithID(id string) ReportDocument {
	// clone metadata
	m := make(map[string]string)
	for k, v := range r.Report.Metadata {
		m[k] = v
	}
	r.Report.Metadata = m
	r.Report.Metadata[ReportMetadataIDField] = id
	return r
}

// NewDocumentTracker returns a Tracker backed by a document.Service.
func NewDocumentTracker(db document.Service) Tracker {
	ret := new(documentTracker)
	ret.db = db
	ret.lastIssueNumber = make(map[string]int)
	return ret
}

func (d *documentTracker) fetchLastIssueNumber(ctx context.Context, pkg string) (int, error) {
	// NOTE if the number of issues is ever large, this could be a bit smarter by
	// trying to sort limit 1. It would require adding sorting capability to document.Service.List.
	// It only happens once the service is started.
	it, err := d.ListPackageReports(ctx, pkg, nil)
	if err != nil {
		return 0, fmt.Errorf("fetch last issue number: %v", err)
	}

	var maxIssueNumber int
	for {
		report, ok := it.Next()
		if !ok {
			if err := it.Err(); err != nil {
				return 0, err
			}
			break
		}
		str, ok := report.Metadata[reportMetadataIssueNumberField]
		if !ok {
			log.Warningf("Found report with no metadata issue number: %#v", report.Metadata)
			continue
		}
		issueNumber, err := strconv.Atoi(str)
		if err != nil {
			log.Warningf("could not parse issue number from metadata: %#v", report.Metadata)
			continue
		}
		if issueNumber > maxIssueNumber {
			maxIssueNumber = issueNumber
		}
	}
	d.lastIssueNumber[pkg] = maxIssueNumber
	log.Debugf("Found max issue number for package %q to be %d", pkg, maxIssueNumber)
	return maxIssueNumber, nil
}

// CreateReport  stores the given report into the underlying document.Service. It also appends
// it to the underlyin Package and Bundle Reports field.
func (d *documentTracker) CreateReport(
	ctx context.Context, report Report,
) (string, error) {
	if report.Package == "" ||
		report.Author == "" ||
		report.Subject == "" {
		return "", errors.New("invalid report: missing package, author, subject")
	}
	if report.Metadata == nil {
		report.Metadata = make(map[string]string)
	}
	if report.CreatedAt.IsZero() {
		report.CreatedAt = time.Now()
	}
	report.UpdatedAt = time.Now()

	lastIssueNumber, ok := d.lastIssueNumber[report.Package]
	if !ok {
		// initialize last issue ptr
		var err error
		lastIssueNumber, err = d.fetchLastIssueNumber(ctx, report.Package)
		if err != nil {
			return "", err
		}
	}

	var id string
	err := retry.Retry(ctx, autoIncrementRetryStrategy, func(ctx context.Context) (bool, error) {
		lastIssueNumber++
		d.lastIssueNumber[report.Package] = lastIssueNumber

		id = makeID(report.Package, lastIssueNumber)
		report.Metadata[ReportMetadataIDField] = id
		report.Metadata[reportMetadataIssueNumberField] = strconv.Itoa(lastIssueNumber)

		p := ReportDocument{
			Type:   documentTypeReport,
			Report: report,
		}
		err := d.db.Create(ctx, id, p)
		return err == document.ErrAlreadyExists, err
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

func (d *documentTracker) list(ctx context.Context, filters []document.Filter) (
	iterator.Iterator[Report], error,
) {
	logrus.Debugf("calling List with filters: %#v", filters)

	it, err := d.db.List(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("document.Service.List: %v", err)
	}

	docIter := iterator.FromDocumentIterator[ReportDocument](it)
	reportIter := iterator.Map(docIter, func(doc ReportDocument) Report {
		return doc.Report
	})
	return reportIter, nil
}

// ListVersionReports returns all the reports stored for a given release.
func (d *documentTracker) ListVersionReports(
	ctx context.Context, pkg, ver string, filters map[string]string,
) (iterator.Iterator[Report], error) {
	docFilters := makeReportFilters(pkg, filters)
	docFilters = addVersionFilter(docFilters, ver)
	return d.list(ctx, docFilters)
}

// ListPackageReports returns all the reports stored for a given package.
func (d *documentTracker) ListPackageReports(
	ctx context.Context, pkg string, filters map[string]string,
) (iterator.Iterator[Report], error) {
	docFilters := makeReportFilters(pkg, filters)
	return d.list(ctx, docFilters)
}

func (m *documentTracker) GetReport(ctx context.Context, id string) (Report, error) {
	var doc ReportDocument
	err := m.db.Get(ctx, id, &doc)
	if err != nil {
		return Report{}, err
	}
	return doc.Report, nil
}

func (d *documentTracker) DeleteReport(ctx context.Context, id string) error {
	var doc ReportDocument
	err := d.db.Get(ctx, id, &doc)
	if err != nil {
		return err
	}
	return d.db.Delete(ctx, id)
}

func (d *documentTracker) CloseReport(ctx context.Context, id string) error {
	now := time.Now()
	updates := []document.Update{
		{FieldPath: []string{"Report", "Closed"}, Value: true},
		{FieldPath: []string{"Report", "ClosedAt"}, Value: now},
		{FieldPath: []string{"Report", "UpdatedAt"}, Value: now},
	}
	err := d.db.Update(ctx, id, updates)
	if err != nil {
		if err == document.ErrNotFound {
			err = fmt.Errorf("Issue %q does not exist", id)
		}
		return err
	}
	return nil
}

func (d *documentTracker) UpdateReport(
	ctx context.Context, id string, report Report,
) error {
	if report.Package == "" ||
		report.Author == "" ||
		report.Subject == "" {
		return errors.New("invalid report: missing package, author, subject")
	}
	if report.Metadata == nil {
		report.Metadata = make(map[string]string)
	}
	now := time.Now()
	report.UpdatedAt = now
	// user is being naughty, prevent zero created_at field
	if report.CreatedAt.IsZero() {
		report.CreatedAt = now
	}
	if !report.Closed {
		report.ClosedAt = time.Time{}
	} else if report.ClosedAt.IsZero() {
		report.ClosedAt = now
	}

	lastIssueNumber, ok := parseID(id)
	if !ok {
		return errors.New("invalid request: extraneous issue id")
	}

	// make sure these fields are always present, even if client didn't
	// include them in report request
	report.Metadata[ReportMetadataIDField] = id
	report.Metadata[reportMetadataIssueNumberField] = strconv.Itoa(lastIssueNumber)

	p := ReportDocument{
		Type:   documentTypeReport,
		Report: report,
	}
	return d.db.Set(ctx, id, p)
}

func addVersionFilter(ret []document.Filter, ver string) []document.Filter {
	ret = append(ret,
		document.Filter{
			Field: document.Field{
				FieldPath: []string{"Report", "Version"},
				Value:     ver,
			},
			Op: document.OpEqual,
		})
	return ret
}

func makeReportFilters(
	pkg string, userFilters map[string]string,
) []document.Filter {
	if _, ok := userFilters["Closed"]; !ok {
		if userFilters == nil {
			userFilters = make(map[string]string)
		}
		userFilters["Closed"] = "false"
	}
	ret := []document.Filter{
		{
			Field: document.Field{
				FieldPath: []string{"Type"},
				Value:     documentTypeReport,
			},
			Op: document.OpEqual,
		},
		{
			Field: document.Field{
				FieldPath: []string{"Report", "Package"},
				Value:     pkg,
			},
			Op: document.OpEqual,
		},
	}
	for k, v := range userFilters {
		ks := strings.Split(k, ".")
		// convert string to other values
		path := append([]string{"Report"}, ks...)
		filter := document.Filter{
			Field: document.Field{
				FieldPath: path,
				Value:     v,
			},
			Op: document.OpEqual,
		}
		// best effort convert string Closed value to bool
		if len(ks) == 1 && ks[0] == "Closed" {
			switch v {
			case "True", "true", "TRUE":
				filter.Field.Value = true
			case "False", "false", "FALSE":
				filter.Field.Value = false
			}
		}
		ret = append(ret, filter)
	}
	return ret
}

func parseID(id string) (int, bool) {
	tokens := strings.Split(id, "-")
	if len(tokens) != 2 {
		return 0, false
	}
	seq, err := strconv.Atoi(tokens[1])
	if err != nil {
		return 0, false
	}
	return seq, true
}

func makeID(pkg string, seq int) string {
	return fmt.Sprintf("%s-%d", strings.ToUpper(pkg), seq)
}
