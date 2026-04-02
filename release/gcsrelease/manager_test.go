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
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/release"
)

func TestManagerSignedDownloadURLDefaultsExpiry(t *testing.T) {
	t.Parallel()
	b := &capturingBucket{}
	m := NewManager(nil, b)

	_, err := m.SignedDownloadURL(context.Background(), "pkg", release.Version("1"))
	require.NoError(t, err)
	require.NotNil(t, b.lastOpts)
	require.Equal(t, "GET", b.lastOpts.Method)
	require.False(t, b.lastOpts.Expires.IsZero())
	require.True(t, b.lastOpts.Expires.After(time.Now()))
	require.Equal(t, "pkg/1", b.lastObject)
}

func TestManagerSignedDownloadURLPreservesFutureExpiry(t *testing.T) {
	t.Parallel()
	want := time.Now().Add(42 * time.Minute).Round(time.Second)
	b := &capturingBucket{}
	m := NewManager(nil, b, WithSignedURLOptions(&storage.SignedURLOptions{
		Method:  "HEAD",
		Expires: want,
	}))

	_, err := m.SignedDownloadURL(context.Background(), "pkg", release.Version("1"))
	require.NoError(t, err)
	require.Equal(t, "HEAD", b.lastOpts.Method)
	require.Equal(t, want, b.lastOpts.Expires)
}

type capturingBucket struct {
	lastObject string
	lastOpts   *storage.SignedURLOptions
}

func (b *capturingBucket) Object(name string) Object {
	return capturingObject{}
}

func (b *capturingBucket) SignedURL(object string, opts *storage.SignedURLOptions) (string, error) {
	b.lastObject = object
	clone := *opts
	b.lastOpts = &clone
	return "https://example.invalid", nil
}

type capturingObject struct{}

func (capturingObject) NewWriter(context.Context) io.WriteCloser        { panic("unused") }
func (capturingObject) NewReader(context.Context) (ObjectReader, error) { panic("unused") }
func (capturingObject) Delete(context.Context) error                    { panic("unused") }
