// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package idepkg

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func mustParseYAML(t *testing.T, s string) *yaml.Node {
	t.Helper()
	if s == "" {
		return &yaml.Node{
			Kind: yaml.DocumentNode,
			Content: []*yaml.Node{
				{Kind: yaml.MappingNode, Tag: "!!map"},
			},
		}
	}
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(s), &doc))
	return &doc
}

func nodeToString(t *testing.T, n *yaml.Node) string {
	t.Helper()
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	require.NoError(t, enc.Encode(n))
	require.NoError(t, enc.Close())
	return buf.String()
}

func TestMergeYAMLNodes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		dst      string
		src      string
		expected string
	}{
		{
			name:     "add new keys",
			dst:      "a: 1\n",
			src:      "b: 2\n",
			expected: "a: 1\nb: 2\n",
		},
		{
			name:     "overwrite scalar",
			dst:      "a: 1\n",
			src:      "a: 2\n",
			expected: "a: 2\n",
		},
		{
			name: "deep merge nested maps",
			dst:  "top:\n  a: 1\n  b: 2\n",
			src:  "top:\n  b: 3\n  c: 4\n",
			expected: "top:\n  a: 1\n  b: 3\n  c: 4\n",
		},
		{
			name: "deep merge two levels",
			dst:  "l1:\n  l2:\n    a: 1\n",
			src:  "l1:\n  l2:\n    b: 2\n",
			expected: "l1:\n  l2:\n    a: 1\n    b: 2\n",
		},
		{
			name:     "replace sequences",
			dst:      "items:\n  - a\n  - b\n",
			src:      "items:\n  - x\n  - y\n  - z\n",
			expected: "items:\n  - x\n  - y\n  - z\n",
		},
		{
			name:     "empty src is no-op",
			dst:      "a: 1\nb: 2\n",
			src:      "",
			expected: "a: 1\nb: 2\n",
		},
		{
			name:     "empty dst gets all src keys",
			dst:      "",
			src:      "a: 1\nb: 2\n",
			expected: "a: 1\nb: 2\n",
		},
		{
			name:     "same values are no-op",
			dst:      "a: 1\nb: 2\n",
			src:      "a: 1\nb: 2\n",
			expected: "a: 1\nb: 2\n",
		},
		{
			name:     "map key replaced by scalar",
			dst:      "a:\n  nested: 1\n",
			src:      "a: flat\n",
			expected: "a: flat\n",
		},
		{
			name:     "scalar replaced by map",
			dst:      "a: flat\n",
			src:      "a:\n  nested: 1\n",
			expected: "a:\n  nested: 1\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dstDoc := mustParseYAML(t, tt.dst)
			srcDoc := mustParseYAML(t, tt.src)
			mergeYAMLNodes(dstDoc.Content[0], srcDoc.Content[0])
			actual := nodeToString(t, dstDoc)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestMergeYAMLNodesPreservesComments(t *testing.T) {
	t.Parallel()
	dst := "# top comment\na: 1 # inline\n"
	src := "b: 2\n"
	dstDoc := mustParseYAML(t, dst)
	srcDoc := mustParseYAML(t, src)
	mergeYAMLNodes(dstDoc.Content[0], srcDoc.Content[0])
	actual := nodeToString(t, dstDoc)
	assert.Contains(t, actual, "# top comment")
	assert.Contains(t, actual, "# inline")
	assert.Contains(t, actual, "b: 2")
}

func TestExpandNodeValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		mapping  func(string) string
		expected string
	}{
		{
			name:  "expand string scalar",
			input: "path: $HOME/bin\n",
			mapping: func(key string) string {
				if key == "HOME" {
					return "/usr"
				}
				return ""
			},
			expected: "path: /usr/bin\n",
		},
		{
			name:  "skip int",
			input: "port: 8080\n",
			mapping: func(string) string {
				return "REPLACED"
			},
			expected: "port: 8080\n",
		},
		{
			name:  "skip bool",
			input: "enabled: true\n",
			mapping: func(string) string {
				return "REPLACED"
			},
			expected: "enabled: true\n",
		},
		{
			name:  "skip null",
			input: "val: null\n",
			mapping: func(string) string {
				return "REPLACED"
			},
			expected: "val: null\n",
		},
		{
			name:  "expand nested values",
			input: "top:\n  inner:\n    path: $DIR/file\n",
			mapping: func(key string) string {
				if key == "DIR" {
					return "/tmp"
				}
				return ""
			},
			expected: "top:\n  inner:\n    path: /tmp/file\n",
		},
		{
			name:  "unknown vars expand to empty",
			input: "path: $UNKNOWN/rest\n",
			mapping: func(string) string {
				return ""
			},
			expected: "path: /rest\n",
		},
		{
			name:  "skip float",
			input: "rate: 3.14\n",
			mapping: func(string) string {
				return "REPLACED"
			},
			expected: "rate: 3.14\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := mustParseYAML(t, tt.input)
			expandNodeValues(doc, tt.mapping)
			actual := nodeToString(t, doc)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestLoadOrCreateUserConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		setup       func(t *testing.T, dir string) string
		expectEmpty bool
		expectErr   bool
	}{
		{
			name: "file missing creates empty doc",
			setup: func(t *testing.T, dir string) string {
				return filepath.Join(dir, "nonexistent.yaml")
			},
			expectEmpty: true,
		},
		{
			name: "file exists",
			setup: func(t *testing.T, dir string) string {
				p := filepath.Join(dir, "config.yaml")
				require.NoError(t, os.WriteFile(p, []byte("key: value\n"), 0644))
				return p
			},
		},
		{
			name: "empty file",
			setup: func(t *testing.T, dir string) string {
				p := filepath.Join(dir, "empty.yaml")
				require.NoError(t, os.WriteFile(p, []byte{}, 0644))
				return p
			},
			expectEmpty: true,
		},
		{
			name: "invalid YAML",
			setup: func(t *testing.T, dir string) string {
				p := filepath.Join(dir, "bad.yaml")
				require.NoError(t, os.WriteFile(p, []byte(":\n  :\n  - {["), 0644))
				return p
			},
			expectErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := tt.setup(t, dir)
			doc, err := loadOrCreateUserConfig(path)
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, doc)
			require.Equal(t, yaml.DocumentNode, doc.Kind)
			require.Len(t, doc.Content, 1)
			require.Equal(t, yaml.MappingNode, doc.Content[0].Kind)
			if tt.expectEmpty {
				assert.Empty(t, doc.Content[0].Content)
			} else {
				assert.NotEmpty(t, doc.Content[0].Content)
			}
		})
	}
}

func TestWriteYAMLAtomic(t *testing.T) {
	t.Parallel()
	t.Run("round-trip write and read", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		doc := mustParseYAML(t, "key: value\nnested:\n  a: 1\n")
		expected := doc.Content[0]

		require.NoError(t, writeYAMLAtomic(path, doc, expected))

		readDoc, err := loadOrCreateUserConfig(path)
		require.NoError(t, err)
		idx := findMappingKey(readDoc.Content[0], "key")
		require.GreaterOrEqual(t, idx, 0)
		assert.Equal(t, "value", readDoc.Content[0].Content[idx+1].Value)
	})

	t.Run("creates parent dirs", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "sub", "deep", "config.yaml")
		doc := mustParseYAML(t, "a: 1\n")
		expected := doc.Content[0]

		require.NoError(t, writeYAMLAtomic(path, doc, expected))

		_, err := os.Stat(path)
		require.NoError(t, err)
	})

	t.Run("verification catches corruption", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		// Write a doc that doesn't contain the expected keys
		doc := mustParseYAML(t, "other: stuff\n")
		expected := &yaml.Node{
			Kind: yaml.MappingNode,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "required_key"},
				{Kind: yaml.ScalarNode, Value: "required_value"},
			},
		}

		err := writeYAMLAtomic(path, doc, expected)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "verification failed")

		// File should not exist since verification failed
		_, statErr := os.Stat(path)
		assert.True(t, os.IsNotExist(statErr))
	})
}

func TestVerifyMerge(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		written   string
		expected  string
		expectErr string
	}{
		{
			name:     "matching docs pass",
			written:  "a: 1\nb: 2\n",
			expected: "a: 1\nb: 2\n",
		},
		{
			name:      "missing key fails",
			written:   "a: 1\n",
			expected:  "a: 1\nb: 2\n",
			expectErr: `key "b" missing from written config`,
		},
		{
			name:      "wrong value fails",
			written:   "a: 1\nb: wrong\n",
			expected:  "a: 1\nb: 2\n",
			expectErr: `key "b": expected "2", got "wrong"`,
		},
		{
			name:      "nested mismatch fails",
			written:   "top:\n  a: 1\n",
			expected:  "top:\n  a: 1\n  b: 2\n",
			expectErr: `key "top": key "b" missing from written config`,
		},
		{
			name:     "written has extra keys and still passes",
			written:  "a: 1\nb: 2\nextra: 3\n",
			expected: "a: 1\nb: 2\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wDoc := mustParseYAML(t, tt.written)
			eDoc := mustParseYAML(t, tt.expected)
			err := verifyMerge(wDoc.Content[0], eDoc.Content[0])
			if tt.expectErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCloneNode(t *testing.T) {
	t.Parallel()
	original := mustParseYAML(t, "a: 1\nnested:\n  b: 2\n")
	clone := cloneNode(original)

	// Mutate clone
	clone.Content[0].Content[1].Value = "changed"
	clone.Content[0].Content[3].Content[1].Value = "mutated"

	// Original should be unchanged
	assert.Equal(t, "1", original.Content[0].Content[1].Value)
	assert.Equal(t, "2", original.Content[0].Content[3].Content[1].Value)
}

func TestBackupUserConfig(t *testing.T) {
	t.Parallel()
	t.Run("creates backup", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		require.NoError(t, os.WriteFile(path, []byte("original: content\n"), 0644))

		_, err := backupUserConfig(path)
		require.NoError(t, err)

		data, err := os.ReadFile(path + ".backup")
		require.NoError(t, err)
		assert.Equal(t, "original: content\n", string(data))
	})

	t.Run("overwrites previous backup", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		backupPath := path + ".backup"
		require.NoError(t, os.WriteFile(backupPath, []byte("old backup\n"), 0644))
		require.NoError(t, os.WriteFile(path, []byte("new content\n"), 0644))

		_, err := backupUserConfig(path)
		require.NoError(t, err)

		data, err := os.ReadFile(backupPath)
		require.NoError(t, err)
		assert.Equal(t, "new content\n", string(data))
	})

	t.Run("returns error if file does not exist", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "nonexistent.yaml")

		_, err := backupUserConfig(path)
		require.Error(t, err)
	})
}
