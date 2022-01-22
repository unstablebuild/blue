package release

import "io"

var _ io.ReadWriteSeeker = seekerProgressDelegate{}

type progresser interface {
	Progress(progress, total int, units string)
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
}

func (d progressDelegate) Progress(progress, total int, units string) {
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
	if d.readDelegate != nil {
		return d.readDelegate.(io.Seeker).Seek(offset, whence)
	}
	return d.writeDelegate.(io.Seeker).Seek(offset, whence)
}

// NopProgressReader wraps r to satisfy ProgressReader. If r satisfies io.Seeker
// then the returned ProgressReader will satisfy io.Seeker as well.
func NopProgressReader(r io.Reader) ProgressReader {
	if _, ok := r.(io.Seeker); ok {
		return seekerProgressDelegate{
			progressDelegate: progressDelegate{readDelegate: r},
		}
	}
	return progressDelegate{readDelegate: r}
}

// NopProgressWriter wraps w to satisfy ProgressWriter.
// If w satisfies io.ReadWriteSeeker then the returned ProgressWriter
// will satisfy io.ReadWriteSeeker as well.
func NopProgressWriter(w io.Writer) ProgressWriter {
	if rws, ok := w.(io.ReadWriteSeeker); ok {
		return seekerProgressDelegate{
			progressDelegate: progressDelegate{
				writeDelegate: rws, readDelegate: rws,
			},
		}
	}
	return progressDelegate{writeDelegate: w}
}
