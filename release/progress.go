package release

import (
	"errors"
	"io"
	"os"
)

var _ io.ReadWriteSeeker = seekerProgressDelegate{}

type progresser interface {
	Progress(progress, total int64, units string)
}

type progressDelegate struct {
	progressDelegate progresser
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

// NopProgressReader wraps r to satisfy ProgressReader.
// If r satisfies io.ReadWriteSeeker then the returned ProgressReader
// will satisfy io.ReadWriteSeeker as well.
func NopProgressReader(r io.Reader) ProgressReader {
	if rws, ok := r.(io.ReadWriteSeeker); ok {
		return newSeekerProgressDelegate(rws)
	}
	return progressDelegate{readDelegate: r}
}

// NopProgressWriter wraps w to satisfy ProgressWriter.
// If w satisfies io.ReadWriteSeeker then the returned ProgressWriter
// will satisfy io.ReadWriteSeeker as well.
func NopProgressWriter(w io.Writer) ProgressWriter {
	if rws, ok := w.(io.ReadWriteSeeker); ok {
		return newSeekerProgressDelegate(rws)
	}
	return progressDelegate{writeDelegate: w}
}
