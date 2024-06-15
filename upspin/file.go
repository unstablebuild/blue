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
package upspin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/unstablebuild/blue/retry"
	"upspin.io/errors"
	"upspin.io/upspin"
)

var (
	maxInt       = int64(^uint(0) >> 1) // mimic upspin.File behaviour
	syncTimeout  = 10 * time.Second
	syncStrategy = retry.CombinedStrategy(
		retry.LimitStrategy(100), retry.ExponentialStrategy(1*time.Millisecond, 1024*time.Millisecond))

	_ upspin.File = (*File)(nil)
)

// File is an alternate implementation of upspin.File, which allows
// for opening a file for read and write at the same time. In contrast,
// it stores the data in-memory, to it should only be used over upspin.File
// when the all the data is usually read anyway.
// This File it's always readable so passing O_WRONLY or O_RDWR is equivalent.
type File struct {
	name        upspin.PathName
	writable    bool
	create      bool
	dirty       bool
	closed      bool
	client      upspin.Client
	readOffset  int64
	writeOffset int64

	// read side
	reader *bytes.Reader
	// write side
	data         []byte
	lastPutSeqID int64
}

var _ upspin.File = (*File)(nil)

// NewFile opens or creates a new file with a given name and uses
// flag to determine if whether to create a file in case it doesn't exist
// O_CREATE and whether to open it in os.O_RDONLY or O_RDWR.
// If name points to a link, it will follow it.
func Open(
	client upspin.Client, name upspin.PathName, flag int,
) (*File, error) {
	const op errors.Op = "Open"
	data, err := client.Get(name)
	create := flag&os.O_CREATE != 0
	isNotExistError := errors.Is(errors.NotExist, err)
	if err != nil {
		if !isNotExistError || !create {
			return nil, err
		}
	} else if create && flag&os.O_EXCL != 0 {
		return nil, errors.E(op, name, errors.Exist)
	}
	f := &File{
		client:      client,
		name:        name,
		data:        data,
		dirty:       true,
		create:      create,
		writeOffset: int64(len(data)),
	}

	if flag&os.O_RDWR == os.O_RDWR || flag&os.O_WRONLY == os.O_WRONLY {
		f.writable = true
	}
	return f, nil
}

// Name implements upspin.File.
func (f *File) Name() upspin.PathName {
	return f.name
}

// Read implements upspin.File.
func (f *File) Read(b []byte) (n int, err error) {
	const op errors.Op = "file.Read"
	if f.dirty {
		err = f.initRead()
		if err != nil {
			return 0, errors.E(op, f.name, err)
		}
	}
	n, err = f.reader.Read(b)
	if err == nil {
		f.readOffset += int64(n)
	}
	return
}

// ReadAt implements upspin.File.
func (f *File) ReadAt(b []byte, off int64) (n int, err error) {
	const op errors.Op = "file.Read"
	if f.dirty {
		err = f.initRead()
		if err != nil {
			return 0, errors.E(op, f.name, err)
		}
	}
	return f.reader.ReadAt(b, off)
}

// Seek implements upspin.File.
func (f *File) Seek(offset int64, whence int) (ret int64, err error) {
	const op errors.Op = "file.Seek"
	if f.closed {
		return 0, f.errClosed(op)
	}

	if f.dirty {
		err = f.initRead()
		if err != nil {
			return 0, errors.E(op, f.name, err)
		}
	}

	if f.writeOffset+offset < 0 || f.readOffset+offset < 0 || offset > maxInt || !f.writable && offset > int64(len(f.data)) {
		return 0, errors.E(op, errors.Invalid, f.name, "bad offset")
	}

	// validate whence
	switch whence {
	case 0, 1, 2:
	default:
		return 0, errors.E(op, errors.Invalid, f.name, "bad whence")
	}

	ret, err = f.reader.Seek(offset, whence)
	if err == nil {
		f.writeOffset = ret
		f.readOffset = ret
	}
	return
}

// Write implements upspin.File.
func (f *File) Write(b []byte) (n int, err error) {
	const op errors.Op = "file.Write"
	n, err = f.writeAt(op, b, f.writeOffset)
	if err == nil {
		f.writeOffset += int64(n)
	}
	return
}

// WriteAt implements upspin.File.
func (f *File) WriteAt(b []byte, off int64) (n int, err error) {
	const op errors.Op = "file.WriteAt"
	return f.writeAt(op, b, off)
}

func (f *File) writeAt(op errors.Op, b []byte, off int64) (n int, err error) {
	if f.closed {
		return 0, f.errClosed(op)
	}
	if !f.writable {
		return 0, errors.E(op, errors.Invalid, f.name, "not open for write")
	}
	if off < 0 {
		return 0, errors.E(op, errors.Invalid, f.name, "negative offset")
	}
	end := off + int64(len(b))
	if end > maxInt {
		return 0, errors.E(op, errors.Invalid, f.name, "file too long")
	}
	// mark as dirty so read resyncs
	f.dirty = true
	if end > int64(cap(f.data)) {
		nLen := end * 3 / 2
		if nLen > maxInt {
			nLen = maxInt
		}
		ndata := make([]byte, len(f.data), nLen)
		copy(ndata, f.data)
		f.data = ndata
	}
	if end > int64(len(f.data)) {
		f.data = f.data[:end]
	}
	copy(f.data[off:], b)
	return len(b), nil
}

// Sync writes the accumulated data to a StoreServer, if this file
// is writeable, otherwise it will return an error. It returns the
// updated DirEntry for the file.
func (f *File) Sync() (*upspin.DirEntry, error) {
	seqID, err := f.sync()
	if err != nil {
		return nil, err
	}

	// wait for consistency
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()

	var ret *upspin.DirEntry
	err = retry.Retry(ctx, syncStrategy, func(ctx context.Context) (retry bool, err error) {
		entry, err := f.client.Lookup(f.Name(), true)
		if err != nil {
			return errors.Is(errors.NotExist, err), err
		}
		if entry.Sequence != seqID {
			return true, fmt.Errorf("expected sequence ID %d but Lookup found %d", entry.Sequence, seqID)
		}
		ret = entry
		return false, nil
	})
	return ret, err
}

func (f *File) sync() (int64, error) {
	const op errors.Op = "file.Sync"
	if f.closed {
		return 0, f.errClosed(op)
	}
	// allow creating an entry, even if file is not writeable
	// if create flag is passed and there's no data in the buffer.
	if f.writable || (len(f.data) == 0 && f.create) {
		err := f.put(op)
		if err != nil {
			return 0, err
		}
		return f.lastPutSeqID, nil
	}
	return 0, errors.E(op, errors.Invalid, f.name, "not open for write")
}

// Truncate changes the size of the file to size.
func (f *File) Truncate(size int) error {
	const op errors.Op = "file.Truncate"
	if f.closed {
		return f.errClosed(op)
	}
	if !f.writable {
		return errors.E(op, errors.Invalid, f.name, "not open for write")
	}

	if size >= len(f.data) {
		return nil
	}

	f.dirty = true
	f.data = f.data[:size]
	f.writeOffset = int64(math.Min(float64(size), float64(f.writeOffset)))
	f.readOffset = int64(math.Min(float64(size), float64(f.readOffset)))
	return nil
}

// Close implements upspin.File.
func (f *File) Close() error {
	const op errors.Op = "file.Close"
	if f.closed {
		return f.errClosed(op)
	}
	f.closed = true
	if !f.writable {
		return nil
	}

	err := f.put(op)
	f.data = nil
	return err
}

func (f *File) errClosed(op errors.Op) error {
	return errors.E(op, errors.Invalid, f.name, "is closed")
}

func (f *File) put(op errors.Op) error {
	entry, err := f.client.Put(f.name, f.data)
	if err == nil {
		f.lastPutSeqID = entry.Sequence
	}
	return err
}

func (f *File) initRead() error {
	f.dirty = false
	f.reader = bytes.NewReader(f.data)
	n, err := f.reader.Seek(f.readOffset, io.SeekStart)
	if err == nil {
		// self-correct in case reader cannot seek to the current offset
		f.readOffset = n
	}
	return err
}
