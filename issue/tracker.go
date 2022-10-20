package issue

import "context"

// Tracker abstracts the ability to manage bug and feature reports.
type Tracker interface {
	// GetReport gets a debug.Report from its uuid.
	GetReport(context.Context, string) (Report, error)

	// DeleteReport deletes a debug.Report with the given uuid.
	DeleteReport(context.Context, string) error

	// AddReport stores the given debug.Report.
	AddReport(context.Context, Report) error

	// ListVersionReports lists all debug.Reports of a package version.
	ListVersionReports(ctx context.Context, pkg, ver string,
		filters map[string]string) ([]Report, error)

	// ListPackageReports lists all debug.Reports of a package.
	ListPackageReports(ctx context.Context, pkg string,
		filters map[string]string) ([]Report, error)
}
