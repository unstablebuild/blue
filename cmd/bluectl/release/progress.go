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
package release

import (
	"os"

	"github.com/cheggaaa/pb/v3"
)

type barProgress struct {
	f   *os.File
	bar *pb.ProgressBar
}

func newBarProgress(f *os.File) *barProgress {
	return &barProgress{f: f}
}

func (bp *barProgress) Progress(progress, total int64, units string) {
	if bp.bar == nil {
		bp.bar = pb.New64(total)
		if units == "bytes" {
			bp.bar.Set(pb.Bytes, true)
		}
		bp.bar.Start()
	}
	bp.bar.SetCurrent(progress)
}

func (bp *barProgress) Read(p []byte) (n int, err error) {
	return bp.f.Read(p)
}

func (bp *barProgress) Write(p []byte) (n int, err error) {
	return bp.f.Write(p)
}

func (bp *barProgress) Seek(offset int64, whence int) (ret int64, err error) {
	return bp.f.Seek(offset, whence)
}

// enables document manager to use Stat and provide an accurate Create progress
func (bp *barProgress) Stat() (os.FileInfo, error) {
	return bp.f.Stat()
}

func (bp *barProgress) Close() error {
	if bp.bar == nil {
		return nil
	}
	bp.bar.Finish()
	bp.bar = nil
	return nil
}
