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
