package issue

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/retry"
	"github.com/ernestrc/go-multierror"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

type documentType int32

const (
	// panic report document
	documentTypeReport = iota

	// ReportMetadataIDField represents the name of the debug.Report.Metadata field used
	// to store the document ID.
	ReportMetadataIDField          = "_id"
	reportMetadataIssueNumberField = "_in"
)

var (
	// something must be wrong if we try 10 times and fail
	autoIncrementRetryStrategy = retry.LimitStrategy(10)
)

type documentTracker struct {
	db              document.Service
	lastIssueNumber map[string]int
}

type reportDocument struct {
	Type   documentType
	Report Report
}

// NewDocumentTracker returns a Tracker backed by a document.Service.
func NewDocumentTracker(db document.Service) Tracker {
	ret := new(documentTracker)
	ret.db = db
	ret.lastIssueNumber = make(map[string]int)
	return ret
}

func (d *documentTracker) fetchLastIssueNumber(ctx context.Context, pkg string) error {
	// NOTE if the number of issues is ever large, this could be a bit smarter by
	// trying to sort limit 1. It would require adding sorting capability to document.Service.List.
	// It only happens once the service is started.
	reports, err := d.ListPackageReports(ctx, pkg, nil)
	if err != nil {
		return fmt.Errorf("fetch last issue number: %d", err)
	}
	var maxIssueNumber int
	for _, report := range reports {
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
	return nil
}

// AddReport stores the given report into the underlying document.Service. It also appends
// it to the underlyin Package and Bundle Reports field.
func (d *documentTracker) AddReport(ctx context.Context, report Report) error {
	if report.Package == "" ||
		report.Author == "" ||
		report.Subject == "" {
		return errors.New("invalid report: missing package, author, subject")
	}
	if report.Metadata == nil {
		report.Metadata = make(map[string]string)
	}
	if report.CreatedAt.IsZero() {
		report.CreatedAt = time.Now()
	}

	lastIssueNumber, ok := d.lastIssueNumber[report.Package]
	if !ok {
		// initialize last issue ptr
		err := d.fetchLastIssueNumber(ctx, report.Package)
		if err != nil {
			return err
		}
	}

	return retry.Retry(ctx, autoIncrementRetryStrategy, func(ctx context.Context) (bool, error) {
		lastIssueNumber++
		d.lastIssueNumber[report.Package] = lastIssueNumber

		id := fmt.Sprintf("%s-%d", strings.ToUpper(report.Package), lastIssueNumber)
		report.Metadata[ReportMetadataIDField] = id
		report.Metadata[reportMetadataIssueNumberField] = strconv.Itoa(lastIssueNumber)

		p := reportDocument{
			Type:   documentTypeReport,
			Report: report,
		}
		err := d.db.Create(ctx, id, p)
		return err == document.ErrAlreadyExists, err
	})
}

func (d *documentTracker) list(ctx context.Context, filters []document.Filter) ([]Report, error) {
	logrus.Debugf("calling List with filters: %#v", filters)

	it, err := d.db.List(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("document.Service.Create: %v", err)
	}
	defer it.Close()

	var ret []Report
	for it.HasNext() {
		var temp reportDocument
		if nextErr := it.NextTo(&temp); nextErr != nil {
			err = multierror.Append(err, nextErr)
			continue
		}
		ret = append(ret, temp.Report)
	}
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// ListVersionReports returns all the reports stored for a given release.
func (d *documentTracker) ListVersionReports(
	ctx context.Context, pkg, ver string, filters map[string]string,
) ([]Report, error) {
	docFilters := makeReportFilters(pkg, filters)
	docFilters = addVersionFilter(docFilters, ver)
	return d.list(ctx, docFilters)
}

// ListPackageReports returns all the reports stored for a given package.
func (d *documentTracker) ListPackageReports(
	ctx context.Context, pkg string, filters map[string]string,
) ([]Report, error) {
	docFilters := makeReportFilters(pkg, filters)
	return d.list(ctx, docFilters)
}

func (m *documentTracker) GetReport(ctx context.Context, id string) (Report, error) {
	var doc reportDocument
	err := m.db.Get(ctx, id, &doc)
	if err != nil {
		return Report{}, err
	}
	return doc.Report, nil
}

func (d *documentTracker) DeleteReport(ctx context.Context, id string) error {
	var doc reportDocument
	err := d.db.Get(ctx, id, &doc)
	if err != nil {
		return err
	}
	return d.db.Delete(ctx, id)
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
		path := append([]string{"Report"}, ks...)
		ret = append(ret,
			document.Filter{
				Field: document.Field{
					FieldPath: path,
					Value:     v,
				},
				Op: document.OpEqual,
			})
	}
	return ret
}
