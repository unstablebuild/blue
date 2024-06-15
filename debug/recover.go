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
package debug

import (
	"fmt"
	"runtime/debug"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/issue"
)

const (
	reportMetadataErrorField      = "error"
	reportMetadataStackTraceField = "stack"
	reportMetadataPanicField      = "panic"
	reportMetadataBugField        = "bug"
)

// CapturePanic attempts to capture a panic during execution of f, logs it
// and returns a Report and false, or returns true if f returned
// successfully.
func CapturePanic(log *log.Logger, pkg, version string, f func()) (
	ok bool, report issue.Report,
) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		report = issue.Report{
			Author:    "debug.CapturePanic",
			Package:   pkg,
			Version:   version,
			CreatedAt: time.Now(),
			Metadata:  make(map[string]string),
		}

		bi, ok := debug.ReadBuildInfo()
		if ok {
			report.Build = *bi
		}

		var errStr string
		switch x := r.(type) {
		case string:
			errStr = x
		case error:
			errStr = x.Error()
		default:
			errStr = fmt.Sprintf("unknown: %v", r)
		}
		report.Metadata[reportMetadataStackTraceField] = string(debug.Stack())
		report.Metadata[reportMetadataBugField] = ""
		report.Metadata[reportMetadataPanicField] = ""
		report.Subject = fmt.Sprintf("%20s", errStr)
		report.Metadata[reportMetadataErrorField] = errStr

		log.Errorf("CapturePanic: panic: %v", report.Metadata[reportMetadataErrorField])
	}()

	f()
	ok = true
	return
}
