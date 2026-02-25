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

package languages

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtensionForLanguage(t *testing.T) {
	tests := []struct {
		language string
		wantExt  string
		wantOK   bool
	}{
		{"python", ".py", true},
		{"javascript", ".js", true},
		{"typescript", ".ts", true},
		{"rust", ".rs", true},
		{"c", ".c", true},
		{"css", ".css", true},
		{"yaml", ".yaml", true},
		// Implicit languages (ext[1:] == languageID) are not in the map.
		{"go", "", false},
		{"somethingmadeup", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			ext, ok := ExtensionForLanguage(tt.language)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantExt, ext)
		})
	}
}

func TestExtensionForLanguageWithFallback(t *testing.T) {
	tests := []struct {
		language string
		wantExt  string
	}{
		{"python", ".py"},
		{"rust", ".rs"},
		// Implicit languages fall back to "." + language.
		{"go", ".go"},
		{"json", ".json"},
		{"somethingmadeup", ".somethingmadeup"},
	}
	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			assert.Equal(t, tt.wantExt, ExtensionForLanguageWithFallback(tt.language))
		})
	}
}

func TestFilenameForLanguage(t *testing.T) {
	tests := []struct {
		language string
		want     string
	}{
		{"go", "foo.go"},
		{"python", "foo.py"},
		{"javascript", "foo.js"},
		{"rust", "foo.rs"},
		{"unknown_lang", "foo.unknown_lang"},
	}
	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			assert.Equal(t, tt.want, FilenameForLanguage(tt.language))
		})
	}
}
