// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2026 Unstable Build, All Rights Reserved.
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

package gcsrelease

import (
	"context"
	"io"

	"cloud.google.com/go/storage"
)

// Bucket abstracts the operations needed on a GCS bucket so that
// implementations can be swapped for testing.
type Bucket interface {
	Object(name string) Object
	SignedURL(object string, opts *storage.SignedURLOptions) (string, error)
}

// Object abstracts the operations needed on a GCS object.
type Object interface {
	NewWriter(ctx context.Context) io.WriteCloser
	NewReader(ctx context.Context) (ObjectReader, error)
	Delete(ctx context.Context) error
}

// ObjectReader extends io.ReadCloser with a Size method, which
// corresponds to storage.Reader.Attrs.Size.
type ObjectReader interface {
	io.ReadCloser
	Size() int64
}

// gcsBucket adapts *storage.BucketHandle to the Bucket interface.
type gcsBucket struct {
	b *storage.BucketHandle
}

// NewBucket wraps a *storage.BucketHandle as a Bucket.
func NewBucket(b *storage.BucketHandle) Bucket {
	return &gcsBucket{b: b}
}

func (g *gcsBucket) Object(name string) Object {
	return &gcsObject{o: g.b.Object(name)}
}

func (g *gcsBucket) SignedURL(object string, opts *storage.SignedURLOptions) (string, error) {
	return g.b.SignedURL(object, opts)
}

// gcsObject adapts *storage.ObjectHandle to the Object interface.
type gcsObject struct {
	o *storage.ObjectHandle
}

func (g *gcsObject) NewWriter(ctx context.Context) io.WriteCloser {
	return g.o.NewWriter(ctx)
}

func (g *gcsObject) NewReader(ctx context.Context) (ObjectReader, error) {
	r, err := g.o.NewReader(ctx)
	if err != nil {
		return nil, err
	}
	return &gcsReader{r: r}, nil
}

func (g *gcsObject) Delete(ctx context.Context) error {
	return g.o.Delete(ctx)
}

// gcsReader adapts *storage.Reader to ObjectReader.
type gcsReader struct {
	r *storage.Reader
}

func (g *gcsReader) Read(p []byte) (int, error) { return g.r.Read(p) }
func (g *gcsReader) Close() error               { return g.r.Close() }
func (g *gcsReader) Size() int64                 { return g.r.Attrs.Size }
