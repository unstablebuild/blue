// Copyright 2026 Unstable Build, LLC.
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

package markdown

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseHeaders(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name     string
		input    string
		expected int
		level    int
	}{
		{
			name:     "H1",
			input:    "# Header 1",
			expected: 1,
			level:    1,
		},
		{
			name:     "H2",
			input:    "## Header 2",
			expected: 1,
			level:    2,
		},
		{
			name:     "H3",
			input:    "### Header 3",
			expected: 1,
			level:    3,
		},
		{
			name:     "H4",
			input:    "#### Header 4",
			expected: 1,
			level:    4,
		},
		{
			name:     "H5",
			input:    "##### Header 5",
			expected: 1,
			level:    5,
		},
		{
			name:     "H6",
			input:    "###### Header 6",
			expected: 1,
			level:    6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks, err := parse(tt.input, &cfg)
			require.NoError(t, err)
			require.Len(t, blocks, tt.expected)

			header, ok := blocks[0].(*headerBlock)
			require.True(t, ok, "expected headerBlock")
			assert.Equal(t, tt.level, header.level)
		})
	}
}

func TestParseParagraphs(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name     string
		input    string
		expected int
		text     string
	}{
		{
			name:     "simple paragraph",
			input:    "Hello world",
			expected: 1,
			text:     "Hello world",
		},
		{
			name:     "paragraph with bold",
			input:    "Hello **bold** world",
			expected: 1,
			text:     "Hello bold world",
		},
		{
			name:     "paragraph with italic",
			input:    "Hello *italic* world",
			expected: 1,
			text:     "Hello italic world",
		},
		{
			name:     "paragraph with code",
			input:    "Hello `code` world",
			expected: 1,
			text:     "Hello code world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks, err := parse(tt.input, &cfg)
			require.NoError(t, err)
			require.Len(t, blocks, tt.expected)

			para, ok := blocks[0].(*paragraphBlock)
			require.True(t, ok, "expected paragraphBlock")
			assert.Equal(t, tt.text, para.content.String())
		})
	}
}

func TestParseCodeBlocks(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name     string
		input    string
		expected int
		language string
		code     string
	}{
		{
			name:     "fenced code block",
			input:    "```\ncode here\n```",
			expected: 1,
			language: "",
			code:     "code here\n",
		},
		{
			name:     "fenced code block with language",
			input:    "```go\nfunc main() {}\n```",
			expected: 1,
			language: "go",
			code:     "func main() {}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks, err := parse(tt.input, &cfg)
			require.NoError(t, err)
			require.Len(t, blocks, tt.expected)

			code, ok := blocks[0].(*codeBlock)
			require.True(t, ok, "expected codeBlock")
			assert.Equal(t, tt.language, code.language)
			assert.Equal(t, tt.code, code.code)
		})
	}
}

func TestParseLists(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name    string
		input   string
		ordered bool
		items   int
	}{
		{
			name:    "unordered list",
			input:   "- item 1\n- item 2\n- item 3",
			ordered: false,
			items:   3,
		},
		{
			name:    "ordered list",
			input:   "1. item 1\n2. item 2\n3. item 3",
			ordered: true,
			items:   3,
		},
		{
			name:    "task list",
			input:   "- [ ] unchecked\n- [x] checked",
			ordered: false,
			items:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks, err := parse(tt.input, &cfg)
			require.NoError(t, err)
			require.Len(t, blocks, 1)

			list, ok := blocks[0].(*listBlock)
			require.True(t, ok, "expected listBlock")
			assert.Equal(t, tt.ordered, list.ordered)
			assert.Len(t, list.items, tt.items)
		})
	}
}

func TestParseBlockquotes(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name     string
		input    string
		expected int
		content  int
	}{
		{
			name:     "simple blockquote",
			input:    "> quoted text",
			expected: 1,
			content:  1,
		},
		{
			name:     "multi-line blockquote",
			input:    "> line 1\n> line 2",
			expected: 1,
			content:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks, err := parse(tt.input, &cfg)
			require.NoError(t, err)
			require.Len(t, blocks, tt.expected)

			quote, ok := blocks[0].(*blockquoteBlock)
			require.True(t, ok, "expected blockquoteBlock")
			assert.Len(t, quote.content, tt.content)
		})
	}
}

func TestParseHorizontalRules(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name  string
		input string
	}{
		{name: "dashes", input: "---"},
		{name: "asterisks", input: "***"},
		{name: "underscores", input: "___"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks, err := parse(tt.input, &cfg)
			require.NoError(t, err)
			require.Len(t, blocks, 1)

			_, ok := blocks[0].(*horizontalRuleBlock)
			assert.True(t, ok, "expected horizontalRuleBlock")
		})
	}
}

func TestParseTables(t *testing.T) {
	cfg := DefaultConfig()
	input := `| Header 1 | Header 2 |
| --- | --- |
| Cell 1 | Cell 2 |
| Cell 3 | Cell 4 |`

	blocks, err := parse(input, &cfg)
	require.NoError(t, err)
	require.Len(t, blocks, 1)

	table, ok := blocks[0].(*tableBlock)
	require.True(t, ok, "expected tableBlock")
	assert.Len(t, table.header, 2)
	assert.Len(t, table.rows, 2)
}

func TestParseInlineFormatting(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name  string
		input string
		style inlineStyle
	}{
		{
			name:  "bold",
			input: "**bold**",
			style: styleBold,
		},
		{
			name:  "italic",
			input: "*italic*",
			style: styleItalic,
		},
		{
			name:  "code",
			input: "`code`",
			style: styleCode,
		},
		{
			name:  "strikethrough",
			input: "~~strike~~",
			style: styleStrikethrough,
		},
		{
			name:  "link",
			input: "[link](http://example.com)",
			style: styleLink,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks, err := parse(tt.input, &cfg)
			require.NoError(t, err)
			require.Len(t, blocks, 1)

			para, ok := blocks[0].(*paragraphBlock)
			require.True(t, ok, "expected paragraphBlock")
			require.NotEmpty(t, para.content)
			assert.Equal(t, tt.style, para.content[0].style)
		})
	}
}

func TestParseComplex(t *testing.T) {
	cfg := DefaultConfig()
	input := `# Title

This is a paragraph with **bold** and *italic* text.

## Section 1

- Item 1
- Item 2

> A blockquote

---

| Col A | Col B |
| ----- | ----- |
| 1     | 2     |
`
	blocks, err := parse(input, &cfg)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(blocks), 6)
}
