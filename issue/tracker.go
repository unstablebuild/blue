package issue

import "context"

// Tracker abstracts the ability to manage bug and feature reports.
type Tracker interface {
	// GetReport gets a Report from its id.
	GetReport(context.Context, string) (Report, error)

	// DeleteReport permanently deletes a Report with the given id.
	DeleteReport(context.Context, string) error

	// CloseReport marks a Report with the given id closed. This method
	// is idempotent.
	CloseReport(context.Context, string) error

	// AddReport stores the given Report. Implementation must return an erro
	// if Report.Package, Report.Author or Report.Subject are not defined.
	AddReport(context.Context, Report) error

	// ListVersionReports lists open reports of a package version, unless
	// "Closed": "true" is passed as a filter, in which case all closed reports
	// are returned.
	ListVersionReports(ctx context.Context, pkg, ver string,
		filters map[string]string) ([]Report, error)

	// ListPackageReports lists open reports of a package, unless
	// "Closed": "true" is passed as a filter, in which case all closed reports
	// are returned.
	ListPackageReports(ctx context.Context, pkg string,
		filters map[string]string) ([]Report, error)

	// UpdateReport overrides the given Report.
	UpdateReport(ctx context.Context, id string, r Report) error
}
