// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package idelsp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveGlobPattern(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		raw     json.RawMessage
		want    string
		wantErr bool
	}{
		{
			name: "plain string",
			raw:  json.RawMessage(`"**/*.go"`),
			want: "**/*.go",
		},
		{
			name: "relative pattern",
			raw:  json.RawMessage(`{"baseUri":"file:///workspace","pattern":"**/*.go"}`),
			want: "**/*.go",
		},
		{
			name:    "invalid json",
			raw:     json.RawMessage(`{invalid`),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveGlobPattern(tt.raw)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchGlob(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{
			name:    "exact match",
			pattern: "go.mod",
			path:    "go.mod",
			want:    true,
		},
		{
			name:    "no match",
			pattern: "go.mod",
			path:    "go.sum",
			want:    false,
		},
		{
			name:    "star wildcard",
			pattern: "*.go",
			path:    "main.go",
			want:    true,
		},
		{
			name:    "star no match different ext",
			pattern: "*.go",
			path:    "main.rs",
			want:    false,
		},
		{
			name:    "double star recursive",
			pattern: "**/*.go",
			path:    "pkg/sub/file.go",
			want:    true,
		},
		{
			name:    "double star root level",
			pattern: "**/*.go",
			path:    "main.go",
			want:    true,
		},
		{
			name:    "double star prefix",
			pattern: "**/go.mod",
			path:    "go.mod",
			want:    true,
		},
		{
			name:    "double star prefix nested",
			pattern: "**/go.mod",
			path:    "subdir/go.mod",
			want:    true,
		},
		{
			name:    "brace expansion",
			pattern: "**/*.{go,mod,sum,work}",
			path:    "go.mod",
			want:    true,
		},
		{
			name:    "brace expansion go file",
			pattern: "**/*.{go,mod,sum,work}",
			path:    "pkg/file.go",
			want:    true,
		},
		{
			name:    "brace expansion go.sum",
			pattern: "**/*.{go,mod,sum,work}",
			path:    "go.sum",
			want:    true,
		},
		{
			name:    "brace expansion no match",
			pattern: "**/*.{go,mod,sum,work}",
			path:    "readme.txt",
			want:    false,
		},
		{
			name:    "question mark",
			pattern: "?.go",
			path:    "a.go",
			want:    true,
		},
		{
			name:    "question mark no match",
			pattern: "?.go",
			path:    "ab.go",
			want:    false,
		},
		{
			name:    "gopls go.mod pattern",
			pattern: "**/go.mod",
			path:    "go.mod",
			want:    true,
		},
		{
			name:    "gopls go.sum pattern",
			pattern: "**/go.sum",
			path:    "go.sum",
			want:    true,
		},
		{
			name:    "gopls go.work pattern",
			pattern: "**/go.work",
			path:    "go.work",
			want:    true,
		},
		{
			name:    "nested go.mod",
			pattern: "**/go.mod",
			path:    "vendor/module/go.mod",
			want:    true,
		},
		{
			name:    "trailing double star",
			pattern: "src/**",
			path:    "src/a/b/c.go",
			want:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := matchGlob(tt.pattern, tt.path)
			assert.Equal(t, tt.want, got,
				"matchGlob(%q, %q)", tt.pattern, tt.path)
		})
	}
}

