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
	// NewCreateWriter returns a writer that creates the object.
	// Finalising the write fails when the object already exists,
	// leaving the existing object untouched.
	NewCreateWriter(ctx context.Context) io.WriteCloser
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

func (g *gcsObject) NewCreateWriter(ctx context.Context) io.WriteCloser {
	return g.o.If(storage.Conditions{DoesNotExist: true}).NewWriter(ctx)
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
func (g *gcsReader) Size() int64                { return g.r.Attrs.Size }
