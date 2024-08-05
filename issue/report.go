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
	"runtime/debug"
	"strings"
	"time"
)

// Report contains informatin about a suspected or confirmed bug.
type Report struct {
	Package   string
	Version   string
	Author    string
	Subject   string
	Notes     string
	Build     debug.BuildInfo `yaml:"build,omitempty"`
	CreatedAt time.Time       `yaml:"created_at,omitempty"`
	UpdatedAt time.Time       `yaml:"updated_at,omitempty"`
	UpdatedBy string          `yaml:"-"`
	Closed    bool            `yaml:"closed,omitempty"`
	ClosedAt  time.Time       `yaml:"closed_at,omitempty"`
	Metadata  map[string]string
}

// IsInternalLabel can be used for Metadata keys to
// figure out if label is internal.
func IsInternalLabel(k string) bool {
	return strings.HasPrefix(k, "_")
}
