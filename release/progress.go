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
	"errors"
	"io"
	"os"
)

// IOProgress is implemented by both ProgressReader and ProgressWriter.
type IOProgress interface {
	Progress(progress, total int64, units string)
}

// IsDiscard is satisfied by all ProgressReader and ProgressWriter
// returned by this package so release.Manager implementations can
// optimize better.
type IsDiscard interface {
	IsDiscard() bool
}

// NopProgressReader wraps r to satisfy ProgressReader, discarding
// any calls to Progress.
//
// If r satisfies io.ReadWriteSeeker then the returned ProgressReader
// will satisfy io.ReadWriteSeeker as well.
func NopProgressReader(r io.Reader) ProgressReader {
	if rws, ok := r.(io.ReadWriteSeeker); ok {
		return newSeekerProgressDelegate(rws)
	}
	return progressDelegate{readDelegate: r}
}

// NopProgressWriter wraps w to satisfy ProgressWriter, discarding
// any calls to Progress.
//
// If w satisfies io.ReadWriteSeeker then the returned ProgressWriter
// will satisfy io.ReadWriteSeeker as well.
func NopProgressWriter(w io.Writer) ProgressWriter {
	if rws, ok := w.(io.ReadWriteSeeker); ok {
		return newSeekerProgressDelegate(rws)
	}
	return progressDelegate{writeDelegate: w}
}

// NewRelayProgressWriter returns a ProgressWriter that relays writes
// to the given io.Writer and relays progress to the given IOProgress.
func NewRelayProgressWriter(w io.Writer, out IOProgress) ProgressWriter {
	return progressDelegate{writeDelegate: w, progressDelegate: out}
}

// NewRelayProgressReader returns a ProgressReader that relays reads
// to the given io.Reader and relays progress to the given IOProgress.
func NewRelayProgressReader(r io.Reader, in IOProgress) ProgressReader {
	statDelegate, _ := in.(interface{ Stat() (os.FileInfo, error) })
	return seekerProgressDelegate{
		progressDelegate: progressDelegate{
			readDelegate:     r,
			progressDelegate: in,
		},
		statDelegate: statDelegate,
	}
}

var _ io.ReadWriteSeeker = seekerProgressDelegate{}

var _ IsDiscard = progressDelegate{}
var _ IsDiscard = seekerProgressDelegate{}

type progressDelegate struct {
	progressDelegate IOProgress
	readDelegate     io.Reader
	writeDelegate    io.Writer
}

// satisfies io.Seeker as well so signing manager can still use
// interface satisfaction to optimize io processing
type seekerProgressDelegate struct {
	progressDelegate
	statDelegate interface{ Stat() (os.FileInfo, error) }
}

func newSeekerProgressDelegate(rws io.ReadWriteSeeker) seekerProgressDelegate {
	statDelegate, _ := rws.(interface{ Stat() (os.FileInfo, error) })
	return seekerProgressDelegate{
		progressDelegate: progressDelegate{
			writeDelegate: rws, readDelegate: rws,
		},
		statDelegate: statDelegate,
	}
}

func (d progressDelegate) IsDiscard() bool {
	return d.writeDelegate == io.Discard
}

func (d progressDelegate) Progress(progress, total int64, units string) {
	if d.progressDelegate == nil {
		return
	}
	d.progressDelegate.Progress(progress, total, units)
}

func (d progressDelegate) Read(p []byte) (n int, err error) {
	return d.readDelegate.Read(p)
}

func (d progressDelegate) Write(p []byte) (n int, err error) {
	return d.writeDelegate.Write(p)
}

func (d seekerProgressDelegate) Seek(offset int64, whence int) (int64, error) {
	return d.writeDelegate.(io.Seeker).Seek(offset, whence)
}

func (d seekerProgressDelegate) Stat() (os.FileInfo, error) {
	if d.statDelegate == nil {
		return nil, errors.New("cannot stat non os.File")
	}
	return d.statDelegate.Stat()
}
