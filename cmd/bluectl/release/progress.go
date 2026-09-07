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
