// Copyright 2018-2026 Unstable Build, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package issue

import (
	"context"
	"github.com/unstablebuild/blue/iterator"
)

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
	CreateReport(context.Context, Report) (string, error)

	// ListVersionReports lists open reports of a package version, unless
	// "Closed": "true" is passed as a filter, in which case all closed reports
	// are returned.
	ListVersionReports(ctx context.Context, pkg, ver string,
		filters map[string]string) (iterator.Iterator[Report], error)

	// ListPackageReports lists open reports of a package, unless
	// "Closed": "true" is passed as a filter, in which case all closed reports
	// are returned.
	ListPackageReports(ctx context.Context, pkg string,
		filters map[string]string) (iterator.Iterator[Report], error)

	// UpdateReport overrides the given Report.
	UpdateReport(ctx context.Context, id string, r Report) error
}
